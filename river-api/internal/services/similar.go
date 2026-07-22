package services

import (
	"encoding/json"
	"sort"
	"time"
)

// similarSourceLoadCap bounds how many records the "similar" services
// pull into memory to rank. Chosen well above realistic library sizes
// so it's effectively "load all," but low enough that a runaway rescan
// during development can't spin the API into GC-thrash. If it starts
// being hit routinely in production, revisit the ranking design (see
// the note on MovieService.Similar).
const similarSourceLoadCap = 20000

// SimilarItem is the trimmed shape returned by /movies/:id/similar,
// /tvshows/:id/similar, /audiobooks/:id/similar. It carries just what
// a client-side card needs — id, type, title, year, poster, backdrop —
// so the response stays small even for a 16-item list on a slow WAN
// connection.
type SimilarItem struct {
	ID           string `json:"id"`
	Type         string `json:"type"` // "movie" | "tvshow" | "audiobook"
	Title        string `json:"title"`
	Year         int    `json:"year,omitempty"`
	PosterPath   string `json:"poster_path"`
	BackdropPath string `json:"backdrop_path,omitempty"`
}

// similarCandidate is the per-item input to the shared ranker. The
// movie / tvshow / audiobook services translate their model rows into
// this shape before scoring so the ranking logic doesn't need to know
// about the concrete types.
type similarCandidate struct {
	ID           string
	Genres       []string
	Rating       float32 // 0 for types without a rating (audiobooks)
	CreatedAt    time.Time
	Title        string
	Year         int
	PosterPath   string
	BackdropPath string
}

// rankBySharedGenres scores candidates by the number of genres they
// share with source, drops zeros, sorts by (shared desc, rating desc,
// created_at desc), and returns up to limit results. The source ID is
// filtered out so a self-reference can't slip in — callers don't need
// to pre-filter.
//
// Genre comparison is case-insensitive so "Sci-Fi" and "sci-fi"
// collide sensibly across enrichment sources.
func rankBySharedGenres(
	sourceID string,
	sourceGenres []string,
	candidates []similarCandidate,
	limit int,
) []similarCandidate {
	if len(sourceGenres) == 0 || len(candidates) == 0 || limit <= 0 {
		return nil
	}
	// Build a set of lowercased source genres so per-candidate scoring
	// is a simple map lookup rather than an O(N) walk.
	sourceSet := make(map[string]struct{}, len(sourceGenres))
	for _, g := range sourceGenres {
		if g == "" {
			continue
		}
		sourceSet[normaliseGenre(g)] = struct{}{}
	}
	if len(sourceSet) == 0 {
		return nil
	}

	type scored struct {
		c     similarCandidate
		share int
	}
	scoredList := make([]scored, 0, len(candidates))
	for _, c := range candidates {
		if c.ID == sourceID {
			continue
		}
		share := 0
		for _, g := range c.Genres {
			if _, ok := sourceSet[normaliseGenre(g)]; ok {
				share++
			}
		}
		if share == 0 {
			continue
		}
		scoredList = append(scoredList, scored{c: c, share: share})
	}

	sort.SliceStable(scoredList, func(i, j int) bool {
		if scoredList[i].share != scoredList[j].share {
			return scoredList[i].share > scoredList[j].share
		}
		if scoredList[i].c.Rating != scoredList[j].c.Rating {
			return scoredList[i].c.Rating > scoredList[j].c.Rating
		}
		return scoredList[i].c.CreatedAt.After(scoredList[j].c.CreatedAt)
	})

	if len(scoredList) > limit {
		scoredList = scoredList[:limit]
	}
	out := make([]similarCandidate, len(scoredList))
	for i, s := range scoredList {
		out[i] = s.c
	}
	return out
}

// normaliseGenre folds case + trims whitespace so "  Sci-Fi ", "sci-fi"
// and "Sci-Fi" all compare equal. Deliberately does not touch
// punctuation — "Sci-Fi" vs "Science Fiction" is a metadata mismatch we
// don't want to paper over silently.
func normaliseGenre(g string) string {
	out := make([]byte, 0, len(g))
	started := false
	trailingSpace := 0
	for i := 0; i < len(g); i++ {
		c := g[i]
		if c == ' ' || c == '\t' {
			if !started {
				continue
			}
			trailingSpace++
			continue
		}
		if trailingSpace > 0 {
			for j := 0; j < trailingSpace; j++ {
				out = append(out, ' ')
			}
			trailingSpace = 0
		}
		started = true
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		out = append(out, c)
	}
	return string(out)
}

// parseGenresJSON decodes a JSON-encoded []string as stored on the
// Movie / TVShow models' Genres column. Missing / malformed values
// yield an empty slice — callers can treat that as "no genres" and
// short-circuit similarity computation.
func parseGenresJSON(s string) []string {
	if s == "" || s == "[]" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	// Filter blanks that a sloppy scraper might have left behind.
	filtered := out[:0]
	for _, g := range out {
		if g != "" {
			filtered = append(filtered, g)
		}
	}
	return filtered
}

// candidatesToSimilarItems trims candidate structs into wire-format
// SimilarItems. Kept as a service-package helper because the mapping
// only makes sense in the ranker's context (candidate → wire item).
func candidatesToSimilarItems(cs []similarCandidate, typ string) []SimilarItem {
	out := make([]SimilarItem, len(cs))
	for i, c := range cs {
		out[i] = SimilarItem{
			ID:           c.ID,
			Type:         typ,
			Title:        c.Title,
			Year:         c.Year,
			PosterPath:   c.PosterPath,
			BackdropPath: c.BackdropPath,
		}
	}
	return out
}
