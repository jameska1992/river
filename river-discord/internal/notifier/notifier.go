// Package notifier is the glue: it filters lifecycle events to the configured
// kinds, batches a burst per title, then enriches from river-api, builds a
// Discord embed, and posts it to the routed channel.
package notifier

import (
	"log"
	"time"

	"river-discord/internal/apiclient"
	"river-discord/internal/batcher"
	"river-discord/internal/embed"
	"river-discord/internal/events"
)

// mediaSource is the slice of river-api the notifier reads for embeds.
type mediaSource interface {
	GetMovie(id string) (*apiclient.Movie, error)
	GetTVShow(id string) (*apiclient.TVShow, error)
	GetAlbum(id string) (*apiclient.Album, error)
	GetArtist(id string) (*apiclient.Artist, error)
	GetAudiobook(id string) (*apiclient.Audiobook, error)
}

// sender posts a built message to a Discord webhook URL.
type sender interface {
	Send(url string, msg embed.Message) error
}

type Notifier struct {
	api    mediaSource
	send   sender
	batch  *batcher.Batcher
	notify map[string]bool   // event kinds that trigger a notification
	routes map[string]string // library id / media type → Discord webhook URL
	def    string            // default Discord webhook URL
}

// New builds the notifier. It owns a batcher whose flush calls back into it.
func New(api mediaSource, send sender, window time.Duration, notify map[string]bool, routes map[string]string, defaultURL string) *Notifier {
	n := &Notifier{api: api, send: send, notify: notify, routes: routes, def: defaultURL}
	n.batch = batcher.New(window, n.flush)
	return n
}

// Handle is the server.Handler: filter, then enqueue into the batcher.
func (n *Notifier) Handle(e events.LifecycleEvent) {
	if !n.notify[e.Kind] {
		return
	}
	n.batch.Add(announceKey(e), e)
}

// Close flushes anything pending (graceful shutdown).
func (n *Notifier) Close() { n.batch.Close() }

// flush enriches the batched events into one message and delivers it.
func (n *Notifier) flush(_ string, evs []events.LifecycleEvent) {
	if len(evs) == 0 {
		return
	}
	e0 := evs[0]
	msg, ok := n.build(e0, evs)
	if !ok {
		return
	}
	url := n.route(e0)
	if err := n.send.Send(url, msg); err != nil {
		log.Printf("ERROR deliver %s (%s): %v", e0.RoutingKey(), e0.MediaID, err)
		return
	}
	log.Printf("INFO delivered %s for %q (%d event(s))", e0.Type, title(msg), len(evs))
}

// build fetches the record(s) and returns the embed message. ok is false when
// the record can't be read (logged) — the event is dropped rather than retried,
// since river-events already retries delivery of the webhook itself.
func (n *Notifier) build(e0 events.LifecycleEvent, evs []events.LifecycleEvent) (embed.Message, bool) {
	switch e0.Type {
	case "movie":
		m, err := n.api.GetMovie(e0.MediaID)
		if err != nil {
			return logDrop(e0, err)
		}
		return embed.Movie(*m), true

	case "tvshow":
		s, err := n.api.GetTVShow(e0.ParentID)
		if err != nil {
			return logDrop(e0, err)
		}
		return embed.TVShowEpisodes(*s, episodeTitles(evs)), true

	case "music":
		a, err := n.api.GetAlbum(e0.ParentID)
		if err != nil {
			return logDrop(e0, err)
		}
		return embed.Music(*a, n.artistName(a.ArtistID)), true

	case "audiobook":
		b, err := n.api.GetAudiobook(e0.MediaID)
		if err != nil {
			return logDrop(e0, err)
		}
		return embed.Audiobook(*b), true

	default:
		return embed.Message{}, false
	}
}

// artistName resolves an album's artist name, best-effort (blank on failure).
func (n *Notifier) artistName(artistID string) string {
	if artistID == "" {
		return ""
	}
	a, err := n.api.GetArtist(artistID)
	if err != nil {
		return ""
	}
	return a.Name
}

// route picks the Discord webhook URL: by library id, then media type, then the
// default channel.
func (n *Notifier) route(e events.LifecycleEvent) string {
	if url := n.routes[e.LibraryID]; url != "" {
		return url
	}
	if url := n.routes[e.Type]; url != "" {
		return url
	}
	return n.def
}

// announceKey groups events by the entity a single message announces: the show
// for an episode, the album for a track, else the item itself. This dedupes
// per-track/per-episode ready events into one notification.
func announceKey(e events.LifecycleEvent) string {
	switch e.Type {
	case "tvshow":
		return "tvshow:" + e.ParentID
	case "music":
		return "music:" + e.ParentID
	default:
		return e.Type + ":" + e.MediaID
	}
}

// episodeTitles collects the (non-empty) titles from a batch of episode events.
func episodeTitles(evs []events.LifecycleEvent) []string {
	var out []string
	for _, e := range evs {
		if e.Title != "" {
			out = append(out, e.Title)
		}
	}
	return out
}

func logDrop(e events.LifecycleEvent, err error) (embed.Message, bool) {
	log.Printf("WARN dropping %s (%s): read record: %v", e.RoutingKey(), e.MediaID, err)
	return embed.Message{}, false
}

func title(msg embed.Message) string {
	if len(msg.Embeds) > 0 {
		return msg.Embeds[0].Title
	}
	return ""
}
