package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestSkipDir(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"Movies", false},
		{"Action", false},
		{"4K Movies", false},
		{".DS_Store", true},
		{".cache", true},
		{"@eaDir", true},
		{"@EADir", true}, // case-insensitive
		{"lost+found", true},
		{"Extras", true},
		{"Featurettes", true},
		{"Behind The Scenes", true},
		{"Trailers", true},
		// Subtitle sidecar directories — walker should refuse to
		// descend, since files inside are companions to a video sibling,
		// not primary media.
		{"subtitles", true},
		{"Subtitles", true},
		{"SUBTITLES", true},
		{"subs", true},
		{"sub", true},
		{"", false},
	}
	for _, tc := range cases {
		if got := skipDir(tc.name); got != tc.want {
			t.Errorf("skipDir(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestIsAudioVariantFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		// Positive: the transcoder's convention. Any digit count works.
		{"Hellboy II- The Golden Army.audio_0.mp4", true},
		{"Hellboy II- The Golden Army.audio_1.mp4", true},
		{"Movie Title.audio_12.mkv", true},
		{"/media/movies/Foo (2020)/Foo.audio_3.mp4", true},
		// Negative: main feature, not a variant.
		{"Hellboy II- The Golden Army.mp4", false},
		{"Inception.mkv", false},
		// Negative: same suffix pattern but not on a video file.
		{"Hellboy.audio_0.mka", false},
		{"Hellboy.audio_0.vtt", false},
		// Negative: partial or malformed matches must not trip.
		// A bare "audio_0.mp4" (no stem prefix, so no leading dot before
		// "audio_") is not a variant — the transcoder never writes that
		// form, and matching it would over-flag legitimate movies whose
		// title happens to start "audio_".
		{"audio_0.mp4", false},
		{"Movie.audio_.mp4", false},
		{"Movie.audio.mp4", false},
		{"Movie.audio_abc.mp4", false},
		// Space-form matches (legacy duplication cleanup): filenames
		// like "Air Force One audio 0.mp4" that the OLD scanner
		// mistakenly promoted to their own movies, then the transcoder
		// copied to a canonical output dir. On future scans we want to
		// keep these out of the movie set even after re-ingest.
		{"Air Force One audio 0.mp4", true},
		{"Air Force One audio 12.mp4", true},
		{"Something Something audio 3.mkv", true},
		// But something that just contains "audio" mid-title shouldn't
		// trip — only trailing " audio <digits>".
		{"Studio Audio Book.mp4", false},
		{"The Audio Movie.mp4", false},
	}
	for _, tc := range cases {
		if got := isAudioVariantFile(tc.path); got != tc.want {
			t.Errorf("isAudioVariantFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// Mirrors the exact layout of /mnt/tank/media/movies/Hellboy II- The
// Golden Army (2008) so a regression to the "one movie per file"
// duplicate bug would fail here immediately.
func TestWalkMovieFiles_HellboyLayout(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"Hellboy II- The Golden Army (2008)/Hellboy II- The Golden Army.mp4":         "x",
		"Hellboy II- The Golden Army (2008)/Hellboy II- The Golden Army.audio_0.mp4": "x",
		"Hellboy II- The Golden Army (2008)/Hellboy II- The Golden Army.audio_1.mp4": "x",
		"Hellboy II- The Golden Army (2008)/subtitles/Hellboy II- The Golden Army.eng.vtt": "x",
		"Hellboy II- The Golden Army (2008)/subtitles/Hellboy II- The Golden Army.ita.vtt": "x",
	})
	got := collectWalk(t, root, 0)
	want := []string{"Hellboy II- The Golden Army (2008)/Hellboy II- The Golden Army.mp4"}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestIsExtraFile(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{"Inception.mkv", false},
		{"The.Matrix.1999.mkv", false},
		{"Inception-trailer.mkv", true},
		{"Inception-BehindTheScenes.mp4", true}, // case-insensitive
		{"Inception-deleted.mkv", true},
		{"Inception-featurette.mkv", true},
		{"Inception-interview.mkv", true},
		{"Inception-scene.mkv", true},
		{"Sample.mkv", true},
		{"sample.mkv", true},
		{"/path/to/Inception-trailer.mkv", true},
	}
	for _, tc := range cases {
		if got := isExtraFile(tc.path); got != tc.want {
			t.Errorf("isExtraFile(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

// fakeRoot builds a directory tree from a map of relative paths -> file
// bytes (empty bytes for empty files). It returns the absolute root.
func fakeRoot(t *testing.T, layout map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range layout {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func collectWalk(t *testing.T, root string, maxDepth int) []string {
	t.Helper()
	var got []string
	err := walkMovieFiles(root, maxDepth, func(p string) error {
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		got = append(got, rel)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	return got
}

func TestWalkMovieFiles_FlatLibrary(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"The Matrix (1999).mkv": "x",
		"Inception.2010.mkv":    "x",
		"readme.txt":            "x", // ignored: not a video
	})
	got := collectWalk(t, root, 0)
	want := []string{"Inception.2010.mkv", "The Matrix (1999).mkv"}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalkMovieFiles_OneFolderPerMovie(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"The Matrix (1999)/The Matrix.mkv":  "x",
		"Inception (2010)/Inception.1080p.mkv": "x",
	})
	got := collectWalk(t, root, 0)
	want := []string{
		"Inception (2010)/Inception.1080p.mkv",
		"The Matrix (1999)/The Matrix.mkv",
	}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalkMovieFiles_NestedCategoryFolders(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"Action/The Matrix (1999)/The Matrix.mkv":  "x",
		"Action/Mad Max (2015).mkv":                "x", // loose file inside category
		"Sci-Fi/Inception (2010)/Inception.mkv":    "x",
	})
	got := collectWalk(t, root, 0)
	want := []string{
		"Action/Mad Max (2015).mkv",
		"Action/The Matrix (1999)/The Matrix.mkv",
		"Sci-Fi/Inception (2010)/Inception.mkv",
	}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalkMovieFiles_SkipsExtrasDirAndExtrasFiles(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"Inception (2010)/Inception.mkv":              "x",
		"Inception (2010)/Inception-trailer.mkv":      "x", // extras file by suffix
		"Inception (2010)/Featurettes/making-of.mkv":  "x", // extras subdir
		"Inception (2010)/Sample.mkv":                 "x", // explicit sample
		"@eaDir/thumb.mkv":                            "x", // NAS system dir
		".cache/preview.mkv":                          "x", // hidden dir
	})
	got := collectWalk(t, root, 0)
	want := []string{"Inception (2010)/Inception.mkv"}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestWalkMovieFiles_RespectsDepthCap(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"a/b/c/d/deep.mkv": "x",
	})
	// maxDepth=2 means we descend at most 2 levels below root: root/a/b.
	// deep.mkv lives at root/a/b/c/d, so it's not reached.
	got := collectWalk(t, root, 2)
	if len(got) != 0 {
		t.Errorf("expected nothing reached, got %v", got)
	}
	// With maxDepth=5 it should be reachable.
	got = collectWalk(t, root, 5)
	if len(got) != 1 || got[0] != filepath.Join("a", "b", "c", "d", "deep.mkv") {
		t.Errorf("unexpected result with depth=5: %v", got)
	}
}

func TestWalkMovieFiles_VariedExtensions(t *testing.T) {
	root := fakeRoot(t, map[string]string{
		"a.mkv":  "x",
		"b.mp4":  "x",
		"c.avi":  "x",
		"d.webm": "x",
		"e.txt":  "x", // dropped
		"f.srt":  "x", // dropped (subtitles)
	})
	got := collectWalk(t, root, 0)
	want := []string{"a.mkv", "b.mp4", "c.avi", "d.webm"}
	if !equalStrings(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
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
