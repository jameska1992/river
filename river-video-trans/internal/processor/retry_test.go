package processor

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"river-video-trans/internal/apiclient"
	"river-video-trans/internal/consumer"
)

// A per-episode failure (here an unprobeable/missing source file) must surface
// as a non-nil error from processTVShow so the event is retried and eventually
// dead-lettered rather than silently dropped (#137). Before the fix the episode
// loop logged the error and continued, returning nil — dropping the failure.
func TestProcessTVShow_EpisodeFailurePropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/episodes") {
			_, _ = w.Write([]byte("[]")) // ListEpisodes: none registered yet
			return
		}
		_, _ = w.Write([]byte(`{"id":"show-1","title":"Show"}`)) // GetTVShow
	}))
	defer srv.Close()

	p := New(apiclient.New(srv.URL, "u", "p", "river-video-trans"), t.TempDir())
	err := p.processTVShow(consumer.MediaDiscoveredEvent{
		LibraryType: "tvshow",
		LibraryID:   "lib-1",
		MediaID:     "show-1",
		SeasonID:    "season-1",
		SeasonName:  "Season 1",
		Files:       []string{"/nonexistent/Show.S01E03.mkv"},
	})
	if err == nil {
		t.Fatal("expected a non-nil error when an episode fails to process, got nil")
	}
}
