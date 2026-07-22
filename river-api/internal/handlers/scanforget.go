package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// scanNotifier wraps the river-scan /forget endpoint so the admin delete
// flow can ask the scanner to drop content-hash entries for paths it just
// removed from the DB. Without this, the next scan would see the content
// unchanged and skip re-discovery, and the row would never come back.
type scanNotifier struct {
	url  string
	http *http.Client
}

func newScanNotifier(url string) *scanNotifier {
	return &scanNotifier{url: url, http: &http.Client{Timeout: 10 * time.Second}}
}

// Forget is best-effort: the DB delete has to succeed regardless of
// whether river-scan is reachable, so errors are logged, not returned.
// `paths` are exact Directories keys (movie source paths, season dirs).
// `prefixes` are parent directories whose whole subtree should be
// forgotten (used when individual children — e.g. seasons without
// enriched episodes — aren't enumerable from the DB).
// `shows` are folder-path → show-id mappings cached for TV resolution.
func (n *scanNotifier) Forget(paths, shows, prefixes []string) {
	if n == nil || n.url == "" {
		return
	}
	if len(paths) == 0 && len(shows) == 0 && len(prefixes) == 0 {
		return
	}
	body, err := json.Marshal(map[string][]string{
		"paths":    paths,
		"shows":    shows,
		"prefixes": prefixes,
	})
	if err != nil {
		log.Printf("scan-state forget: marshal: %v", err)
		return
	}
	resp, err := n.http.Post(n.url+"/forget", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("scan-state forget: post: %v", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		log.Printf("scan-state forget: status %d", resp.StatusCode)
	}
}

// removeUnderBase is a guard-railed os.RemoveAll: refuses to operate on
// anything that isn't strictly *inside* mediaBase, so a malformed/empty
// SourcePath can't take out the wrong directory (or `/`). Empty paths
// no-op silently. Missing files no-op silently (target already gone).
func removeUnderBase(path, mediaBase string) error {
	if path == "" {
		return nil
	}
	if mediaBase == "" {
		return fmt.Errorf("media base path is not configured; refusing to delete %q", path)
	}
	cleanPath, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return fmt.Errorf("clean target: %w", err)
	}
	base, err := filepath.Abs(filepath.Clean(mediaBase))
	if err != nil {
		return fmt.Errorf("clean base: %w", err)
	}
	if base == "/" || base == "" {
		return fmt.Errorf("refusing to delete with root media base")
	}
	if !strings.HasPrefix(cleanPath, base+string(filepath.Separator)) {
		return fmt.Errorf("path %q is outside media base %q", cleanPath, base)
	}
	if err := os.RemoveAll(cleanPath); err != nil {
		return fmt.Errorf("remove %q: %w", cleanPath, err)
	}
	return nil
}

// parseBoolQuery returns true when the named query param is one of the
// usual truthy strings ("1", "true", "yes"). Empty/unrecognized → false.
func parseBoolQuery(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
