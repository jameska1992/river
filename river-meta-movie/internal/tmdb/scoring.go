package tmdb

import (
	"strings"
	"unicode"
)

// matchThreshold is the minimum score a search candidate must clear for us to
// trust it as the right movie. Below this we'd rather return ErrNotFound and
// let the caller log the miss than silently bind to the wrong record.
//
// Reference points (tuned against the test table in scoring_test.go):
//   - exact title + exact year                              → 1.00
//   - exact title + year off by 1                           → 0.90
//   - exact title + year unknown                            → 0.70 (penalty for missing)
//   - 3-of-4 token overlap + exact year                     → 0.75
//   - exact title + year off by ≥3                          → 0.30  (rejected)
const matchThreshold = 0.5

// searchCandidate is the subset of TMDB /search/movie hit fields we need for
// scoring. Kept separate from the wire struct so scoring is testable without
// pulling in the HTTP layer.
type searchCandidate struct {
	ID            int
	Title         string
	OriginalTitle string
	Year          int
	Popularity    float64
}

// bestMatch returns the candidate with the highest score against the query.
// Returns the zero candidate and score 0 when results is empty. Ties on score
// are broken by TMDB popularity (more popular wins) — this mirrors what TMDB
// would have ranked first anyway, but only after we've established a real
// title/year match.
func bestMatch(queryTitle string, queryYear int, results []searchCandidate) (searchCandidate, float64) {
	var best searchCandidate
	var bestScore float64
	for _, r := range results {
		s := scoreCandidate(queryTitle, queryYear, r)
		if s > bestScore || (s == bestScore && r.Popularity > best.Popularity) {
			best = r
			bestScore = s
		}
	}
	return best, bestScore
}

// scoreCandidate combines title similarity and year proximity into a single
// 0..1 score. Title similarity is taken as the max across the localized
// title and the original_title — TMDB often stores foreign films under their
// original title and the localized one is what the user typed.
func scoreCandidate(queryTitle string, queryYear int, r searchCandidate) float64 {
	t1 := titleSimilarity(queryTitle, r.Title)
	t2 := titleSimilarity(queryTitle, r.OriginalTitle)
	ts := t1
	if t2 > ts {
		ts = t2
	}
	return ts * yearFactor(queryYear, r.Year)
}

// yearFactor weights a candidate by how close its release year is to the
// query year. queryYear==0 means "no year info" and is treated neutrally;
// candidateYear==0 means TMDB has no release date yet, which is a small
// signal that the record is unfinished — apply a soft penalty.
func yearFactor(queryYear, candidateYear int) float64 {
	if queryYear == 0 {
		return 1.0
	}
	if candidateYear == 0 {
		return 0.7
	}
	delta := queryYear - candidateYear
	if delta < 0 {
		delta = -delta
	}
	switch delta {
	case 0:
		return 1.0
	case 1:
		return 0.9
	case 2:
		return 0.6
	default:
		return 0.3
	}
}

// titleSimilarity returns 1.0 for normalized-exact match, else a Jaccard
// similarity over word tokens. Jaccard is set-based, which makes it robust
// to word-order variants ("The Matrix" vs "Matrix, The") and to extra
// noise tokens the normalizer didn't strip.
func titleSimilarity(a, b string) float64 {
	na := normalizeTitle(a)
	nb := normalizeTitle(b)
	if na == "" || nb == "" {
		return 0
	}
	if na == nb {
		return 1.0
	}
	return jaccard(strings.Fields(na), strings.Fields(nb))
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}
	aset := make(map[string]struct{}, len(a))
	for _, t := range a {
		aset[t] = struct{}{}
	}
	bset := make(map[string]struct{}, len(b))
	for _, t := range b {
		bset[t] = struct{}{}
	}
	inter := 0
	for t := range aset {
		if _, ok := bset[t]; ok {
			inter++
		}
	}
	union := len(aset) + len(bset) - inter
	if union == 0 {
		return 0
	}
	return float64(inter) / float64(union)
}

// normalizeTitle lowercases, replaces separators with spaces, drops all
// other punctuation, and collapses whitespace. The result is suitable for
// equality comparison and for token-based similarity.
func normalizeTitle(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(unicode.ToLower(r))
		case unicode.IsSpace(r), r == '-', r == '_', r == '.', r == ':':
			b.WriteRune(' ')
		}
	}
	return strings.Join(strings.Fields(b.String()), " ")
}
