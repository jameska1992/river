package processor

import (
	"context"
	"errors"
	"testing"

	"river-events/internal/apiclient"
	"river-events/internal/events"
	"river-events/internal/webhooks"
)

type fakeAPI struct {
	movie     *apiclient.Movie
	show      *apiclient.TVShow
	seasons   []apiclient.Season
	episodes  map[string][]apiclient.Episode // seasonID -> episodes
	album     *apiclient.Album
	tracks    []apiclient.Track
	audiobook *apiclient.Audiobook
	chapters  []apiclient.Chapter
	err       error
	calls     int
}

func (f *fakeAPI) GetMovie(id string) (*apiclient.Movie, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.movie, nil
}
func (f *fakeAPI) GetTVShow(id string) (*apiclient.TVShow, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.show, nil
}
func (f *fakeAPI) ListSeasons(showID string) ([]apiclient.Season, error) {
	return f.seasons, f.err
}
func (f *fakeAPI) ListEpisodes(showID, seasonID string) ([]apiclient.Episode, error) {
	return f.episodes[seasonID], f.err
}
func (f *fakeAPI) GetAlbum(id string) (*apiclient.Album, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.album, nil
}
func (f *fakeAPI) ListAlbumTracks(albumID string) ([]apiclient.Track, error) {
	return f.tracks, f.err
}
func (f *fakeAPI) GetAudiobook(id string) (*apiclient.Audiobook, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return f.audiobook, nil
}
func (f *fakeAPI) ListChapters(audiobookID string) ([]apiclient.Chapter, error) {
	return f.chapters, f.err
}

type fakePub struct {
	published []events.LifecycleEvent
	err       error
}

func (f *fakePub) Publish(_ context.Context, e events.LifecycleEvent) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, e)
	return nil
}

func transcodedMovie(id string) events.LifecycleEvent {
	return events.LifecycleEvent{Kind: events.KindTranscoded, Type: "movie", MediaID: id}
}

func TestJoinMovie_EmitsReadyWhenBothComplete(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", Title: "Inception", LibraryID: "lib1", FilePath: "/out/m1.mp4", TMDBID: 27205}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)

	if err := p.Handle(transcodedMovie("m1")); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 ready event, got %d", len(pub.published))
	}
	got := pub.published[0]
	if got.Kind != events.KindReady || got.Type != "movie" || got.MediaID != "m1" {
		t.Fatalf("unexpected ready event: %+v", got)
	}
	if got.Title != "Inception" || got.LibraryID != "lib1" {
		t.Fatalf("ready event missing enriched context: %+v", got)
	}
}

func TestJoinMovie_NoReadyWhenNotTranscoded(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", FilePath: "", TMDBID: 27205}} // enriched, not transcoded
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	if err := p.Handle((events.LifecycleEvent{Kind: events.KindEnriched, Type: "movie", MediaID: "m1"})); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 0 {
		t.Fatalf("expected no ready event, got %d", len(pub.published))
	}
}

func TestJoinMovie_NoReadyWhenNotEnriched(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", FilePath: "/out/m1.mp4", TMDBID: 0}} // transcoded, not enriched
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	if err := p.Handle(transcodedMovie("m1")); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 0 {
		t.Fatalf("expected no ready event, got %d", len(pub.published))
	}
}

func TestJoinMovie_DedupesReady(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", FilePath: "/out/m1.mp4", TMDBID: 1}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	// Both the transcoded and enriched events find the record complete; ready
	// must be emitted only once.
	_ = p.Handle(transcodedMovie("m1"))
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindEnriched, Type: "movie", MediaID: "m1"})
	if len(pub.published) != 1 {
		t.Fatalf("expected exactly 1 ready event (deduped), got %d", len(pub.published))
	}
}

func TestJoinMovie_APIErrorPropagates(t *testing.T) {
	// A transient API error must propagate so the message is retried, not acked.
	api := &fakeAPI{err: errors.New("api down")}
	p := New(api, &fakePub{}, nil, nil)
	if err := p.Handle(transcodedMovie("m1")); err == nil {
		t.Fatal("expected error to propagate for retry")
	}
}

func TestHandle_ReadyEventNotRejoinedButFannedOut(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", FilePath: "/x", TMDBID: 1}}
	pub := &fakePub{}
	// No fan-out configured: a ready event isn't re-joined and produces nothing.
	p := New(api, pub, nil, nil)
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"})
	if len(pub.published) != 0 || api.calls != 0 {
		t.Fatalf("ready event should not be re-joined; published=%d calls=%d", len(pub.published), api.calls)
	}
}

type fakeHooks struct {
	hooks []apiclient.ActiveWebhook
	err   error
}

func (f *fakeHooks) ActiveWebhooks() ([]apiclient.ActiveWebhook, error) {
	return f.hooks, f.err
}

type fakeDeliveries struct {
	published []webhooks.Delivery
	err       error
}

func (f *fakeDeliveries) Publish(d webhooks.Delivery) error {
	if f.err != nil {
		return f.err
	}
	f.published = append(f.published, d)
	return nil
}

func TestFanOut_DeliversToMatchingWebhooks(t *testing.T) {
	hooks := &fakeHooks{hooks: []apiclient.ActiveWebhook{
		{ID: "w-all", URL: "https://a", Secret: "s1", Events: nil},                               // all kinds
		{ID: "w-ready", URL: "https://b", Secret: "s2", Events: []string{events.KindReady}},      // ready only
		{ID: "w-trans", URL: "https://c", Secret: "s3", Events: []string{events.KindTranscoded}}, // transcoded only
	}}
	del := &fakeDeliveries{}
	// A movie that isn't ready yet, so the join emits nothing — we isolate fan-out.
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", FilePath: "", TMDBID: 0}}
	p := New(api, &fakePub{}, hooks, del)

	if err := p.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"}); err != nil {
		t.Fatalf("Handle: %v", err)
	}
	// The all-kinds and ready-only hooks match; the transcoded-only hook doesn't.
	if len(del.published) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(del.published))
	}
	got := map[string]bool{}
	for _, d := range del.published {
		got[d.WebhookID] = true
		if d.Event.MediaID != "m1" {
			t.Fatalf("delivery carries wrong event: %+v", d.Event)
		}
	}
	if !got["w-all"] || !got["w-ready"] || got["w-trans"] {
		t.Fatalf("wrong webhooks matched: %v", got)
	}
}

func TestFanOut_CarriesSecretAndURL(t *testing.T) {
	hooks := &fakeHooks{hooks: []apiclient.ActiveWebhook{{ID: "w1", URL: "https://hook", Secret: "shh"}}}
	del := &fakeDeliveries{}
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1"}}
	p := New(api, &fakePub{}, hooks, del)
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"}); err != nil {
		t.Fatal(err)
	}
	if len(del.published) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(del.published))
	}
	d := del.published[0]
	if d.URL != "https://hook" || d.Secret != "shh" || d.WebhookID != "w1" {
		t.Fatalf("delivery missing routing/signing context: %+v", d)
	}
}

func TestFanOut_ErrorPropagatesForRetry(t *testing.T) {
	hooks := &fakeHooks{err: errors.New("api down")}
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1"}}
	p := New(api, &fakePub{}, hooks, &fakeDeliveries{})
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"}); err == nil {
		t.Fatal("expected fan-out error to propagate for retry")
	}
}

func TestFanOut_JoinEmitsReadyThenSeparateEventFansOut(t *testing.T) {
	// A transcoded movie that IS ready: the join emits ready (published to the
	// exchange), and fan-out runs for the transcoded event itself.
	hooks := &fakeHooks{hooks: []apiclient.ActiveWebhook{{ID: "w1", URL: "https://h", Secret: "s"}}}
	del := &fakeDeliveries{}
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", Title: "T", FilePath: "/out/m1.mp4", TMDBID: 1}}
	pub := &fakePub{}
	p := New(api, pub, hooks, del)
	if err := p.Handle(transcodedMovie("m1")); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected join to emit 1 ready event, got %d", len(pub.published))
	}
	// Fan-out for the transcoded event goes to the all-kinds webhook. The ready
	// event fans out on its own round-trip (not exercised here).
	if len(del.published) != 1 || del.published[0].Event.Kind != events.KindTranscoded {
		t.Fatalf("expected 1 transcoded delivery, got %+v", del.published)
	}
}

// --- tvshow ---

func TestJoinTVShow_TranscodedEpisodeReadyWhenShowEnriched(t *testing.T) {
	api := &fakeAPI{show: &apiclient.TVShow{ID: "s1", Title: "The Wire", LibraryID: "lib", TMDBID: 1438}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	err := p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "tvshow", MediaID: "ep1", ParentID: "s1", SeasonID: "se1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 ready, got %d", len(pub.published))
	}
	got := pub.published[0]
	if got.Type != "tvshow" || got.MediaID != "ep1" || got.ParentID != "s1" || got.SeasonID != "se1" {
		t.Fatalf("unexpected ready: %+v", got)
	}
}

func TestJoinTVShow_TranscodedNoReadyWhenShowNotEnriched(t *testing.T) {
	api := &fakeAPI{show: &apiclient.TVShow{ID: "s1", TMDBID: 0}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "tvshow", MediaID: "ep1", ParentID: "s1"})
	if len(pub.published) != 0 {
		t.Fatalf("expected no ready, got %d", len(pub.published))
	}
}

func TestJoinTVShow_EnrichedFansOutOverTranscodedEpisodes(t *testing.T) {
	api := &fakeAPI{
		show:    &apiclient.TVShow{ID: "s1", TMDBID: 1438, LibraryID: "lib"},
		seasons: []apiclient.Season{{ID: "se1"}, {ID: "se2"}},
		episodes: map[string][]apiclient.Episode{
			"se1": {{ID: "e1", SeasonID: "se1", FilePath: "/e1.mp4"}, {ID: "e2", SeasonID: "se1", FilePath: ""}},
			"se2": {{ID: "e3", SeasonID: "se2", FilePath: "/e3.mp4"}},
		},
	}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindEnriched, Type: "tvshow", MediaID: "s1"}); err != nil {
		t.Fatal(err)
	}
	// e1 + e3 are transcoded → 2 ready; e2 (no file) skipped.
	if len(pub.published) != 2 {
		t.Fatalf("expected 2 ready (transcoded episodes only), got %d", len(pub.published))
	}
}

// --- music ---

func TestJoinMusic_TrackReadyWhenAlbumEnriched(t *testing.T) {
	api := &fakeAPI{album: &apiclient.Album{ID: "al1", LibraryID: "lib", CoverPath: "/cover.jpg"}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "music", MediaID: "t1", ParentID: "al1"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 || pub.published[0].MediaID != "t1" || pub.published[0].ParentID != "al1" {
		t.Fatalf("unexpected: %+v", pub.published)
	}
}

func TestJoinMusic_NoReadyWhenAlbumNotEnriched(t *testing.T) {
	api := &fakeAPI{album: &apiclient.Album{ID: "al1", CoverPath: ""}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "music", MediaID: "t1", ParentID: "al1"})
	if len(pub.published) != 0 {
		t.Fatalf("expected no ready, got %d", len(pub.published))
	}
}

func TestJoinMusic_EnrichedFansOutOverTracks(t *testing.T) {
	api := &fakeAPI{
		album:  &apiclient.Album{ID: "al1", CoverPath: "/c.jpg", LibraryID: "lib"},
		tracks: []apiclient.Track{{ID: "t1", FilePath: "/t1.m4a"}, {ID: "t2", FilePath: ""}},
	}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindEnriched, Type: "music", MediaID: "al1"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 || pub.published[0].MediaID != "t1" {
		t.Fatalf("expected 1 ready (t1 only), got %+v", pub.published)
	}
}

// --- audiobook (book-level) ---

func TestJoinAudiobook_ReadyOnTranscodedWhenEnriched(t *testing.T) {
	api := &fakeAPI{audiobook: &apiclient.Audiobook{ID: "b1", LibraryID: "lib", Title: "Dune", OpenLibraryKey: "/works/OL1W"}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "audiobook", MediaID: "ch1", ParentID: "b1"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 || pub.published[0].MediaID != "b1" || pub.published[0].Type != "audiobook" {
		t.Fatalf("expected 1 book-level ready, got %+v", pub.published)
	}
}

func TestJoinAudiobook_EnrichedNeedsTranscodedChapter(t *testing.T) {
	// Enriched but no transcoded chapter yet → not ready.
	api := &fakeAPI{
		audiobook: &apiclient.Audiobook{ID: "b1", OpenLibraryKey: "/works/OL1W"},
		chapters:  []apiclient.Chapter{{ID: "ch1", FilePath: ""}},
	}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindEnriched, Type: "audiobook", MediaID: "b1"})
	if len(pub.published) != 0 {
		t.Fatalf("expected no ready without a transcoded chapter, got %d", len(pub.published))
	}
	// Now a chapter exists → ready once.
	api.chapters = []apiclient.Chapter{{ID: "ch1", FilePath: "/ch1.m4a"}}
	if err := p.Handle(events.LifecycleEvent{Kind: events.KindEnriched, Type: "audiobook", MediaID: "b1"}); err != nil {
		t.Fatal(err)
	}
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 ready, got %d", len(pub.published))
	}
}

func TestJoinAudiobook_ReadyDedupedPerBook(t *testing.T) {
	api := &fakeAPI{audiobook: &apiclient.Audiobook{ID: "b1", OpenLibraryKey: "/works/OL1W"}}
	pub := &fakePub{}
	p := New(api, pub, nil, nil)
	// Two chapters transcode; the book should emit ready only once.
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "audiobook", MediaID: "ch1", ParentID: "b1"})
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "audiobook", MediaID: "ch2", ParentID: "b1"})
	if len(pub.published) != 1 {
		t.Fatalf("expected exactly 1 book-level ready, got %d", len(pub.published))
	}
}
