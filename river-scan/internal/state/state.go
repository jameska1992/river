package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type DirectoryRecord struct {
	LibraryID   string    `json:"library_id"`
	ContentHash string    `json:"content_hash"`
	LastScanned time.Time `json:"last_scanned"`
}

type State struct {
	mu          sync.Mutex
	path        string
	dirty       bool
	Directories map[string]DirectoryRecord `json:"directories"`
	// Shows maps a show folder path to the river-api show ID we resolved it
	// to on first scan. Re-identifying by folder path is identity-stable;
	// re-identifying by title isn't (meta-tv mutates the title to TMDB's
	// canonical form, which often differs from the parsed folder name —
	// "Shameless (US)" → parsed "Shameless US" but TMDB returns "Shameless",
	// causing the next scan's title-based lookup to mint a duplicate row).
	Shows map[string]string `json:"shows,omitempty"`
}

func Load(path string) (*State, error) {
	s := &State{
		path:        path,
		Directories: make(map[string]DirectoryRecord),
		Shows:       make(map[string]string),
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return nil, err
	}
	if s.Shows == nil {
		s.Shows = make(map[string]string)
	}
	return s, nil
}

// IsKnown returns true if the key has already been recorded with this exact
// content hash. The key is a directory or file path depending on the scan
// path (file-keyed for movies, directory-keyed for tv/audiobook/music).
func (s *State) IsKnown(key, contentHash string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec, ok := s.Directories[key]
	return ok && rec.ContentHash == contentHash
}

// Record updates the in-memory state. Writes are batched — call Flush to
// persist. We used to write the entire JSON file on every Record, which
// degraded badly when per-file movie scanning produced thousands of
// updates per run. A crash before Flush re-emits affected events on the
// next scan, which is harmless: downstream consumers are idempotent.
func (s *State) Record(key, libraryID, contentHash string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Directories[key] = DirectoryRecord{
		LibraryID:   libraryID,
		ContentHash: contentHash,
		LastScanned: time.Now().UTC(),
	}
	s.dirty = true
	return nil
}

// Forget removes the directory/file key from state so the next scan
// treats it as new again. Used by the admin "Remove Media" flow in
// river-api, which wants the row to be re-discoverable on the next scan
// instead of permanently suppressed by a still-matching content hash.
func (s *State) Forget(key string) {
	if key == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Directories[key]; !ok {
		return
	}
	delete(s.Directories, key)
	s.dirty = true
}

// ForgetPrefix removes every Directories key that sits at or beneath the
// given path. Used by the admin "delete show" / "delete audiobook" flow
// to wipe the per-season / per-folder content-hash entries that the DB
// can't enumerate when a season has zero episodes (or, in general,
// whenever we delete by parent rather than by every individual leaf).
//
// Matching is "key == prefix" OR "key starts with prefix + separator" so
// /tv/Show doesn't accidentally also forget /tv/Show 2.
func (s *State) ForgetPrefix(prefix string) {
	if prefix == "" {
		return
	}
	sep := string(filepath.Separator)
	clean := strings.TrimRight(prefix, sep)
	pfx := clean + sep
	s.mu.Lock()
	defer s.mu.Unlock()
	for key := range s.Directories {
		if key == clean || strings.HasPrefix(key, pfx) {
			delete(s.Directories, key)
			s.dirty = true
		}
	}
}

// LookupShow returns the show ID previously recorded for showPath, if any.
func (s *State) LookupShow(showPath string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	id, ok := s.Shows[showPath]
	return id, ok && id != ""
}

// RecordShow associates a show folder path with the river-api show ID we
// resolved it to. Persists on the next Flush.
func (s *State) RecordShow(showPath, showID string) {
	if showPath == "" || showID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Shows[showPath] == showID {
		return
	}
	s.Shows[showPath] = showID
	s.dirty = true
}

// Snapshot returns a read-only copy of the current state suitable for
// serializing to a JSON response. Maps are cloned so the caller can iterate
// without holding the lock and without racing with concurrent Record /
// Forget calls.
func (s *State) Snapshot() (map[string]DirectoryRecord, map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dirs := make(map[string]DirectoryRecord, len(s.Directories))
	for k, v := range s.Directories {
		dirs[k] = v
	}
	shows := make(map[string]string, len(s.Shows))
	for k, v := range s.Shows {
		shows[k] = v
	}
	return dirs, shows
}

// ForgetShow clears a recorded show ID. Called when the cached ID no longer
// resolves on the server (manual delete by the admin, DB wipe) so the next
// scan falls back to find-or-create instead of looping on a dead ID.
func (s *State) ForgetShow(showPath string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.Shows[showPath]; !ok {
		return
	}
	delete(s.Shows, showPath)
	s.dirty = true
}

// Flush writes pending state changes to disk. No-op when nothing is dirty.
func (s *State) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.dirty {
		return nil
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(s.path, data, 0o644); err != nil {
		return err
	}
	s.dirty = false
	return nil
}
