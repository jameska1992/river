package processor

import (
	"path/filepath"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"The Matrix", "The Matrix"},
		{"Schindler's List", "Schindler's List"}, // apostrophe is filesystem-safe
		{"Mission: Impossible", "Mission- Impossible"},
		{"Foo/Bar", "Foo-Bar"},
		{`Foo\Bar`, "Foo-Bar"},
		{"Q?A", "Q-A"},
		{`"Quoted"`, "Quoted"},
		{"  ", "untitled"},
		{"", "untitled"},
		{".hidden", "hidden"},
		{"trailing.", "trailing"},
		{"weird   spacing", "weird spacing"},
		{`*all|the:bad?chars"<>`, "all-the-bad-chars"},
		{"Foo--Bar", "Foo-Bar"},
		{"Spider-Man", "Spider-Man"}, // single legitimate hyphen preserved
	}
	for _, tc := range cases {
		if got := sanitizeFilename(tc.in); got != tc.want {
			t.Errorf("sanitizeFilename(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMovieOutputPath(t *testing.T) {
	got := movieOutputPath(
		"/mnt/lib/Movies/Inception (2010)/inception.mkv",
		"Inception", 2010,
		"/mnt/out",
	)
	want := filepath.Join("/mnt/out", "movies", "Inception (2010)", "Inception.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMovieOutputPath_SanitizesTitle(t *testing.T) {
	got := movieOutputPath(
		"/mnt/lib/Movies/Mission Impossible/mi.mkv",
		"Mission: Impossible", 2018,
		"/mnt/out",
	)
	want := filepath.Join("/mnt/out", "movies", "Mission- Impossible (2018)", "Mission- Impossible.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMovieOutputPath_YearZeroOmitsParens(t *testing.T) {
	got := movieOutputPath(
		"/mnt/lib/Movies/Inception/foo.mkv",
		"Inception", 0,
		"/mnt/out",
	)
	want := filepath.Join("/mnt/out", "movies", "Inception", "Inception.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMovieOutputPath_NoOutputDirPutsNextToSource(t *testing.T) {
	got := movieOutputPath(
		"/mnt/lib/Movies/Foo/foo.mkv",
		"Foo", 2020,
		"",
	)
	want := filepath.Join("/mnt/lib/Movies/Foo", "Foo.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEpisodeOutputPath(t *testing.T) {
	got := episodeOutputPath(
		"/mnt/lib/Shows/Breaking Bad/Season 01/episode_03.mkv",
		"Breaking Bad", 1, 3,
		"/mnt/out",
	)
	want := filepath.Join("/mnt/out", "shows", "Breaking Bad", "Season 1", "S01E03.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEpisodeOutputPath_SanitizesShow(t *testing.T) {
	got := episodeOutputPath(
		"/x/foo/file.mkv",
		"Marvel's Agents of S.H.I.E.L.D.", 2, 5,
		"/mnt/out",
	)
	// Trailing dots are stripped — Windows silently drops them, so keeping
	// them would mean cross-platform filesystem ambiguity.
	want := filepath.Join("/mnt/out", "shows", "Marvel's Agents of S.H.I.E.L.D", "Season 2", "S02E05.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestEpisodeOutputPath_ZeroPadsFile(t *testing.T) {
	got := filepath.Base(episodeOutputPath("/x/show/s10/file.mkv", "Foo", 10, 23, "/out"))
	if got != "S10E23.mp4" {
		t.Errorf("got %q, want S10E23.mp4", got)
	}
}

func TestEpisodeOutputPath_NoOutputDirPutsNextToSource(t *testing.T) {
	got := episodeOutputPath(
		"/mnt/lib/Shows/Foo/Season 01/ep.mkv",
		"Foo", 1, 1,
		"",
	)
	want := filepath.Join("/mnt/lib/Shows/Foo/Season 01", "S01E01.mp4")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
