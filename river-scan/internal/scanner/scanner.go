package scanner

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"river-scan/internal/apiclient"
	"river-scan/internal/nameinfo"
	"river-scan/internal/publisher"
	"river-scan/internal/state"
)

// forceRescanCtxKey is a context flag that asks scanSeason to ignore its
// content-hash short-circuit (state.IsKnown) on this invocation. Set by
// ScanDir when called with force=true from the /scan-dir endpoint — used
// by the admin "identify" flow so a re-scan picks up episodes that were
// missed on the previous walk even when files haven't changed on disk.
// Periodic scans never set this; they want the short-circuit.
type forceRescanCtxKey struct{}

func withForceRescan(ctx context.Context) context.Context {
	return context.WithValue(ctx, forceRescanCtxKey{}, true)
}

func forceRescanRequested(ctx context.Context) bool {
	v, _ := ctx.Value(forceRescanCtxKey{}).(bool)
	return v
}

var videoExtensions = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
	".m4v": true, ".ts": true, ".wmv": true, ".webm": true,
	".flv": true,
}

var audioExtensions = map[string]bool{
	".mp3": true, ".flac": true, ".wav": true, ".aac": true,
	".m4a": true, ".ogg": true, ".opus": true, ".wma": true,
	".m4b": true,
}

type Scanner struct {
	api         *apiclient.Client
	pub         *publisher.Publisher
	state       *state.State
	noTranscode bool
	maxDepth    int
}

func New(api *apiclient.Client, pub *publisher.Publisher, st *state.State, noTranscode bool, maxDepth int) *Scanner {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	return &Scanner{api: api, pub: pub, state: st, noTranscode: noTranscode, maxDepth: maxDepth}
}

func (s *Scanner) Run(ctx context.Context) error {
	if err := s.api.Login(); err != nil {
		return fmt.Errorf("authenticate: %w", err)
	}
	libs, err := s.api.Libraries()
	if err != nil {
		return fmt.Errorf("fetch libraries: %w", err)
	}
	cache := newScanCache(s.api)
	for _, lib := range libs {
		if err := s.scanLibrary(ctx, lib, cache); err != nil {
			log.Printf("error scanning library %q (%s): %v", lib.Name, lib.ID, err)
		}
	}
	if err := s.state.Flush(); err != nil {
		log.Printf("error flushing state: %v", err)
	}
	return nil
}

func (s *Scanner) scanLibrary(ctx context.Context, lib apiclient.Library, cache *scanCache) error {
	paths, err := lib.ParsedPaths()
	if err != nil {
		return fmt.Errorf("parse paths for library %s: %w", lib.ID, err)
	}
	for _, libPath := range paths {
		if err := s.scanPath(ctx, libPath, lib, cache); err != nil {
			log.Printf("error scanning path %s: %v", libPath, err)
		}
	}
	return nil
}

func (s *Scanner) scanPath(ctx context.Context, libPath string, lib apiclient.Library, cache *scanCache) error {
	switch lib.Type {
	case "movie":
		return s.scanMoviesUnder(ctx, libPath, libPath, lib, cache)
	}
	// All other types still iterate one level deep: each subdirectory is the
	// containing unit for that library (show / audiobook / artist).
	entries, err := os.ReadDir(libPath)
	if err != nil {
		return fmt.Errorf("read dir %s: %w", libPath, err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if skipDir(entry.Name()) {
			continue
		}
		dirPath := filepath.Join(libPath, entry.Name())
		switch lib.Type {
		case "tvshow":
			if err := s.scanTVShow(ctx, dirPath, entry.Name(), libPath, lib); err != nil {
				log.Printf("error scanning show %q: %v", entry.Name(), err)
			}
		case "audiobook":
			if err := s.scanAudiobook(ctx, dirPath, entry.Name(), libPath, lib); err != nil {
				log.Printf("error scanning audiobook %q: %v", entry.Name(), err)
			}
		case "music":
			if err := s.scanMusic(ctx, dirPath, entry.Name(), libPath, lib); err != nil {
				log.Printf("error scanning artist %q: %v", entry.Name(), err)
			}
		}
	}
	return nil
}

// scanMoviesUnder walks root recursively and emits one event per video file.
// root may be the library path (full-scan) or an arbitrary sub-path (called
// from the /scan-dir HTTP handler when a user uploads a folder).
func (s *Scanner) scanMoviesUnder(ctx context.Context, root, libPath string, lib apiclient.Library, cache *scanCache) error {
	return walkMovieFiles(root, s.maxDepth, func(filePath string) error {
		if err := s.scanMovieFile(ctx, filePath, libPath, lib, cache); err != nil {
			log.Printf("error scanning movie file %q: %v", filePath, err)
		}
		return nil
	})
}

// scanTVShow scans one level deeper, emitting a media_discovered event per
// season directory. It handles two layouts:
//
//   - Show/Season N/files      — canonical. Walk each subdir as a season.
//   - Show Season N/files      — flat. The show folder name itself encodes a
//                                season; treat the folder as the season and
//                                unify siblings under the stripped title.
//
// In the flat layout, scanning `Show S01/`, `Show S02/`, `Show S03/` as three
// separate shows is exactly the duplicated-show bug; the suffix detection
// keeps them under one show record.
func (s *Scanner) scanTVShow(ctx context.Context, showPath, showName, libPath string, lib apiclient.Library) error {
	entries, err := os.ReadDir(showPath)
	if err != nil {
		return fmt.Errorf("read show dir %s: %w", showPath, err)
	}
	var seasonSubdirs []os.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() || skipDir(entry.Name()) {
			continue
		}
		seasonSubdirs = append(seasonSubdirs, entry)
	}

	strippedTitle, _, hasSeasonSuffix := detectShowSeasonSuffix(showName)
	effectiveShowName := showName
	if hasSeasonSuffix {
		effectiveShowName = strippedTitle
	}

	if len(seasonSubdirs) > 0 {
		// Canonical "Show/Season N/" — or "Show Season 1/Season 1/" if the
		// user double-nested. Either way the inner subdirs are the seasons.
		for _, entry := range seasonSubdirs {
			seasonPath := filepath.Join(showPath, entry.Name())
			if err := s.scanSeason(ctx, seasonPath, entry.Name(), showPath, effectiveShowName, libPath, lib); err != nil {
				log.Printf("error scanning season %q: %v", entry.Name(), err)
			}
		}
		return nil
	}

	// No subdirs. If the show folder name encodes a season, the show dir IS
	// the season dir (Layout B). Otherwise fall through with an empty
	// seasonName so scanSeason can sniff a season number from SxxExx tags in
	// the actual filenames.
	seasonName := ""
	if hasSeasonSuffix {
		seasonName = showName
	}
	if err := s.scanSeason(ctx, showPath, seasonName, showPath, effectiveShowName, libPath, lib); err != nil {
		log.Printf("error scanning flat-season show %q: %v", showName, err)
	}
	return nil
}

func (s *Scanner) scanSeason(ctx context.Context, seasonPath, seasonName, showPath, showName, libPath string, lib apiclient.Library) error {
	files, err := collectMediaFiles(seasonPath, lib.Type)
	if err != nil {
		return fmt.Errorf("collect files from %s: %w", seasonPath, err)
	}
	if len(files) == 0 {
		return nil
	}

	// Resolve show + season up front (cheap with the per-run cache in
	// resolveShow / FindOrCreate). We need their IDs for the source_path
	// backfill below, which has to run even when the content hash matches
	// the previous scan — that's the whole point of the backfill, to
	// repair legacy rows whose files haven't changed on disk but whose DB
	// records pre-date the source_path field.
	info := nameinfo.ParseDir(showPath, showName, nil)

	show, err := s.resolveShow(showPath, showName, lib.ID)
	if err != nil {
		return fmt.Errorf("resolve show %q: %w", showName, err)
	}

	seasonNum := bestSeasonNumber(seasonName, files)
	season, err := s.api.FindOrCreateSeason(show.ID, seasonNum)
	if err != nil {
		return fmt.Errorf("find/create season %d for show %s: %w", seasonNum, show.ID, err)
	}

	// Always backfill source_path on existing episodes (transcode mode
	// only — no-transcode mode sets source_path on create/update via
	// registerEpisodesDirect). The helper is a no-op for episodes that
	// already have source_path set, so steady-state cost is one
	// ListEpisodes call per season per scan.
	if !s.noTranscode {
		s.backfillEpisodeSourcePaths(show.ID, season.ID, files)
	}

	// Now check the content-hash short-circuit. If nothing changed on disk
	// (and we're not forced), there's no new event to publish, but the
	// backfill above has already run so legacy rows still get repaired.
	hash := contentHash(files)
	if !forceRescanRequested(ctx) && s.state.IsKnown(seasonPath, hash) {
		return nil
	}

	// See scanMovieFile for the directRegister/publish mode split.
	directRegister := s.noTranscode || lib.PreTranscoded
	publish := !s.noTranscode

	if directRegister {
		if err := s.registerEpisodesDirect(show.ID, season.ID, seasonNum, files); err != nil {
			return fmt.Errorf("register episodes direct: %w", err)
		}
	}
	if publish {
		event := publisher.MediaDiscoveredEvent{
			EventID:       uuid.New().String(),
			LibraryID:     lib.ID,
			LibraryType:   lib.Type,
			LibraryPath:   libPath,
			DirectoryName: showName,
			DirectoryPath: showPath,
			SeasonName:    seasonName,
			SeasonPath:    seasonPath,
			MediaID:       show.ID,
			SeasonID:      season.ID,
			TMDBID:        info.TMDBID,
			IMDBID:        info.IMDBID,
			Files:         files,
			DiscoveredAt:  time.Now().UTC(),
			PreTranscoded: lib.PreTranscoded,
		}
		if err := s.pub.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish event for %s: %w", seasonPath, err)
		}
	}
	if err := s.state.Record(seasonPath, lib.ID, hash); err != nil {
		log.Printf("error recording state for %s: %v", seasonPath, err)
	}
	msg := fmt.Sprintf("detected tvshow %q / %s", showName, seasonName)
	log.Printf("discovered: [tvshow] %s / %s", showName, seasonName)
	s.api.Log("info", msg)
	return nil
}

// resolveShow returns the river-api show record for a show folder, preferring
// the per-folder show ID we cached on a prior scan (path is identity-stable,
// title isn't — meta-tv rewrites titles to TMDB's canonical form after the
// scan creates the record). Falls back to find-or-create by title when the
// path has never been resolved, or when the cached ID no longer exists on
// the server (manual admin delete, DB wipe, etc.).
func (s *Scanner) resolveShow(showPath, showName, libraryID string) (*apiclient.TVShow, error) {
	if cachedID, ok := s.state.LookupShow(showPath); ok {
		if show, err := s.api.GetTVShow(cachedID); err == nil {
			// Detect a stale mapping from the legacy merge bug: an older
			// scanner build matched purely by title, so two folders with
			// the same name (e.g. "Avatar The Last Airbender" 2005 and
			// 2024) both got cached against the first folder's show ID.
			// If the cached row's FolderPath disagrees with the path we're
			// scanning now, drop the cache and let find-or-create below
			// (which now disambiguates on folder_path) mint a fresh row.
			if show.FolderPath != "" && show.FolderPath != showPath {
				log.Printf("INFO cached show %s for %q has folder_path=%q; treating as stale merge and re-resolving",
					cachedID, showPath, show.FolderPath)
				s.state.ForgetShow(showPath)
			} else {
				// Backfill folder_path on pre-existing rows so the admin
				// "identify" flow can target a re-scan at this directory.
				if show.FolderPath == "" {
					if err := s.api.UpdateTVShowFolderPath(show.ID, showPath); err == nil {
						show.FolderPath = showPath
					}
				}
				return show, nil
			}
		} else {
			// Any error — 404 from a deleted show, transient network blip,
			// authz hiccup — falls through to find-or-create. Forgetting the
			// stale id keeps us out of a loop on a dead reference.
			log.Printf("INFO cached show id %s for %q no longer resolves, falling back to find-or-create", cachedID, showPath)
			s.state.ForgetShow(showPath)
		}
	}

	// Parse show folder name with the full PTN-style normalizer (handles
	// release-named folders like "Breaking.Bad.S01.1080p.BluRay") and read
	// any tvshow.nfo sidecar for embedded TMDB/IMDb IDs. Same machinery the
	// movie path uses, pointed at the show folder.
	info := nameinfo.ParseDir(showPath, showName, nil)
	title := info.Title
	if title == "" {
		title = showName
	}
	// Drop a trailing "Season N" / "S01" off the title so siblings that
	// disagree only on the season tag converge on a single show record.
	if stripped, _, ok := stripSeasonSuffix(title); ok {
		title = stripped
	}

	show, err := s.api.FindOrCreateTVShow(libraryID, title, showPath)
	if err != nil {
		return nil, err
	}
	// FindOrCreateTVShow only sets folder_path on create. For a row we
	// matched by title (different scan, prior version of the scanner), make
	// sure folder_path gets populated too.
	if show.FolderPath == "" {
		if err := s.api.UpdateTVShowFolderPath(show.ID, showPath); err == nil {
			show.FolderPath = showPath
		}
	}
	s.state.RecordShow(showPath, show.ID)
	return show, nil
}

func (s *Scanner) scanAudiobook(ctx context.Context, dirPath, dirName, libPath string, lib apiclient.Library) error {
	files, err := collectMediaFiles(dirPath, lib.Type)
	if err != nil {
		return fmt.Errorf("collect files from %s: %w", dirPath, err)
	}
	if len(files) == 0 {
		return nil
	}
	hash := contentHash(files)
	if s.state.IsKnown(dirPath, hash) {
		return nil
	}

	title, year := parseDirName(dirName)
	book, err := s.api.FindOrCreateAudiobook(lib.ID, title, year)
	if err != nil {
		return fmt.Errorf("find/create audiobook %q: %w", dirName, err)
	}

	directRegister := s.noTranscode || lib.PreTranscoded
	publish := !s.noTranscode

	if directRegister {
		if err := s.registerAudiobookDirect(book.ID, files); err != nil {
			return fmt.Errorf("register audiobook direct %q: %w", dirName, err)
		}
	}
	if publish {
		event := publisher.MediaDiscoveredEvent{
			EventID:       uuid.New().String(),
			LibraryID:     lib.ID,
			LibraryType:   lib.Type,
			LibraryPath:   libPath,
			DirectoryName: dirName,
			DirectoryPath: dirPath,
			MediaID:       book.ID,
			Files:         files,
			DiscoveredAt:  time.Now().UTC(),
			PreTranscoded: lib.PreTranscoded,
		}
		if err := s.pub.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish event for %s: %w", dirPath, err)
		}
	}
	if err := s.state.Record(dirPath, lib.ID, hash); err != nil {
		log.Printf("error recording state for %s: %v", dirPath, err)
	}
	log.Printf("discovered: [audiobook] %s", dirName)
	s.api.Log("info", fmt.Sprintf("detected audiobook %q", dirName))
	return nil
}

// ScanDir processes a single directory from the upload handler. Publishes a
// media_discovered event for new content, records state, and (for movies)
// recursively walks any subdirectories. For tvshow libraries, dirPath is the
// season dir, showPath is the show root, showName is the show dir name; for
// other library types, showPath and showName are unused.
func (s *Scanner) ScanDir(ctx context.Context, lib apiclient.Library, dirPath, showPath, showName string, force bool) error {
	libPath := s.findLibPath(lib, dirPath)
	defer func() {
		if err := s.state.Flush(); err != nil {
			log.Printf("error flushing state: %v", err)
		}
	}()
	if force {
		ctx = withForceRescan(ctx)
	}
	switch lib.Type {
	case "tvshow":
		// Show-level scan: caller passed a show folder, not a season. Walk
		// every season subdir so episodes added since the last scan get
		// picked up. The admin "identify" flow hits this case — it knows
		// the show's folder_path but not which season changed.
		if showPath == "" || showPath == dirPath {
			return s.scanTVShow(ctx, dirPath, filepath.Base(dirPath), libPath, lib)
		}
		// Season-level scan: legacy upload flow that points at a single
		// season directory inside a known show.
		return s.scanSeason(ctx, dirPath, filepath.Base(dirPath), showPath, showName, libPath, lib)
	case "audiobook":
		return s.scanAudiobook(ctx, dirPath, filepath.Base(dirPath), libPath, lib)
	case "music":
		return s.scanMusic(ctx, dirPath, filepath.Base(dirPath), libPath, lib)
	default:
		cache := newScanCache(s.api)
		return s.scanMoviesUnder(ctx, dirPath, libPath, lib, cache)
	}
}

// findLibPath returns the library path containing dirPath. The library passed in from
// the /scan-dir endpoint only has ID + Type set, so we look up the full library to find
// its configured paths and pick the one that contains dirPath. Returns "" if not found.
func (s *Scanner) findLibPath(lib apiclient.Library, dirPath string) string {
	paths, err := lib.ParsedPaths()
	if err != nil || len(paths) == 0 {
		libs, err := s.api.Libraries()
		if err != nil {
			return ""
		}
		for _, l := range libs {
			if l.ID == lib.ID {
				if p, err := l.ParsedPaths(); err == nil {
					paths = p
				}
				break
			}
		}
	}
	for _, p := range paths {
		clean := filepath.Clean(p)
		if dirPath == clean || strings.HasPrefix(dirPath, clean+string(filepath.Separator)) {
			return clean
		}
	}
	return ""
}

// scanMovieFile is the unit of work for movie libraries: one video file
// becomes one movie record. Title/year/IDs are derived from the file's own
// basename first, falling back to the parent directory's name and any NFO
// sidecar there. State is keyed on the file path so adding a single new
// release into a thousand-file library only emits one event.
func (s *Scanner) scanMovieFile(ctx context.Context, filePath, libPath string, lib apiclient.Library, cache *scanCache) error {
	hash := contentHash([]string{filePath})
	if s.state.IsKnown(filePath, hash) {
		return nil
	}

	parentDir := filepath.Dir(filePath)
	parentName := filepath.Base(parentDir)
	fileBase := strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))

	// Parse the filename first (it's almost always the most specific name),
	// then layer parent-dir info underneath for year/IDs the filename lacks.
	info := nameinfo.Parse(fileBase)
	parentInfo := nameinfo.ParseDir(parentDir, parentName, []string{filePath})
	if info.Year == 0 {
		info.Year = parentInfo.Year
	}
	if info.TMDBID == 0 {
		info.TMDBID = parentInfo.TMDBID
	}
	if info.IMDBID == "" {
		info.IMDBID = parentInfo.IMDBID
	}
	title := info.Title
	if title == "" {
		title = parentInfo.Title
	}
	if title == "" {
		title = fileBase
	}

	movie, err := cache.findOrCreateMovie(lib.ID, title, info.Year, filePath)
	if err != nil {
		return fmt.Errorf("find/create movie %q: %w", title, err)
	}
	// Backfill source_path for cache hits (pre-existing rows that were
	// created before this field existed). Fire-and-forget — if it fails
	// we'll try again on the next scan.
	if movie.SourcePath == "" {
		if err := s.api.UpdateMovieSourcePath(movie.ID, filePath); err == nil {
			movie.SourcePath = filePath
		}
	}

	// Two dimensions of behavior:
	//   directRegister — write file_path directly against the record
	//                    (source path IS the stream path). True when
	//                    the transcoder is disabled entirely OR when
	//                    this specific library is marked pre-transcoded.
	//   publish        — emit the RabbitMQ event so downstream metadata
	//                    consumers enrich the record. Still true for
	//                    pre-transcoded libraries — the event carries
	//                    the flag and the video-trans consumer skips.
	directRegister := s.noTranscode || lib.PreTranscoded
	publish := !s.noTranscode

	if directRegister {
		if err := s.updateMovieFilePath(movie.ID, filePath); err != nil {
			return fmt.Errorf("update file_path for %q: %w", title, err)
		}
	}
	if publish {
		event := publisher.MediaDiscoveredEvent{
			EventID:       uuid.New().String(),
			LibraryID:     lib.ID,
			LibraryType:   lib.Type,
			LibraryPath:   libPath,
			DirectoryName: parentName,
			DirectoryPath: parentDir,
			MediaID:       movie.ID,
			TMDBID:        info.TMDBID,
			IMDBID:        info.IMDBID,
			Files:         []string{filePath},
			DiscoveredAt:  time.Now().UTC(),
			PreTranscoded: lib.PreTranscoded,
		}
		if err := s.pub.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish event for %s: %w", filePath, err)
		}
	}

	if err := s.state.Record(filePath, lib.ID, hash); err != nil {
		log.Printf("error recording state for %s: %v", filePath, err)
	}
	log.Printf("discovered: [movie] %s (%d) from %s", title, info.Year, filepath.Base(filePath))
	yearTag := ""
	if info.Year > 0 {
		yearTag = fmt.Sprintf(" (%d)", info.Year)
	}
	s.api.Log("info", fmt.Sprintf("detected movie %q%s from %s", title, yearTag, filepath.Base(filePath)))

	// Sidecars: the transcoder-produced .audio_N.mp4 variants (siblings
	// of the main file) and the subtitles/ subdir. Done regardless of
	// mode so a library pointed at an output tree still gets its extra
	// audio and captions surfaced. Idempotent — see registerSidecars.
	s.registerMovieSidecars(movie.ID, filePath)
	return nil
}

// updateMovieFilePath fetches the movie's current record and writes back its
// FilePath only if it changed. Reading first preserves any metadata that
// river-meta-movie has already enriched between scans.
func (s *Scanner) updateMovieFilePath(movieID, filePath string) error {
	full, err := s.api.GetMovie(movieID)
	if err != nil {
		return fmt.Errorf("get movie: %w", err)
	}
	if full.FilePath == filePath {
		return nil
	}
	log.Printf("INFO setting movie %q file_path → %s", full.Title, filePath)
	_, err = s.api.UpdateMovie(movieID, apiclient.MovieUpdateRequest{
		LibraryID:     full.LibraryID,
		Title:         full.Title,
		OriginalTitle: full.OriginalTitle,
		Description:   full.Description,
		Year:          full.Year,
		Genres:        full.Genres,
		Rating:        full.Rating,
		Runtime:       full.Runtime,
		PosterPath:    full.PosterPath,
		BackdropPath:  full.BackdropPath,
		FilePath:      filePath,
	})
	return err
}

// scanMusic handles one artist directory inside a music library. Music keeps
// its dir-based grouping (artist → albums → tracks) so we don't break the
// album-as-folder convention that audio metadata depends on.
func (s *Scanner) scanMusic(ctx context.Context, dirPath, dirName, libPath string, lib apiclient.Library) error {
	files, err := collectMediaFiles(dirPath, lib.Type)
	if err != nil {
		return fmt.Errorf("collect files from %s: %w", dirPath, err)
	}
	if len(files) == 0 {
		return nil
	}
	hash := contentHash(files)
	if s.state.IsKnown(dirPath, hash) {
		return nil
	}

	directRegister := s.noTranscode || lib.PreTranscoded
	publish := !s.noTranscode

	// The artist record is needed either way — for direct registration
	// registerMusicDirect resolves it itself; for the event path we
	// resolve here so the artist ID rides the event.
	if directRegister {
		if err := s.registerMusicDirect(dirPath, dirName, files, lib); err != nil {
			return fmt.Errorf("register music direct %q: %w", dirName, err)
		}
	}
	if publish {
		artist, err := s.api.FindOrCreateArtist(lib.ID, dirName)
		if err != nil {
			return fmt.Errorf("find/create artist %q: %w", dirName, err)
		}
		event := publisher.MediaDiscoveredEvent{
			EventID:       uuid.New().String(),
			LibraryID:     lib.ID,
			LibraryType:   lib.Type,
			LibraryPath:   libPath,
			DirectoryName: dirName,
			DirectoryPath: dirPath,
			MediaID:       artist.ID,
			Files:         files,
			DiscoveredAt:  time.Now().UTC(),
			PreTranscoded: lib.PreTranscoded,
		}
		if err := s.pub.Publish(ctx, event); err != nil {
			return fmt.Errorf("publish event for %s: %w", dirPath, err)
		}
	}
	if err := s.state.Record(dirPath, lib.ID, hash); err != nil {
		log.Printf("error recording state for %s: %v", dirPath, err)
	}
	log.Printf("discovered: [music] %s", dirName)
	s.api.Log("info", fmt.Sprintf("detected artist %q", dirName))
	return nil
}

func collectMediaFiles(dir, libType string) ([]string, error) {
	exts := videoExtensions
	if libType == "music" || libType == "audiobook" {
		exts = audioExtensions
	}
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		// Don't descend into subtitle sidecar dirs — anything in them
		// is by convention a companion file, not primary media.
		if d.IsDir() && d != nil {
			if systemDirs[strings.ToLower(d.Name())] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if !exts[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		// For video libs, exclude the transcoder's .audio_N variants so
		// a single episode/movie doesn't get counted N+1 times.
		if libType != "music" && libType != "audiobook" && isAudioVariantFile(path) {
			return nil
		}
		files = append(files, path)
		return nil
	})
	return files, err
}

func contentHash(files []string) string {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Strings(sorted)

	h := sha256.New()
	for _, f := range sorted {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		fmt.Fprintf(h, "%s|%d|%d\n", f, info.Size(), info.ModTime().Unix())
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

var yearSuffix = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)\s*$`)

// nonNumberSeparatorRe matches runs of common release-name separators:
// dot, underscore, hyphen. We deliberately don't normalize parentheses
// here so the "(YYYY)" suffix still matches yearSuffix.
var nonNumberSeparatorRe = regexp.MustCompile(`[._\-]+`)

// normalizeSeparators replaces dot/underscore/hyphen runs with single
// spaces and collapses whitespace. Used by the non-movie scanners
// (audiobook, music album, TV season folder) where filename punctuation
// is artifact, not part of the real title — "Doctor.Who" / "Doctor_Who"
// both become "Doctor Who".
func normalizeSeparators(s string) string {
	return strings.TrimSpace(strings.Join(strings.Fields(
		nonNumberSeparatorRe.ReplaceAllString(s, " ")), " "))
}

func parseDirName(name string) (title string, year int) {
	cleaned := normalizeSeparators(name)
	if m := yearSuffix.FindStringSubmatch(cleaned); m != nil {
		y, _ := strconv.Atoi(m[2])
		return strings.TrimSpace(m[1]), y
	}
	return cleaned, 0
}

// seasonNumRe matches "Season N" or "SN" (with optional whitespace after
// either) inside a directory name. Separator normalization happens before
// matching, so "Season.1" / "S_03" / "S-12" all resolve correctly.
var seasonNumRe = regexp.MustCompile(`(?i)(?:season\s*|s\s*)(\d+)`)

// trailingSeasonRe matches when a name ENDS in a season indicator. Used to
// peel off a season-tag suffix from a show-folder name ("Foo Season 1",
// "Foo S03"). Requires at least one character of title before the indicator
// so we don't match a bare "Season 1" / "S01" as a stripped suffix.
var trailingSeasonRe = regexp.MustCompile(`(?i)^(.+?)\s+(?:season\s*|s)\s*(\d+)\s*$`)

// seasonFromFilenameRe pulls a season number out of an SxxExx-style filename.
var seasonFromFilenameRe = regexp.MustCompile(`(?i)[Ss](\d+)[Ee]\d+`)

// parseSeasonNumber returns the season number from a season folder name,
// defaulting to 1 when the name lacks a "Season N" / "SN" marker. Kept for
// callers that always need a non-zero number; the scanner's TV path now uses
// bestSeasonNumber instead so the default only kicks in as a last resort.
func parseSeasonNumber(name string) int {
	if n := parseSeasonNumberStrict(name); n > 0 {
		return n
	}
	return 1
}

// parseSeasonNumberStrict returns 0 when the name has no season marker,
// letting the caller layer in additional fallbacks (file-name sniffing).
// Note that an explicit "Season 0" / "S00" also returns 0 here — the caller
// can't distinguish "marker present and 0" from "no marker", but the
// callers that care (stripSeasonSuffix etc.) already gate on >0, so the
// collapse is fine. Use seasonNumberFromName when you do need to tell them
// apart.
func parseSeasonNumberStrict(name string) int {
	cleaned := normalizeSeparators(name)
	if m := seasonNumRe.FindStringSubmatch(cleaned); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// specialsFolderNames is the case-insensitive set of folder names we treat
// as the Specials season (season 0 under the Plex/TMDB convention). This
// is separate from the "Season N" / "SN" regex because "Specials" carries
// no number to capture — it's a sentinel. "Extras" is NOT in here: the
// walker already routes it to the systemDirs skip list because Plex/
// Jellyfin reserve it for behind-the-scenes/featurette content rather
// than for episodes.
var specialsFolderNames = map[string]bool{
	"specials": true,
	"special":  true,
}

// seasonNumberFromName parses a season number from a folder name and tells
// the caller whether the name carried any season signal at all. It accepts:
//
//   - "Season N" / "SN" / "S0N" — returns (N, true)
//   - "Season 0" / "S0" / "S00" — returns (0, true)
//   - "Specials" / "Special" / "Extras" — returns (0, true)
//   - anything else — returns (0, false)
//
// Used by bestSeasonNumber so an explicit Specials/Season-0 folder doesn't
// get rewritten to season 1 by the "no signal → default 1" fallback.
func seasonNumberFromName(name string) (int, bool) {
	cleaned := normalizeSeparators(name)
	if specialsFolderNames[strings.ToLower(strings.TrimSpace(cleaned))] {
		return 0, true
	}
	if m := seasonNumRe.FindStringSubmatch(cleaned); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n, true
	}
	return 0, false
}

// stripSeasonSuffix peels "Season N" / "SN" off the trailing edge of a
// already-cleaned title. Returns (cleanedTitle, seasonNumber, true) on
// match, or (original, 0, false) otherwise. Operates after separator
// normalization so "Foo.Bar.S01" reduces correctly.
func stripSeasonSuffix(name string) (string, int, bool) {
	cleaned := normalizeSeparators(name)
	if m := trailingSeasonRe.FindStringSubmatch(cleaned); m != nil {
		n, _ := strconv.Atoi(m[2])
		title := strings.TrimSpace(m[1])
		if title != "" && n > 0 {
			return title, n, true
		}
	}
	return cleaned, 0, false
}

// detectShowSeasonSuffix runs nameinfo.Parse first to drop release-name
// noise that would otherwise hide the trailing season tag (e.g. "Foo S01
// 1080p BluRay" → "Foo S01"), then peels the season suffix. The result is
// the canonical show title to register the record under.
func detectShowSeasonSuffix(showName string) (string, int, bool) {
	cleaned := nameinfo.Parse(showName).Title
	if cleaned == "" {
		cleaned = showName
	}
	return stripSeasonSuffix(cleaned)
}

// bestSeasonNumber prefers the season folder name when it has an explicit
// marker (including "Specials"/"Special"/"Extras" → 0, and explicit
// "Season 0"/"S00" → 0), otherwise scans the actual episode filenames for
// an SxxExx tag — accepting S00 there too, so files dropped at the show
// root from a Specials rip still map to season 0. Defaults to 1 only when
// nothing yields a number.
func bestSeasonNumber(folderName string, files []string) int {
	if n, ok := seasonNumberFromName(folderName); ok {
		return n
	}
	for _, f := range files {
		base := filepath.Base(f)
		if m := seasonFromFilenameRe.FindStringSubmatch(base); m != nil {
			if n, err := strconv.Atoi(m[1]); err == nil {
				return n
			}
		}
	}
	return 1
}

// --- no-transcode direct registration ---

// backfillEpisodeSourcePaths fills in source_path on episodes that exist in
// river-api but were created without it (older code paths, or video-trans
// update path that previously didn't include it). Each on-disk file's
// episode number gets matched to an existing row; rows with empty
// source_path get patched. Best-effort: errors only log.
func (s *Scanner) backfillEpisodeSourcePaths(showID, seasonID string, files []string) {
	existing, err := s.api.ListEpisodes(showID, seasonID)
	if err != nil {
		return
	}
	byNum := make(map[int]apiclient.Episode, len(existing))
	for _, ep := range existing {
		byNum[ep.Number] = ep
	}
	for _, file := range files {
		if !isVideoFile(file) {
			continue
		}
		epNum := parseEpisodeNumber(file)
		if epNum == 0 {
			continue
		}
		ep, ok := byNum[epNum]
		if !ok || ep.SourcePath != "" {
			continue
		}
		if err := s.api.UpdateEpisodeSourcePath(showID, seasonID, ep.ID, file); err != nil {
			log.Printf("WARN backfill source_path for episode %s: %v", ep.ID, err)
		}
	}
}

// registerEpisodesDirect creates or updates episode records from the given
// file list without going through RabbitMQ. In addition to file_path /
// source_path bookkeeping it also triggers per-episode sidecar registration
// so any `<stem>.audio_N.mp4` variants and `subtitles/<stem>.*.vtt` files
// alongside a video get surfaced as AudioTrack / Subtitle records.
func (s *Scanner) registerEpisodesDirect(showID, seasonID string, seasonNum int, files []string) error {
	existing, err := s.api.ListEpisodes(showID, seasonID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}
	episodeByNum := make(map[int]apiclient.Episode, len(existing))
	for _, ep := range existing {
		episodeByNum[ep.Number] = ep
	}
	for _, file := range files {
		if !isVideoFile(file) {
			continue
		}
		epNum := parseEpisodeNumber(file)
		if epNum == 0 {
			log.Printf("WARN cannot parse episode number from %q, skipping", filepath.Base(file))
			continue
		}
		var epID string
		if ep, exists := episodeByNum[epNum]; exists {
			// No-transcode mode: file_path IS the source. Keep them aligned
			// and also populate source_path so the API-side fallback works
			// uniformly across modes.
			if ep.FilePath == file && ep.SourcePath == file {
				epID = ep.ID
			} else {
				log.Printf("INFO updating episode S%02dE%02d file_path", seasonNum, epNum)
				updated, err := s.api.UpdateEpisode(showID, seasonID, ep.ID, apiclient.EpisodeRequest{
					Number:     ep.Number,
					FilePath:   file,
					SourcePath: file,
				})
				if err != nil {
					log.Printf("ERROR update episode S%02dE%02d: %v", seasonNum, epNum, err)
					continue
				}
				episodeByNum[epNum] = *updated
				epID = updated.ID
			}
		} else {
			log.Printf("INFO creating episode S%02dE%02d", seasonNum, epNum)
			ep, err := s.api.CreateEpisode(showID, seasonID, apiclient.EpisodeRequest{
				Number:     epNum,
				FilePath:   file,
				SourcePath: file,
			})
			if err != nil {
				log.Printf("ERROR create episode S%02dE%02d: %v", seasonNum, epNum, err)
				continue
			}
			episodeByNum[epNum] = *ep
			epID = ep.ID
		}
		// Sidecars are per-episode (each file lives in the season dir
		// alongside its own .audio_N.mp4 variants and matching subtitle
		// files under subtitles/), so the registration happens after
		// each episode ID is known.
		s.registerEpisodeSidecars(showID, seasonID, epID, file)
	}
	return nil
}

func (s *Scanner) registerMusicDirect(dirPath, dirName string, files []string, lib apiclient.Library) error {
	artist, err := s.api.FindOrCreateArtist(lib.ID, dirName)
	if err != nil {
		return fmt.Errorf("find/create artist: %w", err)
	}
	for albumName, albumFiles := range groupByAlbum(dirPath, files) {
		albumTitle, albumYear := parseDirName(albumName)
		album, err := s.api.FindOrCreateAlbum(lib.ID, artist.ID, albumTitle, albumYear)
		if err != nil {
			log.Printf("ERROR find/create album %q: %v", albumTitle, err)
			continue
		}
		existing, err := s.api.ListAlbumTracks(album.ID)
		if err != nil {
			log.Printf("ERROR list tracks for album %s: %v", album.ID, err)
			continue
		}
		registered := make(map[int]bool, len(existing))
		for _, t := range existing {
			registered[t.Number] = true
		}
		for _, file := range albumFiles {
			num := parseTrackNumber(file)
			if num > 0 && registered[num] {
				continue
			}
			title := parseAudioTitle(file)
			if _, err := s.api.CreateTrack(apiclient.TrackRequest{
				LibraryID: lib.ID,
				AlbumID:   album.ID,
				ArtistID:  artist.ID,
				Title:     title,
				Number:    num,
				FilePath:  file,
			}); err != nil {
				log.Printf("ERROR create track %q: %v", title, err)
				continue
			}
			if num > 0 {
				registered[num] = true
			}
		}
	}
	return nil
}

func (s *Scanner) registerAudiobookDirect(bookID string, files []string) error {
	existing, err := s.api.ListChapters(bookID)
	if err != nil {
		return fmt.Errorf("list chapters: %w", err)
	}
	registered := make(map[int]bool, len(existing))
	for _, ch := range existing {
		registered[ch.Number] = true
	}
	sorted := sortedAudioFiles(files)
	for i, file := range sorted {
		num := parseChapterNumber(file)
		if num == 0 {
			num = i + 1
		}
		if registered[num] {
			continue
		}
		chTitle := parseAudioTitle(file)
		if _, err := s.api.CreateChapter(bookID, apiclient.ChapterRequest{
			Number:   num,
			Title:    chTitle,
			FilePath: file,
		}); err != nil {
			log.Printf("ERROR create chapter %d: %v", num, err)
			continue
		}
		registered[num] = true
	}
	return nil
}

// --- helpers for no-transcode mode ---

func isVideoFile(path string) bool {
	return videoExtensions[strings.ToLower(filepath.Ext(path))]
}

var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[Ss]\d+[Ee](\d+)`),
	regexp.MustCompile(`(?i)\d+[xX](\d+)`),
	regexp.MustCompile(`(?i)episode\s*(\d+)`),
	regexp.MustCompile(`(?:^|\D)0*(\d{1,3})(?:\D|$)`),
}

func parseEpisodeNumber(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, re := range episodePatterns {
		if m := re.FindStringSubmatch(base); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n > 0 {
				return n
			}
		}
	}
	return 0
}

func groupByAlbum(dirPath string, files []string) map[string][]string {
	groups := make(map[string][]string)
	for _, f := range files {
		rel, err := filepath.Rel(dirPath, filepath.Dir(f))
		if err != nil || rel == "." {
			groups[filepath.Base(dirPath)] = append(groups[filepath.Base(dirPath)], f)
			continue
		}
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		groups[parts[0]] = append(groups[parts[0]], f)
	}
	return groups
}

var trackNumRe = regexp.MustCompile(`^(?:track\s*)?0*(\d{1,3})[\s.\-_]+`)

func parseTrackNumber(path string) int {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if m := trackNumRe.FindStringSubmatch(base); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

var chapterNumRe = regexp.MustCompile(`^(?:chapter\s*|part\s*)?0*(\d{1,3})[\s.\-_]`)

func parseChapterNumber(path string) int {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if m := chapterNumRe.FindStringSubmatch(base); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

func sortedAudioFiles(files []string) []string {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return filepath.Base(sorted[i]) < filepath.Base(sorted[j])
	})
	return sorted
}

var numPrefixRe = regexp.MustCompile(`^(?:(?:track|chapter|part)\s*)?0*\d+[\s.\-_]+`)
var dotSpaceRe = regexp.MustCompile(`[._]+`)
var qualityTagRe = regexp.MustCompile(`(?i)[._\s](128k|192k|256k|320k|flac|lossless|hd|remaster(ed)?|deluxe).*$`)

func parseAudioTitle(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = numPrefixRe.ReplaceAllString(base, "")
	base = qualityTagRe.ReplaceAllString(base, "")
	base = dotSpaceRe.ReplaceAllString(base, " ")
	return strings.TrimSpace(base)
}
