// Package notifier is the glue: it filters lifecycle events to the configured
// kinds, batches a burst per title, then enriches from river-api, builds a
// Discord embed, and posts it to the routed channel.
package notifier

import (
	"fmt"
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
	ListSeasons(showID string) ([]apiclient.Season, error)
	ListEpisodes(showID, seasonID string) ([]apiclient.Episode, error)
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
		return embed.TVShowEpisodes(*s, n.episodeLabels(e0.ParentID, evs)), true

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

// episodeLabels builds the per-episode lines for the digest, prefixing each
// title with its SxxExx code (e.g. "S02E05 — The Suitcase") when the numbers
// resolve from river-api. It is best-effort: the numbers aren't in the webhook
// envelope, so any lookup failure degrades to the plain title rather than
// dropping the post. Batch order is preserved.
func (n *Notifier) episodeLabels(showID string, evs []events.LifecycleEvent) []string {
	seasonNum := n.seasonNumbers(showID)          // season id → season number
	episodes := n.episodesByID(showID, evs)       // episode id → episode record

	out := make([]string, 0, len(evs))
	for _, e := range evs {
		ep, ok := episodes[e.MediaID]
		title := e.Title
		if ok && ep.Title != "" {
			title = ep.Title
		}
		var code string
		if ok {
			code = episodeCode(seasonNum[ep.SeasonID], ep.Number, ep.IsSpecial)
		}
		switch {
		case code != "" && title != "":
			out = append(out, code+" — "+title)
		case code != "":
			out = append(out, code)
		case title != "":
			out = append(out, title)
		}
	}
	return out
}

// seasonNumbers maps a show's season ids to their numbers. Best-effort: an empty
// map (on error) just means episodes fall back to title-only labels.
func (n *Notifier) seasonNumbers(showID string) map[string]int {
	out := map[string]int{}
	seasons, err := n.api.ListSeasons(showID)
	if err != nil {
		log.Printf("WARN episode labels: list seasons %s: %v", showID, err)
		return out
	}
	for _, s := range seasons {
		out[s.ID] = s.Number
	}
	return out
}

// episodesByID resolves the batch's episodes, fetching each distinct season's
// episode list once. Best-effort per the same rationale as seasonNumbers.
func (n *Notifier) episodesByID(showID string, evs []events.LifecycleEvent) map[string]apiclient.Episode {
	out := map[string]apiclient.Episode{}
	fetched := map[string]bool{}
	for _, e := range evs {
		if e.SeasonID == "" || fetched[e.SeasonID] {
			continue
		}
		fetched[e.SeasonID] = true
		eps, err := n.api.ListEpisodes(showID, e.SeasonID)
		if err != nil {
			log.Printf("WARN episode labels: list episodes %s/%s: %v", showID, e.SeasonID, err)
			continue
		}
		for _, ep := range eps {
			out[ep.ID] = ep
		}
	}
	return out
}

// episodeCode renders an episode's "SxxExx" code, or "Sxx · Special" for a
// special. Returns "" when the season number is unknown so the caller falls
// back to the title alone.
func episodeCode(seasonNum, episodeNum int, special bool) string {
	if seasonNum <= 0 {
		return ""
	}
	if special {
		return fmt.Sprintf("S%02d · Special", seasonNum)
	}
	return fmt.Sprintf("S%02dE%02d", seasonNum, episodeNum)
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
