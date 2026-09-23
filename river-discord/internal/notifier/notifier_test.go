package notifier

import (
	"strings"
	"sync"
	"testing"
	"time"

	"river-discord/internal/apiclient"
	"river-discord/internal/embed"
	"river-discord/internal/events"
)

type fakeAPI struct {
	movie     *apiclient.Movie
	show      *apiclient.TVShow
	album     *apiclient.Album
	artist    *apiclient.Artist
	audiobook *apiclient.Audiobook
	err       error
}

func (f *fakeAPI) GetMovie(string) (*apiclient.Movie, error)   { return f.movie, f.err }
func (f *fakeAPI) GetTVShow(string) (*apiclient.TVShow, error) { return f.show, f.err }
func (f *fakeAPI) GetAlbum(string) (*apiclient.Album, error)   { return f.album, f.err }
func (f *fakeAPI) GetArtist(string) (*apiclient.Artist, error) { return f.artist, f.err }
func (f *fakeAPI) GetAudiobook(string) (*apiclient.Audiobook, error) {
	return f.audiobook, f.err
}

type capture struct {
	url string
	msg embed.Message
}

type fakeSender struct {
	mu   sync.Mutex
	sent []capture
	done chan struct{}
}

func newSender() *fakeSender { return &fakeSender{done: make(chan struct{}, 8)} }

func (s *fakeSender) Send(url string, msg embed.Message) error {
	s.mu.Lock()
	s.sent = append(s.sent, capture{url, msg})
	s.mu.Unlock()
	s.done <- struct{}{}
	return nil
}

func (s *fakeSender) last() capture {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.sent[len(s.sent)-1]
}

func (s *fakeSender) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sent)
}

func readyKinds() map[string]bool { return map[string]bool{events.KindReady: true} }

func waitSend(t *testing.T, s *fakeSender) {
	t.Helper()
	select {
	case <-s.done:
	case <-time.After(time.Second):
		t.Fatal("no message delivered")
	}
}

func TestHandle_IgnoresNonNotifyKinds(t *testing.T) {
	s := newSender()
	n := New(&fakeAPI{}, s, 10*time.Millisecond, readyKinds(), nil, "https://default")
	n.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "movie", MediaID: "m1"})

	select {
	case <-s.done:
		t.Fatal("transcoded event should not notify")
	case <-time.After(50 * time.Millisecond):
	}
}

func TestHandle_MovieReadyDelivers(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", Title: "Inception", Year: 2010}}
	s := newSender()
	n := New(api, s, 10*time.Millisecond, readyKinds(), nil, "https://default")

	n.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"})
	waitSend(t, s)

	got := s.last()
	if got.url != "https://default" {
		t.Fatalf("url = %q", got.url)
	}
	if got.msg.Embeds[0].Title != "Inception (2010)" {
		t.Fatalf("title = %q", got.msg.Embeds[0].Title)
	}
}

func TestHandle_TVEpisodesBatchIntoOneMessage(t *testing.T) {
	api := &fakeAPI{show: &apiclient.TVShow{ID: "show1", Title: "Severance", Year: 2022}}
	s := newSender()
	n := New(api, s, 30*time.Millisecond, readyKinds(), nil, "https://default")

	for _, ep := range []string{"Ep1", "Ep2", "Ep3"} {
		n.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "tvshow", ParentID: "show1", MediaID: ep, Title: ep})
	}
	waitSend(t, s)

	if s.count() != 1 {
		t.Fatalf("expected a single batched message, got %d", s.count())
	}
	content := s.last().msg.Content
	if content == "" || content == "📺 **Severance** — new episode ready" {
		t.Fatalf("expected a multi-episode digest, got %q", content)
	}
}

func TestHandle_MusicResolvesArtistAndDedupesTracks(t *testing.T) {
	api := &fakeAPI{
		album:  &apiclient.Album{ID: "al1", ArtistID: "ar1", Title: "RAM", Year: 2013},
		artist: &apiclient.Artist{ID: "ar1", Name: "Daft Punk"},
	}
	s := newSender()
	n := New(api, s, 20*time.Millisecond, readyKinds(), nil, "https://default")

	// Two tracks of the same album → one album message.
	n.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "music", ParentID: "al1", MediaID: "t1"})
	n.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "music", ParentID: "al1", MediaID: "t2"})
	waitSend(t, s)

	if s.count() != 1 {
		t.Fatalf("tracks of one album should dedupe to 1 message, got %d", s.count())
	}
	if c := s.last().msg.Content; !strings.Contains(c, "Daft Punk") {
		t.Fatalf("expected artist in content, got %q", c)
	}
}

func TestRoute_LibraryThenTypeThenDefault(t *testing.T) {
	routes := map[string]string{"lib-movies": "https://movies", "audiobook": "https://books"}
	n := New(&fakeAPI{}, newSender(), time.Millisecond, readyKinds(), routes, "https://default")

	// library id wins
	if got := n.route(events.LifecycleEvent{LibraryID: "lib-movies", Type: "movie"}); got != "https://movies" {
		t.Fatalf("library route = %q", got)
	}
	// falls back to type
	if got := n.route(events.LifecycleEvent{LibraryID: "other", Type: "audiobook"}); got != "https://books" {
		t.Fatalf("type route = %q", got)
	}
	// falls back to default
	if got := n.route(events.LifecycleEvent{LibraryID: "x", Type: "movie"}); got != "https://default" {
		t.Fatalf("default route = %q", got)
	}
}

func TestFlush_DropsOnAPIError(t *testing.T) {
	api := &fakeAPI{err: errTest}
	s := newSender()
	n := New(api, s, 10*time.Millisecond, readyKinds(), nil, "https://default")
	n.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"})

	select {
	case <-s.done:
		t.Fatal("a record-read failure should drop, not deliver")
	case <-time.After(60 * time.Millisecond):
	}
}

var errTest = &apiError{}

type apiError struct{}

func (*apiError) Error() string { return "boom" }
