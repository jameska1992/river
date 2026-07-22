package scanner

import (
	"fmt"
	"strings"
	"sync"
	"unicode"

	"river-scan/internal/apiclient"
)

// scanCache caches the per-library movie list for a single scan run.
//
// FindOrCreateMovie on the API client paginates /api/movies on every call;
// in a flat layout where each video file becomes its own movie event, doing
// that per file is the bottleneck — a 3000-file library would issue 3000
// paginated list calls. The cache loads the list once per library, dedups
// new creates back into the map, and serves subsequent lookups from memory.
//
// One instance per scan run. The instance lives only as long as the Scanner
// call that built it.
//
// The map value is a slice rather than a single movie because two films
// can legitimately share a title and differ only by year (e.g. "The
// Italian Job" 1969 vs 2003 — remakes are common). Pre-slice the cache
// keyed on title alone, the second one to be discovered would silently
// collapse onto the first.
type scanCache struct {
	api    *apiclient.Client
	mu     sync.Mutex
	loaded map[string]bool                         // libraryID -> already populated
	byLib  map[string]map[string][]apiclient.Movie // libraryID -> normalized title -> movies (year-disambiguated)
}

func newScanCache(api *apiclient.Client) *scanCache {
	return &scanCache{
		api:    api,
		loaded: make(map[string]bool),
		byLib:  make(map[string]map[string][]apiclient.Movie),
	}
}

// findOrCreateMovie returns the existing movie matching title+year in
// libraryID, or creates a new one. New creates are appended to the cache
// so a second call with the same title+year hits memory. sourcePath is
// the original on-disk location of the movie file — recorded on creation
// so the stream endpoint can fall back to it before video-trans finalizes
// FilePath; for cache hits, the caller is responsible for any backfill
// they want via UpdateMovieSourcePath.
//
// Year-aware match (mirroring meta-tv's findShow):
//   - exact year match → return that record
//   - if either side has year=0 → tentative fallback (used only if no
//     exact match exists for the same title)
//   - both sides have non-zero, differing years → no match, create a new
//     record
func (c *scanCache) findOrCreateMovie(libraryID, title string, year int, sourcePath string) (*apiclient.Movie, error) {
	if err := c.ensureLoaded(libraryID); err != nil {
		return nil, err
	}
	needle := normalizeTitleKey(title)

	c.mu.Lock()
	if m := pickMovieByYear(c.byLib[libraryID][needle], year); m != nil {
		out := *m
		c.mu.Unlock()
		return &out, nil
	}
	c.mu.Unlock()

	created, err := c.api.CreateMovie(apiclient.MovieRequest{
		LibraryID:  libraryID,
		Title:      title,
		Year:       year,
		SourcePath: sourcePath,
	})
	if err != nil {
		return nil, fmt.Errorf("create movie %q: %w", title, err)
	}

	c.mu.Lock()
	c.byLib[libraryID][needle] = append(c.byLib[libraryID][needle], *created)
	c.mu.Unlock()
	return created, nil
}

// pickMovieByYear chooses the best candidate from a set of same-title
// movies. Returns nil when nothing matches — the caller treats that as
// "create a new record". Exported (lowercase) only for testing.
func pickMovieByYear(candidates []apiclient.Movie, year int) *apiclient.Movie {
	var fallback *apiclient.Movie
	for i := range candidates {
		if year > 0 && candidates[i].Year == year {
			return &candidates[i]
		}
		if (year == 0 || candidates[i].Year == 0) && fallback == nil {
			fallback = &candidates[i]
		}
	}
	return fallback
}

func (c *scanCache) ensureLoaded(libraryID string) error {
	c.mu.Lock()
	if c.loaded[libraryID] {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	list, err := c.api.ListMovies(libraryID)
	if err != nil {
		return fmt.Errorf("list movies for library %s: %w", libraryID, err)
	}
	m := make(map[string][]apiclient.Movie, len(list))
	for _, mov := range list {
		key := normalizeTitleKey(mov.Title)
		m[key] = append(m[key], mov)
	}

	c.mu.Lock()
	c.byLib[libraryID] = m
	c.loaded[libraryID] = true
	c.mu.Unlock()
	return nil
}

// normalizeTitleKey is the in-memory cache equivalent of apiclient.matchKey
// and must stay in lock-step with it. Letters and digits survive lowercased,
// apostrophes are stripped, anything else becomes whitespace and folds. See
// the comment on apiclient.matchKey for the cross-source normalization
// failure modes this prevents.
func normalizeTitleKey(title string) string {
	var b strings.Builder
	b.Grow(len(title))
	for _, r := range title {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case r == '\'' || r == '’':
			// Skip entirely.
		default:
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
