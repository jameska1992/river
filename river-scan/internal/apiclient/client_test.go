package apiclient

import "testing"

func TestParsedPaths_ObjectForm(t *testing.T) {
	lib := Library{Paths: `[{"path":"/media/movies","pre_transcoded":false},{"path":"/media/remux","pre_transcoded":true}]`}
	paths, err := lib.ParsedPaths()
	if err != nil {
		t.Fatalf("ParsedPaths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].Path != "/media/movies" || paths[0].PreTranscoded {
		t.Errorf("path 0 = %+v, want {/media/movies false}", paths[0])
	}
	if paths[1].Path != "/media/remux" || !paths[1].PreTranscoded {
		t.Errorf("path 1 = %+v, want {/media/remux true}", paths[1])
	}
}

// A library written before per-path flags existed stores bare strings. Those
// must still parse, with PreTranscoded defaulting to false so the scanner
// falls back to the library-wide flag.
func TestParsedPaths_LegacyStringForm(t *testing.T) {
	lib := Library{Paths: `["/media/movies","/media/tv"]`}
	paths, err := lib.ParsedPaths()
	if err != nil {
		t.Fatalf("ParsedPaths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	for i, want := range []string{"/media/movies", "/media/tv"} {
		if paths[i].Path != want || paths[i].PreTranscoded {
			t.Errorf("path %d = %+v, want {%s false}", i, paths[i], want)
		}
	}
}

// A hand-edited or partially-migrated library may mix both forms.
func TestParsedPaths_MixedForm(t *testing.T) {
	lib := Library{Paths: `["/media/movies",{"path":"/media/remux","pre_transcoded":true}]`}
	paths, err := lib.ParsedPaths()
	if err != nil {
		t.Fatalf("ParsedPaths: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("expected 2 paths, got %d", len(paths))
	}
	if paths[0].Path != "/media/movies" || paths[0].PreTranscoded {
		t.Errorf("path 0 = %+v, want {/media/movies false}", paths[0])
	}
	if paths[1].Path != "/media/remux" || !paths[1].PreTranscoded {
		t.Errorf("path 1 = %+v, want {/media/remux true}", paths[1])
	}
}
