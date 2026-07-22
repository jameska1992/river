package scanner

import (
	"testing"

	"river-scan/internal/apiclient"
)

func TestPickMovieByYear(t *testing.T) {
	italianJob1969 := apiclient.Movie{ID: "a", Title: "The Italian Job", Year: 1969}
	italianJob2003 := apiclient.Movie{ID: "b", Title: "The Italian Job", Year: 2003}
	noYear := apiclient.Movie{ID: "c", Title: "The Italian Job"}

	cases := []struct {
		name       string
		candidates []apiclient.Movie
		year       int
		wantID     string // "" → expect nil
	}{
		// The bug we just fixed: two same-title movies with different
		// years must resolve to the one with the matching year rather
		// than collapsing onto whichever was discovered first.
		{
			name:       "exact year picks the right remake",
			candidates: []apiclient.Movie{italianJob1969, italianJob2003},
			year:       2003,
			wantID:     "b",
		},
		{
			name:       "exact year picks the original",
			candidates: []apiclient.Movie{italianJob1969, italianJob2003},
			year:       1969,
			wantID:     "a",
		},
		// Year missing on either side is a fallback — used only if no
		// exact match. This mirrors meta-tv's findShow semantics so an
		// unenriched record (year=0) can still be reattached on a
		// subsequent scan once the year is parsed.
		{
			name:       "incoming year zero falls back to first candidate",
			candidates: []apiclient.Movie{italianJob1969, italianJob2003},
			year:       0,
			wantID:     "a",
		},
		{
			name:       "stored year zero is a fallback when incoming has year",
			candidates: []apiclient.Movie{noYear},
			year:       1999,
			wantID:     "c",
		},
		// Both sides have year and they don't match → no candidate.
		// This is what makes the new movie get created instead of
		// collapsing onto the wrong existing record.
		{
			name:       "differing years yield no match",
			candidates: []apiclient.Movie{italianJob1969},
			year:       2003,
			wantID:     "",
		},
		{
			name:       "empty candidates",
			candidates: nil,
			year:       2003,
			wantID:     "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := pickMovieByYear(tc.candidates, tc.year)
			if tc.wantID == "" {
				if got != nil {
					t.Fatalf("want nil, got %+v", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("want match with ID %q, got nil", tc.wantID)
			}
			if got.ID != tc.wantID {
				t.Fatalf("want match with ID %q, got %q", tc.wantID, got.ID)
			}
		})
	}
}
