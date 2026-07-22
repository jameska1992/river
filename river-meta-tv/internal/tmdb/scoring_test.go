package tmdb

import (
	"math"
	"testing"
)

func TestNormalizeTitle(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"The Matrix", "the matrix"},
		{"Spider-Man", "spider man"},
		{"Mission: Impossible", "mission impossible"},
		{"  The   Matrix  ", "the matrix"},
		{"Amélie", "amélie"},
		{"WALL·E", "walle"}, // middle dot dropped, letters kept
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeTitle(tc.in); got != tc.want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestTitleSimilarity(t *testing.T) {
	cases := []struct {
		a, b string
		want float64
	}{
		{"The Matrix", "The Matrix", 1.0},
		{"The Matrix", "the matrix", 1.0},
		{"Spider-Man", "Spider Man", 1.0},
		{"Mission: Impossible", "Mission Impossible", 1.0},
		{"The Matrix", "The Matrix Reloaded", 2.0 / 3.0}, // {the,matrix} ∩ {the,matrix,reloaded}
		{"Matrix", "The Matrix", 0.5},                    // {matrix} ∩ {the,matrix}
		{"", "anything", 0},
		{"Pinocchio", "Pinocchio", 1.0},
	}
	for _, tc := range cases {
		got := titleSimilarity(tc.a, tc.b)
		if math.Abs(got-tc.want) > 1e-9 {
			t.Errorf("titleSimilarity(%q,%q) = %f, want %f", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestYearFactor(t *testing.T) {
	cases := []struct {
		q, c int
		want float64
	}{
		{0, 1999, 1.0},  // no query year → neutral
		{1999, 0, 0.7},  // candidate has no year → soft penalty
		{1999, 1999, 1.0},
		{1999, 2000, 0.9},
		{1999, 1998, 0.9},
		{1999, 2001, 0.6},
		{1999, 2005, 0.3},
	}
	for _, tc := range cases {
		if got := yearFactor(tc.q, tc.c); got != tc.want {
			t.Errorf("yearFactor(%d,%d) = %f, want %f", tc.q, tc.c, got, tc.want)
		}
	}
}

func TestBestMatch(t *testing.T) {
	t.Run("picks exact title + year over high-popularity decoy", func(t *testing.T) {
		// User asked for "Pinocchio" (2022). The 1940 Disney classic has higher
		// popularity but the 2022 remake is the right pick by year.
		results := []searchCandidate{
			{ID: 1, Title: "Pinocchio", Year: 1940, Popularity: 50.0},
			{ID: 2, Title: "Pinocchio", Year: 2022, Popularity: 10.0},
		}
		best, score := bestMatch("Pinocchio", 2022, results)
		if best.ID != 2 {
			t.Errorf("got id=%d, want 2", best.ID)
		}
		if score < matchThreshold {
			t.Errorf("expected score above threshold, got %f", score)
		}
	})

	t.Run("popularity tiebreak when scores equal", func(t *testing.T) {
		// No year info → all "Pinocchio" candidates score 1.0; popularity wins.
		results := []searchCandidate{
			{ID: 1, Title: "Pinocchio", Year: 1940, Popularity: 50.0},
			{ID: 2, Title: "Pinocchio", Year: 2022, Popularity: 10.0},
		}
		best, _ := bestMatch("Pinocchio", 0, results)
		if best.ID != 1 {
			t.Errorf("got id=%d, want 1 (higher popularity)", best.ID)
		}
	})

	t.Run("falls below threshold when title is wrong", func(t *testing.T) {
		// Query "The Matrix" (1999) against unrelated candidates → ErrNotFound.
		results := []searchCandidate{
			{ID: 1, Title: "The Matrix Reloaded", Year: 2003, Popularity: 50.0},
			{ID: 2, Title: "Pinocchio", Year: 1940, Popularity: 100.0},
		}
		_, score := bestMatch("The Matrix", 1999, results)
		if score >= matchThreshold {
			t.Errorf("expected score < threshold, got %f", score)
		}
	})

	t.Run("year off by one still passes", func(t *testing.T) {
		results := []searchCandidate{
			{ID: 1, Title: "The Matrix", Year: 1999, Popularity: 50.0},
		}
		_, score := bestMatch("The Matrix", 2000, results)
		if score < matchThreshold {
			t.Errorf("expected ≥threshold for ±1 year, got %f", score)
		}
		if math.Abs(score-0.9) > 1e-9 {
			t.Errorf("expected score 0.9, got %f", score)
		}
	})

	t.Run("original_title match wins when localized title differs", func(t *testing.T) {
		// Localized title is unrelated; original_title matches the query.
		results := []searchCandidate{
			{ID: 1, Title: "Crouching Tiger, Hidden Dragon", OriginalTitle: "Wo hu cang long", Year: 2000, Popularity: 30.0},
		}
		_, score := bestMatch("Wo hu cang long", 2000, results)
		if math.Abs(score-1.0) > 1e-9 {
			t.Errorf("expected 1.0 via original_title, got %f", score)
		}
	})

	t.Run("empty results", func(t *testing.T) {
		_, score := bestMatch("anything", 2020, nil)
		if score != 0 {
			t.Errorf("expected 0, got %f", score)
		}
	})
}
