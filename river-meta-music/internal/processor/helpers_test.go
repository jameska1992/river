package processor

import (
	"testing"

	"river-meta-music/internal/apiclient"
)

func TestNormalizeTitle(t *testing.T) {
	cases := map[string]string{
		"The Beatles": "the beatles",
		"  Miles Davis  ": "miles davis",
		"QUEEN": "queen",
		"":      "",
	}
	for in, want := range cases {
		if got := normalizeTitle(in); got != want {
			t.Errorf("normalizeTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFindArtist(t *testing.T) {
	artists := []apiclient.Artist{
		{ID: "1", Name: "The Beatles"},
		{ID: "2", Name: "Miles Davis"},
	}

	// Case-insensitive + whitespace-insensitive match.
	if got := findArtist(artists, "  the BEATLES "); got == nil || got.ID != "1" {
		t.Errorf("findArtist should match The Beatles case/space-insensitively, got %+v", got)
	}
	if got := findArtist(artists, "Miles Davis"); got == nil || got.ID != "2" {
		t.Errorf("findArtist exact match failed, got %+v", got)
	}
	if got := findArtist(artists, "Nobody"); got != nil {
		t.Errorf("findArtist should return nil for a miss, got %+v", got)
	}
	if got := findArtist(nil, "The Beatles"); got != nil {
		t.Errorf("findArtist on empty slice should be nil, got %+v", got)
	}
}

func TestCoalesce(t *testing.T) {
	if got := coalesce("first", "second"); got != "first" {
		t.Errorf("coalesce(non-zero, x) = %q, want first", got)
	}
	if got := coalesce("", "fallback"); got != "fallback" {
		t.Errorf("coalesce(zero, y) = %q, want fallback", got)
	}
	if got := coalesce(0, 42); got != 42 {
		t.Errorf("coalesce(0, 42) = %d, want 42", got)
	}
	if got := coalesce(7, 42); got != 7 {
		t.Errorf("coalesce(7, 42) = %d, want 7", got)
	}
}
