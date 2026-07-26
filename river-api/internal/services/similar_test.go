package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormaliseGenre(t *testing.T) {
	cases := map[string]string{
		"Sci-Fi":          "sci-fi",
		"  Sci-Fi ":       "sci-fi",
		"\tAction\t":      "action",
		"Science Fiction": "science fiction", // internal space preserved
		"ROM  COM":        "rom  com",        // internal runs preserved
		"":                "",
	}
	for in, want := range cases {
		assert.Equalf(t, want, normaliseGenre(in), "normaliseGenre(%q)", in)
	}
}

func TestRankBySharedGenres_EmptyInputs(t *testing.T) {
	c := []similarCandidate{{ID: "a", Genres: []string{"action"}}}
	assert.Nil(t, rankBySharedGenres("x", nil, c, 10), "no source genres → nil")
	assert.Nil(t, rankBySharedGenres("x", []string{"action"}, nil, 10), "no candidates → nil")
	assert.Nil(t, rankBySharedGenres("x", []string{"action"}, c, 0), "limit 0 → nil")
	assert.Nil(t, rankBySharedGenres("x", []string{"", ""}, c, 10), "only empty source genres → nil")
}

func TestRankBySharedGenres_OrderingAndFilters(t *testing.T) {
	now := time.Now()
	src := []string{"Action", "Sci-Fi", "Drama"}
	candidates := []similarCandidate{
		{ID: "self", Genres: []string{"Action", "Sci-Fi"}},                        // filtered: same id as source
		{ID: "none", Genres: []string{"Comedy"}},                                  // dropped: zero shared
		{ID: "one-lo", Genres: []string{"action"}, Rating: 2, CreatedAt: now},     // share 1
		{ID: "one-hi", Genres: []string{"drama"}, Rating: 9, CreatedAt: now},      // share 1, higher rating
		{ID: "two", Genres: []string{"sci-fi", "action"}, Rating: 1},              // share 2 → first
		{ID: "one-new", Genres: []string{"Drama"}, Rating: 9, CreatedAt: now.Add(time.Hour)}, // share 1, ties rating with one-hi, newer
	}

	got := rankBySharedGenres("self", src, candidates, 10)
	ids := make([]string, len(got))
	for i, c := range got {
		ids[i] = c.ID
	}

	// two (share 2) first; then the share-1 group ordered by rating desc,
	// breaking rating ties by created_at desc; "none" and "self" excluded.
	assert.Equal(t, []string{"two", "one-new", "one-hi", "one-lo"}, ids)
}

func TestRankBySharedGenres_LimitTruncates(t *testing.T) {
	candidates := []similarCandidate{
		{ID: "a", Genres: []string{"action"}, Rating: 3},
		{ID: "b", Genres: []string{"action"}, Rating: 2},
		{ID: "c", Genres: []string{"action"}, Rating: 1},
	}
	got := rankBySharedGenres("src", []string{"Action"}, candidates, 2)
	require.Len(t, got, 2)
	assert.Equal(t, "a", got[0].ID)
	assert.Equal(t, "b", got[1].ID)
}

func TestCandidatesToSimilarItems(t *testing.T) {
	in := []similarCandidate{{
		ID: "id1", Genres: []string{"x"}, Title: "T", Year: 1999,
		PosterPath: "/p.jpg", BackdropPath: "/b.jpg",
	}}
	out := candidatesToSimilarItems(in, "movie")
	require.Len(t, out, 1)
	assert.Equal(t, SimilarItem{
		ID: "id1", Type: "movie", Title: "T", Year: 1999,
		PosterPath: "/p.jpg", BackdropPath: "/b.jpg",
	}, out[0])
	assert.Empty(t, candidatesToSimilarItems(nil, "movie"), "nil in → empty out")
}
