// Package processor joins media.transcoded.* and media.enriched.* into
// media.ready.* — a title is "ready" once it has both a transcoded output and
// enriched metadata.
//
// The join is stateless with respect to the event stream: on any completion
// event, the processor reads the record back from river-api (the source of
// truth for both halves) and emits ready when both are satisfied. Because
// transcode is per-unit (episode/track/chapter) while enrichment is per-parent
// (show/album/audiobook), an enrichment event fans out over the parent's
// already-transcoded units so nothing is missed when enrichment lands last.
//
// A small in-process guard suppresses the obvious duplicate (both events
// finding the record already complete); downstream consumers must be idempotent
// regardless, since a restart clears the guard.
package processor

import (
	"context"
	"log"
	"sync"
	"time"

	"river-events/internal/apiclient"
	"river-events/internal/events"
)

// mediaAPI is the slice of river-api river-events needs to compute readiness.
type mediaAPI interface {
	GetMovie(id string) (*apiclient.Movie, error)
	GetTVShow(id string) (*apiclient.TVShow, error)
	ListSeasons(showID string) ([]apiclient.Season, error)
	ListEpisodes(showID, seasonID string) ([]apiclient.Episode, error)
	GetAlbum(id string) (*apiclient.Album, error)
	ListAlbumTracks(albumID string) ([]apiclient.Track, error)
	GetAudiobook(id string) (*apiclient.Audiobook, error)
	ListChapters(audiobookID string) ([]apiclient.Chapter, error)
}

// publisher emits the joined media.ready.* event.
type publisher interface {
	Publish(ctx context.Context, e events.LifecycleEvent) error
}

type Processor struct {
	api mediaAPI
	pub publisher

	mu           sync.Mutex
	readyEmitted map[string]bool // dedupe key -> emitted
}

func New(api mediaAPI, pub publisher) *Processor {
	return &Processor{api: api, pub: pub, readyEmitted: make(map[string]bool)}
}

// Handle processes one lifecycle event. Only transcoded/enriched drive the
// join; a stray ready event (we don't bind it, but be defensive) is ignored.
func (p *Processor) Handle(e events.LifecycleEvent) error {
	switch e.Kind {
	case events.KindTranscoded, events.KindEnriched:
	default:
		return nil
	}

	switch e.Type {
	case "movie":
		return p.joinMovie(e)
	case "tvshow":
		return p.joinTVShow(e)
	case "music":
		return p.joinMusic(e)
	case "audiobook":
		return p.joinAudiobook(e)
	default:
		return nil
	}
}

// --- movie: the record carries both halves ---

func (p *Processor) joinMovie(e events.LifecycleEvent) error {
	m, err := p.api.GetMovie(e.MediaID)
	if err != nil {
		return err
	}
	if m.FilePath == "" || m.TMDBID == 0 {
		return nil
	}
	return p.emitReady("movie:"+m.ID, events.LifecycleEvent{
		Kind: events.KindReady, Type: "movie", MediaID: m.ID,
		LibraryID: m.LibraryID, Title: m.Title,
	})
}

// --- tvshow: episode transcoded + show enriched (tmdb_id) ---

func (p *Processor) joinTVShow(e events.LifecycleEvent) error {
	if e.Kind == events.KindTranscoded {
		// One episode just transcoded; ready if its show is enriched.
		show, err := p.api.GetTVShow(e.ParentID)
		if err != nil {
			return err
		}
		if show.TMDBID == 0 {
			return nil
		}
		return p.readyEpisode(show, e.MediaID, e.SeasonID, e.Title)
	}
	// Show just enriched; fan out over its already-transcoded episodes.
	show, err := p.api.GetTVShow(e.MediaID)
	if err != nil {
		return err
	}
	if show.TMDBID == 0 {
		return nil
	}
	seasons, err := p.api.ListSeasons(show.ID)
	if err != nil {
		return err
	}
	for _, s := range seasons {
		eps, err := p.api.ListEpisodes(show.ID, s.ID)
		if err != nil {
			return err
		}
		for _, ep := range eps {
			if ep.FilePath == "" {
				continue
			}
			if err := p.readyEpisode(show, ep.ID, ep.SeasonID, ep.Title); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p *Processor) readyEpisode(show *apiclient.TVShow, episodeID, seasonID, title string) error {
	return p.emitReady("episode:"+episodeID, events.LifecycleEvent{
		Kind: events.KindReady, Type: "tvshow", MediaID: episodeID,
		ParentID: show.ID, SeasonID: seasonID, LibraryID: show.LibraryID, Title: title,
	})
}

// --- music: track transcoded + album enriched (cover_path) ---

func (p *Processor) joinMusic(e events.LifecycleEvent) error {
	if e.Kind == events.KindTranscoded {
		album, err := p.api.GetAlbum(e.ParentID)
		if err != nil {
			return err
		}
		if album.CoverPath == "" {
			return nil
		}
		return p.readyTrack(album, e.MediaID, e.Title)
	}
	album, err := p.api.GetAlbum(e.MediaID)
	if err != nil {
		return err
	}
	if album.CoverPath == "" {
		return nil
	}
	tracks, err := p.api.ListAlbumTracks(album.ID)
	if err != nil {
		return err
	}
	for _, t := range tracks {
		if t.FilePath == "" {
			continue
		}
		if err := p.readyTrack(album, t.ID, t.Title); err != nil {
			return err
		}
	}
	return nil
}

func (p *Processor) readyTrack(album *apiclient.Album, trackID, title string) error {
	return p.emitReady("track:"+trackID, events.LifecycleEvent{
		Kind: events.KindReady, Type: "music", MediaID: trackID,
		ParentID: album.ID, LibraryID: album.LibraryID, Title: title,
	})
}

// --- audiobook: book-level ready (enriched + >=1 transcoded chapter) ---

func (p *Processor) joinAudiobook(e events.LifecycleEvent) error {
	// Resolve the audiobook id: it's ParentID on a chapter-transcoded event,
	// MediaID on a book-enriched event.
	bookID := e.MediaID
	if e.Kind == events.KindTranscoded {
		bookID = e.ParentID
	}
	book, err := p.api.GetAudiobook(bookID)
	if err != nil {
		return err
	}
	if book.OpenLibraryKey == "" {
		return nil // not enriched yet
	}
	if e.Kind == events.KindEnriched {
		// Confirm at least one chapter is transcoded before declaring ready.
		chapters, err := p.api.ListChapters(book.ID)
		if err != nil {
			return err
		}
		if !anyTranscoded(chapters) {
			return nil
		}
	}
	// On a transcoded event the chapter that triggered us is itself proof of a
	// transcoded chapter.
	return p.emitReady("audiobook:"+book.ID, events.LifecycleEvent{
		Kind: events.KindReady, Type: "audiobook", MediaID: book.ID,
		LibraryID: book.LibraryID, Title: book.Title,
	})
}

func anyTranscoded(chapters []apiclient.Chapter) bool {
	for _, c := range chapters {
		if c.FilePath != "" {
			return true
		}
	}
	return false
}

// --- shared emit + dedupe ---

// emitReady publishes ready once per dedupe key. A publish failure releases the
// claim so a retry can re-attempt.
func (p *Processor) emitReady(dedupeKey string, ready events.LifecycleEvent) error {
	if !p.claim(dedupeKey) {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := p.pub.Publish(ctx, ready); err != nil {
		p.unclaim(dedupeKey)
		return err
	}
	log.Printf("INFO %s media_id=%s %q", ready.RoutingKey(), ready.MediaID, ready.Title)
	return nil
}

func (p *Processor) claim(key string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.readyEmitted[key] {
		return false
	}
	p.readyEmitted[key] = true
	return true
}

func (p *Processor) unclaim(key string) {
	p.mu.Lock()
	delete(p.readyEmitted, key)
	p.mu.Unlock()
}
