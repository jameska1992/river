package processor

import (
	"path/filepath"
	"reflect"
	"sort"
	"testing"
)

func TestParseDirName(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantYear  int
	}{
		{"Abbey Road (1969)", "Abbey Road", 1969},
		{"Kind of Blue (1959) ", "Kind of Blue", 1959}, // trailing space
		{"Greatest Hits", "Greatest Hits", 0},          // no year
	}
	for _, c := range cases {
		title, year := parseDirName(c.in)
		if title != c.wantTitle || year != c.wantYear {
			t.Errorf("parseDirName(%q) = (%q, %d), want (%q, %d)", c.in, title, year, c.wantTitle, c.wantYear)
		}
	}
}

func TestParseTrackNumber(t *testing.T) {
	cases := map[string]int{
		"01 - Hey Jude.flac":   1,
		"05. Yesterday.mp3":    5,
		"12 Come Together.mp3": 12,
		"Track 07 - Blue.mp3":  7,
		"No Number.mp3":        0,
	}
	for in, want := range cases {
		if got := parseTrackNumber(in); got != want {
			t.Errorf("parseTrackNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseChapterNumber(t *testing.T) {
	cases := map[string]int{
		"Chapter 01 - Intro.mp3": 1,
		"01 - Opening.mp3":       1,
		"Part 3 - Middle.mp3":    3,
		"Prologue.mp3":           0,
	}
	for in, want := range cases {
		if got := parseChapterNumber(in); got != want {
			t.Errorf("parseChapterNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseAudioTitle(t *testing.T) {
	cases := map[string]string{
		"01 - Hey Jude.flac":    "Hey Jude",   // numeric prefix stripped
		"03 - Song 320k.mp3":    "Song",       // quality tag stripped
		"07.Track.Name.flac":    "Track Name", // dots → spaces
		"Plain Title.mp3":       "Plain Title",
	}
	for in, want := range cases {
		if got := parseAudioTitle(in); got != want {
			t.Errorf("parseAudioTitle(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGroupByAlbum(t *testing.T) {
	dir := filepath.FromSlash("/lib/Artist")
	f := func(p string) string { return filepath.FromSlash(p) }
	files := []string{
		f("/lib/Artist/Album1/01.flac"),
		f("/lib/Artist/Album1/02.flac"),
		f("/lib/Artist/Album2/01.flac"),
		f("/lib/Artist/loose.flac"),         // directly under dir → grouped by dir base
		f("/lib/Artist/Album1/CD1/03.flac"), // nested → collapses to Album1
	}
	groups := groupByAlbum(dir, files)

	if len(groups["Album1"]) != 3 {
		t.Errorf("Album1 = %d files, want 3 (incl. nested CD1)", len(groups["Album1"]))
	}
	if len(groups["Album2"]) != 1 {
		t.Errorf("Album2 = %d files, want 1", len(groups["Album2"]))
	}
	if len(groups["Artist"]) != 1 {
		t.Errorf("loose file should group under the dir base %q, got %d", "Artist", len(groups["Artist"]))
	}
}

func TestSortedFiles(t *testing.T) {
	in := []string{
		filepath.FromSlash("/a/03.mp3"),
		filepath.FromSlash("/a/01.mp3"),
		filepath.FromSlash("/a/02.mp3"),
	}
	got := sortedFiles(in)
	want := []string{
		filepath.FromSlash("/a/01.mp3"),
		filepath.FromSlash("/a/02.mp3"),
		filepath.FromSlash("/a/03.mp3"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sortedFiles = %v, want %v", got, want)
	}
	// The input slice must not be mutated.
	if sort.SliceIsSorted(in, func(i, j int) bool { return in[i] < in[j] }) {
		t.Error("sortedFiles must not sort the caller's slice in place")
	}
}
