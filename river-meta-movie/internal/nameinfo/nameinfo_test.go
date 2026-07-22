package nameinfo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in         string
		wantTitle  string
		wantYear   int
		wantTMDB   int
		wantIMDB   string
	}{
		// canonical "Title (YYYY)"
		{"The Matrix (1999)", "The Matrix", 1999, 0, ""},
		{"Inception (2010)", "Inception", 2010, 0, ""},

		// release-name patterns
		{"The.Matrix.1999.1080p.BluRay.x264-GROUP", "The Matrix", 1999, 0, ""},
		{"Avatar.The.Way.of.Water.2022.UHD.2160p.HDR.HEVC-RELEASE", "Avatar The Way of Water", 2022, 0, ""},
		{"Inception_2010_1080p_BluRay_x264", "Inception", 2010, 0, ""},
		{"Mad.Max.Fury.Road.2015.REMUX.UHD.2160p.HDR.DV.HEVC.TrueHD.7.1-FraMeSToR", "Mad Max Fury Road", 2015, 0, ""},

		// titles that LOOK like years should keep them when no other year is present
		{"2001 A Space Odyssey", "2001 A Space Odyssey", 0, 0, ""},
		{"2001.A.Space.Odyssey.1968.1080p.BluRay", "2001 A Space Odyssey", 1968, 0, ""},
		{"1917 (2019)", "1917", 2019, 0, ""},

		// embedded Plex/Jellyfin IDs
		{"The Matrix (1999) {tmdb-603}", "The Matrix", 1999, 603, ""},
		{"The Matrix (1999) {imdb-tt0133093}", "The Matrix", 1999, 0, "tt0133093"},
		{"The Matrix (1999) [tmdbid-603]", "The Matrix", 1999, 603, ""},
		{"The Matrix (1999) [imdbid-tt0133093] [tmdbid-603]", "The Matrix", 1999, 603, "tt0133093"},
		{"Movie.Name.2020.tt1234567.1080p", "Movie Name", 2020, 0, "tt1234567"},

		// Plex edition tags should be stripped from title
		{"Blade Runner (1982) {edition-Final Cut}", "Blade Runner", 1982, 0, ""},

		// punctuation preserved where meaningful
		{"Mission: Impossible (2018)", "Mission: Impossible", 2018, 0, ""},

		// hyphens in titles become spaces (TMDB tolerates this)
		{"Spider-Man (2002)", "Spider Man", 2002, 0, ""},

		// resolution-like numbers (2160) must not be confused with year
		{"Some.Movie.2021.2160p.HDR", "Some Movie", 2021, 0, ""},

		// no year, no tags
		{"Just A Plain Name", "Just A Plain Name", 0, 0, ""},

		// Bracketed year prefix + release noise + release group (Dr No case):
		// year survives the bracket strip; "BrRip" boundary cuts the trailing
		// release group name. Collection prefix "007" stays — RetryTitles
		// strips it on the second TMDB attempt.
		{"[1962] 007 Dr No BrRip Pimp4003", "007 Dr No", 1962, 0, ""},
		{"[2010] Inception 1080p BluRay-RARBG", "Inception", 2010, 0, ""},
		// Release-group name with no static-list entry — boundary slice
		// still kills it because "x264" comes before it.
		{"Foo.Bar.2018.x264-WHATEVERGROUP", "Foo Bar", 2018, 0, ""},
	}

	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			got := Parse(tc.in)
			if got.Title != tc.wantTitle {
				t.Errorf("Title: got %q, want %q", got.Title, tc.wantTitle)
			}
			if got.Year != tc.wantYear {
				t.Errorf("Year: got %d, want %d", got.Year, tc.wantYear)
			}
			if got.TMDBID != tc.wantTMDB {
				t.Errorf("TMDBID: got %d, want %d", got.TMDBID, tc.wantTMDB)
			}
			if got.IMDBID != tc.wantIMDB {
				t.Errorf("IMDBID: got %q, want %q", got.IMDBID, tc.wantIMDB)
			}
		})
	}
}

func TestReadNFO_KodiMovieNFO(t *testing.T) {
	dir := t.TempDir()
	nfo := `<?xml version="1.0" encoding="UTF-8"?>
<movie>
  <title>The Matrix</title>
  <year>1999</year>
  <imdb_id>tt0133093</imdb_id>
  <tmdbid>603</tmdbid>
</movie>`
	if err := os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte(nfo), 0644); err != nil {
		t.Fatal(err)
	}
	tmdb, imdb, ok := ReadNFO(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tmdb != 603 || imdb != "tt0133093" {
		t.Errorf("got tmdb=%d imdb=%q, want 603/tt0133093", tmdb, imdb)
	}
}

func TestReadNFO_UniqueID(t *testing.T) {
	dir := t.TempDir()
	nfo := `<movie>
  <uniqueid type="imdb" default="true">tt0133093</uniqueid>
  <uniqueid type="tmdb">603</uniqueid>
</movie>`
	if err := os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte(nfo), 0644); err != nil {
		t.Fatal(err)
	}
	tmdb, imdb, ok := ReadNFO(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tmdb != 603 || imdb != "tt0133093" {
		t.Errorf("got tmdb=%d imdb=%q, want 603/tt0133093", tmdb, imdb)
	}
}

func TestReadNFO_BareIMDB(t *testing.T) {
	dir := t.TempDir()
	nfo := `Some random release notes
https://www.imdb.com/title/tt0133093/
Encoded by Anonymous`
	if err := os.WriteFile(filepath.Join(dir, "release.nfo"), []byte(nfo), 0644); err != nil {
		t.Fatal(err)
	}
	tmdb, imdb, ok := ReadNFO(dir)
	if !ok {
		t.Fatal("expected ok=true")
	}
	if tmdb != 0 || imdb != "tt0133093" {
		t.Errorf("got tmdb=%d imdb=%q, want 0/tt0133093", tmdb, imdb)
	}
}

func TestReadNFO_MissingDir(t *testing.T) {
	tmdb, imdb, ok := ReadNFO(filepath.Join(t.TempDir(), "does-not-exist"))
	if ok || tmdb != 0 || imdb != "" {
		t.Errorf("expected zero result, got tmdb=%d imdb=%q ok=%v", tmdb, imdb, ok)
	}
}

func TestParseDir_PrefersNFOID(t *testing.T) {
	dir := t.TempDir()
	nfo := `<movie><tmdbid>603</tmdbid></movie>`
	if err := os.WriteFile(filepath.Join(dir, "movie.nfo"), []byte(nfo), 0644); err != nil {
		t.Fatal(err)
	}
	info := ParseDir(dir, "The Matrix (1999)", nil)
	if info.TMDBID != 603 {
		t.Errorf("TMDBID: got %d, want 603", info.TMDBID)
	}
	if info.Title != "The Matrix" || info.Year != 1999 {
		t.Errorf("title/year: got %q/%d", info.Title, info.Year)
	}
}

func TestParseDir_FallbackToFilename(t *testing.T) {
	dir := t.TempDir()
	// directory is unhelpful; the big file has the real name
	bigPath := filepath.Join(dir, "The.Matrix.1999.1080p.BluRay.x264.mkv")
	if err := os.WriteFile(bigPath, make([]byte, 1024), 0644); err != nil {
		t.Fatal(err)
	}
	info := ParseDir(dir, "Movies", []string{bigPath})
	if info.Title != "The Matrix" || info.Year != 1999 {
		t.Errorf("got title=%q year=%d, want The Matrix/1999", info.Title, info.Year)
	}
}

func TestRetryTitles(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		// trailing " 1" stripped
		{"Toy Story 1", []string{"Toy Story"}},
		{"Mission Impossible 1", []string{"Mission Impossible"}},
		{"Show 1 1", []string{"Show"}}, // iterative

		// "remake" word stripped (case-insensitive)
		{"Pinocchio Remake", []string{"Pinocchio"}},
		{"Pinocchio REMAKE", []string{"Pinocchio"}},
		{"Suspiria (Remake)", []string{"Suspiria"}},
		{"The Lion King Remake (2019)", []string{"The Lion King (2019)"}},

		// both transforms — stripOnes only fires on a trailing 1, so the
		// stripRemake step is what enables the eventual " 1" strip in the
		// combined variant. Two distinct results.
		{"Toy Story 1 Remake", []string{"Toy Story 1", "Toy Story"}},

		// "1" not preceded by whitespace is preserved
		{"1917", nil},
		// "13" is not " 1"
		{"Apollo 13", nil},
		// clean title: no variants
		{"Pinocchio", nil},
		{"", nil},

		// Leading collection-number prefix stripped — common scraper artifact
		// for franchise entries ("007 Dr No", "100 The Man Who...").
		{"007 Dr No", []string{"Dr No"}},
		// Leading-digit strip combined with trailing-one strip
		{"007 Toy Story 1", []string{"007 Toy Story", "Toy Story 1", "Toy Story"}},
		// Leading number that's part of the title (no trailing whitespace
		// before the digit) — handled correctly by the regex anchor: leading
		// digits ARE there. Strip yields "Jump Street"; TMDB's primary search
		// for the full title already wins, so this retry just sits unused.
		{"21 Jump Street", []string{"Jump Street"}},
	}
	for _, tc := range cases {
		got := RetryTitles(tc.in)
		if !equalSlices(got, tc.want) {
			t.Errorf("RetryTitles(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
