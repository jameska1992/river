package processor

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"river-audio-trans/internal/apiclient"
	"river-audio-trans/internal/consumer"
)

// A per-chapter failure (here an unprobeable/missing source file) must surface
// as a non-nil error from processAudiobook so the event is retried and
// eventually dead-lettered rather than silently dropped (#137). Before the fix
// the goroutine logged the error and returned, and Handle returned nil.
func TestProcessAudiobook_ChapterFailurePropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("[]")) // ListChapters: none registered yet
	}))
	defer srv.Close()

	p := New(apiclient.New(srv.URL, "u", "p", "river-audio-trans"), t.TempDir(), 2)
	err := p.processAudiobook(consumer.MediaDiscoveredEvent{
		LibraryType:   "audiobook",
		LibraryID:     "lib-1",
		MediaID:       "book-1",
		DirectoryName: "Some Book (2020)",
		Files:         []string{"/nonexistent/Chapter 01.mp3"},
	})
	if err == nil {
		t.Fatal("expected a non-nil error when a chapter fails to process, got nil")
	}
}

// A per-track failure must likewise surface from processMusic.
func TestProcessMusic_TrackFailurePropagates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Albums list (find/create) and album-tracks list both return empty;
		// the album is then created via POST, which we echo back with an id.
		if r.Method == http.MethodPost {
			_, _ = w.Write([]byte(`{"id":"album-1","title":"Album"}`))
			return
		}
		_, _ = w.Write([]byte("[]"))
	}))
	defer srv.Close()

	p := New(apiclient.New(srv.URL, "u", "p", "river-audio-trans"), t.TempDir(), 2)
	err := p.processMusic(consumer.MediaDiscoveredEvent{
		LibraryType:   "music",
		LibraryID:     "lib-1",
		MediaID:       "artist-1",
		DirectoryName: "Artist",
		DirectoryPath: "/nonexistent",
		Files:         []string{"/nonexistent/01 - Song.flac"},
	})
	if err == nil {
		t.Fatal("expected a non-nil error when a track fails to process, got nil")
	}
}
