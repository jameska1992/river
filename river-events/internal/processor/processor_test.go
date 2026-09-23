package processor

import (
	"context"
	"errors"
	"testing"

	"river-events/internal/apiclient"
	"river-events/internal/events"
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
	p := New(api, pub)

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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, &fakePub{})
	if err := p.Handle(transcodedMovie("m1")); err == nil {
		t.Fatal("expected error to propagate for retry")
	}
}

func TestHandle_IgnoresReadyEvents(t *testing.T) {
	api := &fakeAPI{movie: &apiclient.Movie{ID: "m1", FilePath: "/x", TMDBID: 1}}
	pub := &fakePub{}
	p := New(api, pub)
	// A ready event isn't re-joined (we don't bind it, but be defensive).
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindReady, Type: "movie", MediaID: "m1"})
	if len(pub.published) != 0 || api.calls != 0 {
		t.Fatalf("ready event should be a no-op; published=%d calls=%d", len(pub.published), api.calls)
	}
}

// --- tvshow ---

func TestJoinTVShow_TranscodedEpisodeReadyWhenShowEnriched(t *testing.T) {
	api := &fakeAPI{show: &apiclient.TVShow{ID: "s1", Title: "The Wire", LibraryID: "lib", TMDBID: 1438}}
	pub := &fakePub{}
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
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
	p := New(api, pub)
	// Two chapters transcode; the book should emit ready only once.
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "audiobook", MediaID: "ch1", ParentID: "b1"})
	_ = p.Handle(events.LifecycleEvent{Kind: events.KindTranscoded, Type: "audiobook", MediaID: "ch2", ParentID: "b1"})
	if len(pub.published) != 1 {
		t.Fatalf("expected exactly 1 book-level ready, got %d", len(pub.published))
	}
}
