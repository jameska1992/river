package processor

import (
	"testing"

	"river-meta-book/internal/apiclient"
)

func TestParseDirectoryName(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantYear  int
	}{
		{"Dracula (1897)", "Dracula", 1897},
		{"The Hobbit (1937)", "The Hobbit", 1937},
		{"Moby Dick", "Moby Dick", 0}, // no year
		// The (YYYY) must sit at the very end — a trailing space defeats the
		// pattern, so the whole name is kept and year stays 0.
		{"Frankenstein (1818) ", "Frankenstein (1818) ", 0},
	}
	for _, c := range cases {
		title, year := parseDirectoryName(c.in)
		if title != c.wantTitle || year != c.wantYear {
			t.Errorf("parseDirectoryName(%q) = (%q, %d), want (%q, %d)", c.in, title, year, c.wantTitle, c.wantYear)
		}
	}
}

func TestFindAudiobook(t *testing.T) {
	books := []apiclient.Audiobook{
		{ID: "1", Title: "Dune"},
		{ID: "2", Title: "1984"},
	}

	if got := findAudiobook(books, "  DUNE "); got == nil || got.ID != "1" {
		t.Errorf("findAudiobook should match case/space-insensitively, got %+v", got)
	}
	if got := findAudiobook(books, "1984"); got == nil || got.ID != "2" {
		t.Errorf("findAudiobook exact match failed, got %+v", got)
	}
	if got := findAudiobook(books, "Unknown"); got != nil {
		t.Errorf("findAudiobook miss should be nil, got %+v", got)
	}
	if got := findAudiobook(nil, "Dune"); got != nil {
		t.Errorf("findAudiobook on empty slice should be nil, got %+v", got)
	}
}

func TestCoalesce(t *testing.T) {
	if got := coalesce(0, 1897); got != 1897 {
		t.Errorf("coalesce(0, 1897) = %d, want 1897", got)
	}
	if got := coalesce(1922, 1897); got != 1922 {
		t.Errorf("coalesce(1922, 1897) = %d, want 1922", got)
	}
}
