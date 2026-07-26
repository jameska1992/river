package processor

import "testing"

func TestParseDirName(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantYear  int
	}{
		{"Breaking Bad (2008)", "Breaking Bad", 2008},
		{"The Matrix (1999) ", "The Matrix", 1999}, // trailing space tolerated
		{"Firefly", "Firefly", 0},                  // no year
		{"Doctor Who (2005)", "Doctor Who", 2005},
		{"No Parens 2020", "No Parens 2020", 0}, // year must be in parentheses
	}
	for _, c := range cases {
		title, year := parseDirName(c.in)
		if title != c.wantTitle || year != c.wantYear {
			t.Errorf("parseDirName(%q) = (%q, %d), want (%q, %d)", c.in, title, year, c.wantTitle, c.wantYear)
		}
	}
}

func TestParseSeasonNumber(t *testing.T) {
	cases := map[string]int{
		"Season 3":  3,
		"Season 12": 12,
		"S05":       5,
		"s1":        1,
		"Specials":  1, // no match → default season 1
		"":          1,
	}
	for in, want := range cases {
		if got := parseSeasonNumber(in); got != want {
			t.Errorf("parseSeasonNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestParseEpisodeNumber(t *testing.T) {
	cases := map[string]int{
		"S01E03.mkv":            3,  // SxxExx
		"Show.S02E10.mp4":       10, // SxxExx mid-name
		"1x04.mkv":              4,  // NxNN
		"Episode 05.mkv":        5,  // Episode NN
		"07 - The Reveal.mkv":   7,  // leading number
		"no number here.mkv":    0,  // nothing parseable
	}
	for in, want := range cases {
		if got := parseEpisodeNumber(in); got != want {
			t.Errorf("parseEpisodeNumber(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSidecarLang(t *testing.T) {
	cases := []struct {
		base, stem, want string
	}{
		{"Movie", "Movie", "und"},        // no language segment
		{"Movie.en", "Movie", "en"},      // simple tag
		{"Movie.en.sdh", "Movie", "en"},  // only the first segment
		{"Movie.FR", "Movie", "fr"},      // lowercased
		{"Movie.", "Movie", "und"},       // empty segment
	}
	for _, c := range cases {
		if got := sidecarLang(c.base, c.stem); got != c.want {
			t.Errorf("sidecarLang(%q, %q) = %q, want %q", c.base, c.stem, got, c.want)
		}
	}
}

func TestLanguageLabel(t *testing.T) {
	cases := map[string]string{
		"en":  "English",
		"EN":  "English", // case-insensitive
		"fr":  "French",
		"und": "Unknown",
		"xx":  "xx", // unknown code passes through unchanged
	}
	for in, want := range cases {
		if got := languageLabel(in); got != want {
			t.Errorf("languageLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
