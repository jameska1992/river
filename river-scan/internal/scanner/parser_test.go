package scanner

import "testing"

func TestParseDirName(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantYear  int
	}{
		// Canonical "Title (YYYY)" already worked
		{"Doctor Who (2005)", "Doctor Who", 2005},
		// Dots / underscores / hyphens get normalized to spaces
		{"Doctor.Who.(2005)", "Doctor Who", 2005},
		{"Doctor_Who_(2005)", "Doctor Who", 2005},
		{"Doctor-Who-(2005)", "Doctor Who", 2005},
		{"Doctor.Who", "Doctor Who", 0},
		{"Doctor_Who", "Doctor Who", 0},
		// Mixed separators collapse to a single space each
		{"The.Matrix._.1999", "The Matrix 1999", 0}, // no parens, no year
		{"The_Lord_of_the_Rings_(2001)", "The Lord of the Rings", 2001},
		// No separators
		{"Plain Title (2020)", "Plain Title", 2020},
		{"Plain Title", "Plain Title", 0},
	}
	for _, tc := range cases {
		gotTitle, gotYear := parseDirName(tc.in)
		if gotTitle != tc.wantTitle || gotYear != tc.wantYear {
			t.Errorf("parseDirName(%q) = (%q, %d), want (%q, %d)",
				tc.in, gotTitle, gotYear, tc.wantTitle, tc.wantYear)
		}
	}
}

func TestParseSeasonNumber(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		// Canonical
		{"Season 1", 1},
		{"Season 02", 2},
		{"S01", 1},
		{"s12", 12},
		// Dot / underscore / hyphen separators normalize to space
		{"Season.1", 1},
		{"Season.2", 2},
		{"Season_3", 3},
		{"Season-4", 4},
		{"S_03", 3},
		{"S.05", 5},
		{"S-10", 10},
		// Default fallback when no number is present
		{"Specials", 1},
		{"", 1},
	}
	for _, tc := range cases {
		if got := parseSeasonNumber(tc.in); got != tc.want {
			t.Errorf("parseSeasonNumber(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestStripSeasonSuffix(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantNum   int
		wantOK    bool
	}{
		// Trailing "Season N"
		{"Yellowstone Season 1", "Yellowstone", 1, true},
		{"Breaking Bad Season 03", "Breaking Bad", 3, true},
		// Trailing SN / S0N
		{"Foo S01", "Foo", 1, true},
		{"Foo Bar S12", "Foo Bar", 12, true},
		// Separator-normalized inputs
		{"Foo.Bar.Season.2", "Foo Bar", 2, true},
		{"Foo_Bar_S03", "Foo Bar", 3, true},
		// No trailing season indicator
		{"Yellowstone", "Yellowstone", 0, false},
		{"Doctor Who (2005)", "Doctor Who (2005)", 0, false},
		// Bare season name without a title prefix shouldn't match — there's
		// nothing to strip down to.
		{"Season 1", "Season 1", 0, false},
		{"S01", "S01", 0, false},
		// Empty
		{"", "", 0, false},
	}
	for _, tc := range cases {
		gotTitle, gotNum, gotOK := stripSeasonSuffix(tc.in)
		if gotTitle != tc.wantTitle || gotNum != tc.wantNum || gotOK != tc.wantOK {
			t.Errorf("stripSeasonSuffix(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.in, gotTitle, gotNum, gotOK, tc.wantTitle, tc.wantNum, tc.wantOK)
		}
	}
}

func TestDetectShowSeasonSuffix(t *testing.T) {
	cases := []struct {
		in        string
		wantTitle string
		wantNum   int
		wantOK    bool
	}{
		// Release-tagged folder: nameinfo.Parse strips "1080p BluRay",
		// leaving "Breaking Bad S01" with the season tag now at the end.
		{"Breaking.Bad.S01.1080p.BluRay", "Breaking Bad", 1, true},
		// Plain trailing season
		{"Yellowstone Season 2", "Yellowstone", 2, true},
		// No season
		{"Yellowstone", "Yellowstone", 0, false},
	}
	for _, tc := range cases {
		gotTitle, gotNum, gotOK := detectShowSeasonSuffix(tc.in)
		if gotTitle != tc.wantTitle || gotNum != tc.wantNum || gotOK != tc.wantOK {
			t.Errorf("detectShowSeasonSuffix(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.in, gotTitle, gotNum, gotOK, tc.wantTitle, tc.wantNum, tc.wantOK)
		}
	}
}

func TestBestSeasonNumber(t *testing.T) {
	cases := []struct {
		folder string
		files  []string
		want   int
	}{
		// Folder name wins when it has a marker
		{"Season 3", []string{"S01E01.mkv"}, 3},
		{"S02", []string{}, 2},
		// Fall back to SxxExx from filenames when folder is uninformative
		{"", []string{"Show.S04E01.mkv", "Show.S04E02.mkv"}, 4},
		// "Specials" / "Special" map to season 0
		{"Specials", []string{"Show.S00E01.mkv"}, 0},
		{"Special", nil, 0},
		{"specials", nil, 0},
		// Explicit "Season 0" / "S00" also map to 0
		{"Season 0", nil, 0},
		{"S00", nil, 0},
		// Folder uninformative but filenames carry S00 — accept it as season 0
		// rather than skipping to the default 1.
		{"", []string{"Show.S00E01.mkv"}, 0},
		// Default of 1 when nothing helps
		{"Random", []string{"random.mkv"}, 1},
		// Empty everything
		{"", nil, 1},
	}
	for _, tc := range cases {
		if got := bestSeasonNumber(tc.folder, tc.files); got != tc.want {
			t.Errorf("bestSeasonNumber(%q, %v) = %d, want %d",
				tc.folder, tc.files, got, tc.want)
		}
	}
}

func TestSeasonNumberFromName(t *testing.T) {
	cases := []struct {
		in     string
		wantN  int
		wantOK bool
	}{
		{"Season 1", 1, true},
		{"Season 02", 2, true},
		{"S01", 1, true},
		{"s12", 12, true},
		{"Season 0", 0, true},
		{"S00", 0, true},
		{"Specials", 0, true},
		{"Special", 0, true},
		{"SPECIALS", 0, true},
		{"  specials  ", 0, true},
		{"", 0, false},
		{"Random", 0, false},
		{"Behind The Scenes", 0, false},
	}
	for _, tc := range cases {
		gotN, gotOK := seasonNumberFromName(tc.in)
		if gotN != tc.wantN || gotOK != tc.wantOK {
			t.Errorf("seasonNumberFromName(%q) = (%d, %v), want (%d, %v)",
				tc.in, gotN, gotOK, tc.wantN, tc.wantOK)
		}
	}
}

func TestNormalizeTitleKey(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		// Hyphen / dot / underscore all collapse — stops "Brooklyn-Nine-Nine"
		// (TMDB) drifting from "Brooklyn Nine Nine" (nameinfo-parsed).
		{"Brooklyn Nine-Nine", "brooklyn nine nine"},
		{"Brooklyn Nine Nine", "brooklyn nine nine"},
		{"Brooklyn.Nine.Nine", "brooklyn nine nine"},
		{"Brooklyn_Nine_Nine", "brooklyn nine nine"},
		// Colons + ampersands fold to whitespace — Law & Order: SVU was
		// duplicating because TMDB writes the colon and the scanner doesn't.
		{"Law & Order: Special Victims Unit", "law order special victims unit"},
		{"Law & Order - Special Victims Unit", "law order special victims unit"},
		{"Law & Order Special Victims Unit", "law order special victims unit"},
		// Apostrophes drop entirely so the word stays joined.
		{"Don't Look Up", "dont look up"},
		{"Dont Look Up", "dont look up"},
		{"Grey's Anatomy", "greys anatomy"},
		{"Grey’s Anatomy", "greys anatomy"}, // curly apostrophe
		// Other punctuation soup
		{"M*A*S*H", "m a s h"},
		{"WALL-E", "wall e"},
		// Whitespace + case
		{"  Brooklyn   Nine-Nine  ", "brooklyn nine nine"},
		{"BROOKLYN NINE-NINE", "brooklyn nine nine"},
		// Non-ASCII letters preserved (treats them as letters, lowercased).
		{"Pokémon", "pokémon"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeTitleKey(tc.in); got != tc.want {
			t.Errorf("normalizeTitleKey(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeSeparators(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"Doctor.Who", "Doctor Who"},
		{"Doctor_Who", "Doctor Who"},
		{"Doctor-Who", "Doctor Who"},
		{"a.b_c-d", "a b c d"},
		{"already clean", "already clean"},
		{"multiple..separators__together", "multiple separators together"},
		{"  edges  trimmed  ", "edges trimmed"},
		// Parentheses preserved so yearSuffix still matches
		{"Show (2010)", "Show (2010)"},
		{"Show.(2010)", "Show (2010)"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := normalizeSeparators(tc.in); got != tc.want {
			t.Errorf("normalizeSeparators(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
