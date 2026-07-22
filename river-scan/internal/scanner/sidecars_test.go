package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestAudioVariantIndex(t *testing.T) {
	cases := []struct {
		name    string
		stem    string
		wantIdx int
		wantOK  bool
	}{
		{"Hellboy II- The Golden Army.audio_0.mp4", "Hellboy II- The Golden Army", 0, true},
		{"Hellboy II- The Golden Army.audio_1.mp4", "Hellboy II- The Golden Army", 1, true},
		{"Movie.audio_12.mkv", "Movie", 12, true},
		// Wrong stem: no match.
		{"OtherMovie.audio_0.mp4", "Hellboy II- The Golden Army", 0, false},
		// Not a video extension: not a variant.
		{"Movie.audio_0.mka", "Movie", 0, false},
		// Not a numeric suffix.
		{"Movie.audio_abc.mp4", "Movie", 0, false},
		// Main file, no audio_ suffix.
		{"Movie.mp4", "Movie", 0, false},
	}
	for _, tc := range cases {
		gotIdx, gotOK := audioVariantIndex(tc.name, tc.stem)
		if gotIdx != tc.wantIdx || gotOK != tc.wantOK {
			t.Errorf("audioVariantIndex(%q, %q) = (%d, %v), want (%d, %v)",
				tc.name, tc.stem, gotIdx, gotOK, tc.wantIdx, tc.wantOK)
		}
	}
}

func TestSubtitleLangForStem(t *testing.T) {
	stem := "Hellboy II- The Golden Army"
	cases := []struct {
		name     string
		wantLang string
		wantOK   bool
	}{
		{"Hellboy II- The Golden Army.eng.vtt", "eng", true},
		{"Hellboy II- The Golden Army.ita.vtt", "ita", true},
		{"Hellboy II- The Golden Army.en.forced.vtt", "en", true},
		{"Hellboy II- The Golden Army.vtt", "und", true},
		// Case difference in stem still lines up.
		{"HELLBOY II- THE GOLDEN ARMY.eng.vtt", "eng", true},
		// Wrong stem.
		{"OtherMovie.eng.vtt", "", false},
		// Wrong extension.
		{"Hellboy II- The Golden Army.eng.srt", "", false},
	}
	for _, tc := range cases {
		gotLang, gotOK := subtitleLangForStem(tc.name, stem)
		if gotLang != tc.wantLang || gotOK != tc.wantOK {
			t.Errorf("subtitleLangForStem(%q, stem) = (%q, %v), want (%q, %v)",
				tc.name, gotLang, gotOK, tc.wantLang, tc.wantOK)
		}
	}
}

func TestFindSubtitleSubdir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "Subtitles"), 0o755); err != nil {
		t.Fatal(err)
	}
	got := findSubtitleSubdir(root)
	if got != filepath.Join(root, "Subtitles") {
		t.Errorf("findSubtitleSubdir(%q) = %q, want %q", root, got, filepath.Join(root, "Subtitles"))
	}

	// No sub dir.
	empty := t.TempDir()
	if got := findSubtitleSubdir(empty); got != "" {
		t.Errorf("expected empty for dir with no subtitles subdir, got %q", got)
	}

	// Alternate name "subs".
	subs := t.TempDir()
	if err := os.MkdirAll(filepath.Join(subs, "subs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := findSubtitleSubdir(subs); got != filepath.Join(subs, "subs") {
		t.Errorf("subs dir not detected: got %q", got)
	}
}

func TestLanguageLabel(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"en", "English"},
		{"eng", "English"},
		{"EN", "English"}, // case-insensitive lookup
		{"ita", "Italian"},
		{"und", "Unknown"},
		// Unknown code passes through as-is.
		{"xx", "xx"},
		{"", ""},
	}
	for _, tc := range cases {
		if got := languageLabel(tc.in); got != tc.want {
			t.Errorf("languageLabel(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestCollectMediaFiles_FiltersAudioVariants ensures the TV-show
// collector doesn't include audio_N variant files when they end up in
// a season directory. Without the filter, an audio-variant would be
// mistaken for a valid episode video and clobber the primary file's
// path via parseEpisodeNumber's match.
func TestCollectMediaFiles_FiltersAudioVariants(t *testing.T) {
	root := t.TempDir()
	files := []string{
		"S01E01.mp4",
		"S01E01.audio_0.mp4",
		"S01E01.audio_1.mp4",
		"S01E02.mp4",
		"subtitles/S01E01.eng.vtt", // never picked up (wrong ext); listed for completeness
	}
	for _, rel := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got, err := collectMediaFiles(root, "tvshow")
	if err != nil {
		t.Fatal(err)
	}
	// Take only the basenames for a stable, order-agnostic comparison.
	names := make([]string, len(got))
	for i, p := range got {
		names[i] = filepath.Base(p)
	}
	sort.Strings(names)
	want := []string{"S01E01.mp4", "S01E02.mp4"}
	if !equalStrings(names, want) {
		t.Errorf("got %v, want %v", names, want)
	}
}
