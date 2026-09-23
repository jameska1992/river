// Package embed builds Discord webhook messages (rich embeds) from river-api
// records. Builders are pure — no HTTP, no config — so they're trivially
// testable; the delivery layer (Phase C) marshals and POSTs the result.
package embed

import (
	"fmt"
	"strings"

	"river-discord/internal/apiclient"
)

// Per-type accent colours (Discord uses a decimal RGB int).
const (
	colorMovie     = 0xE5A00D // gold
	colorTVShow    = 0x5865F2 // blurple
	colorMusic     = 0x1DB954 // green
	colorAudiobook = 0x8E44AD // purple
)

// Discord message + embed shapes (only the fields we set). See
// https://discord.com/developers/docs/resources/webhook#execute-webhook.
type Message struct {
	Username  string  `json:"username,omitempty"`
	AvatarURL string  `json:"avatar_url,omitempty"`
	Content   string  `json:"content,omitempty"`
	Embeds    []Embed `json:"embeds,omitempty"`
}

type Embed struct {
	Title       string  `json:"title,omitempty"`
	Description string  `json:"description,omitempty"`
	Color       int     `json:"color,omitempty"`
	Fields      []Field `json:"fields,omitempty"`
	Thumbnail   *Media  `json:"thumbnail,omitempty"`
	Image       *Media  `json:"image,omitempty"`
	Footer      *Footer `json:"footer,omitempty"`
}

type Field struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

type Media struct {
	URL string `json:"url"`
}

type Footer struct {
	Text string `json:"text"`
}

const username = "River"

// Movie builds a "ready to watch" embed for a movie.
func Movie(m apiclient.Movie) Message {
	e := Embed{
		Title:       titleWithYear(m.Title, m.Year),
		Description: truncate(m.Description, 500),
		Color:       colorMovie,
		Thumbnail:   imageURL(m.PosterPath),
		Image:       imageURL(m.BackdropPath),
		Footer:      &Footer{Text: "River · Ready to watch"},
	}
	e.Fields = appendField(e.Fields, "Genre", genreValue(m.Genres), true)
	e.Fields = appendField(e.Fields, "Rating", ratingValue(m.Rating), true)
	e.Fields = appendField(e.Fields, "Rated", m.Certification, true)
	return Message{
		Username: username,
		Content:  fmt.Sprintf("🎬 **%s** is ready to watch", m.Title),
		Embeds:   []Embed{e},
	}
}

// TVShow builds a "new episode ready" embed for a single episode.
func TVShow(s apiclient.TVShow, episodeTitle string) Message {
	var titles []string
	if episodeTitle != "" {
		titles = []string{episodeTitle}
	}
	return TVShowEpisodes(s, titles)
}

// TVShowEpisodes builds one embed for a batch of episodes of the same show that
// became ready together — the digest form used when a burst is coalesced.
func TVShowEpisodes(s apiclient.TVShow, episodeTitles []string) Message {
	e := Embed{
		Title:       titleWithYear(s.Title, s.Year),
		Description: truncate(s.Description, 500),
		Color:       colorTVShow,
		Thumbnail:   imageURL(s.PosterPath),
		Image:       imageURL(s.BackdropPath),
		Footer:      &Footer{Text: "River · Ready to watch"},
	}
	e.Fields = appendField(e.Fields, episodeFieldName(len(episodeTitles)), episodeList(episodeTitles), false)
	e.Fields = appendField(e.Fields, "Genre", genreValue(s.Genres), true)
	e.Fields = appendField(e.Fields, "Rating", ratingValue(s.Rating), true)

	content := fmt.Sprintf("📺 **%s** — new episode ready", s.Title)
	switch {
	case len(episodeTitles) > 1:
		content = fmt.Sprintf("📺 **%s** — %d new episodes ready", s.Title, len(episodeTitles))
	case len(episodeTitles) == 1 && episodeTitles[0] != "":
		content += ": " + episodeTitles[0]
	}
	return Message{Username: username, Content: content, Embeds: []Embed{e}}
}

func episodeFieldName(n int) string {
	if n > 1 {
		return "Episodes"
	}
	return "Episode"
}

// episodeList renders up to 10 titles as a bulleted list, summarising the rest.
func episodeList(titles []string) string {
	const max = 10
	var b strings.Builder
	for i, t := range titles {
		if i == max {
			fmt.Fprintf(&b, "…and %d more", len(titles)-max)
			break
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("• " + t)
	}
	return b.String()
}

// Music builds a "album ready" embed. artist may be empty if unresolved.
func Music(a apiclient.Album, artist string) Message {
	e := Embed{
		Title:     titleWithYear(a.Title, a.Year),
		Color:     colorMusic,
		Thumbnail: imageURL(a.CoverPath),
		Footer:    &Footer{Text: "River · Ready to play"},
	}
	e.Fields = appendField(e.Fields, "Artist", artist, true)
	e.Fields = appendField(e.Fields, "Genre", a.Genre, true)
	content := fmt.Sprintf("🎵 **%s** is ready to play", a.Title)
	if artist != "" {
		content = fmt.Sprintf("🎵 **%s** by %s is ready to play", a.Title, artist)
	}
	return Message{Username: username, Content: content, Embeds: []Embed{e}}
}

// Audiobook builds a "audiobook ready" embed.
func Audiobook(a apiclient.Audiobook) Message {
	e := Embed{
		Title:       titleWithYear(a.Title, a.Year),
		Description: truncate(a.Description, 500),
		Color:       colorAudiobook,
		Thumbnail:   imageURL(a.CoverPath),
		Footer:      &Footer{Text: "River · Ready to listen"},
	}
	e.Fields = appendField(e.Fields, "Author", a.Author, true)
	e.Fields = appendField(e.Fields, "Narrator", a.Narrator, true)
	e.Fields = appendField(e.Fields, "Genre", a.Genre, true)
	content := fmt.Sprintf("🎧 **%s** is ready to listen", a.Title)
	if a.Author != "" {
		content = fmt.Sprintf("🎧 **%s** by %s is ready to listen", a.Title, a.Author)
	}
	return Message{Username: username, Content: content, Embeds: []Embed{e}}
}

// --- helpers ---

func titleWithYear(title string, year int) string {
	if year > 0 {
		return fmt.Sprintf("%s (%d)", title, year)
	}
	return title
}

// appendField adds a field only when it has a value, so embeds don't carry
// empty rows.
func appendField(fields []Field, name, value string, inline bool) []Field {
	if strings.TrimSpace(value) == "" {
		return fields
	}
	return append(fields, Field{Name: name, Value: value, Inline: inline})
}

func genreValue(encoded string) string {
	return strings.Join(apiclient.ParseGenres(encoded), ", ")
}

func ratingValue(r float32) string {
	if r <= 0 {
		return ""
	}
	return fmt.Sprintf("⭐ %.1f", r)
}

// imageURL returns an image ref only for absolute http(s) URLs — a local/relative
// path (e.g. an un-enriched cover) would render as a broken image in Discord, so
// it's omitted instead.
func imageURL(u string) *Media {
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return &Media{URL: u}
	}
	return nil
}

func truncate(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	// Trim to a rune boundary and add an ellipsis.
	cut := max - 1
	for cut > 0 && !isRuneStart(s[cut]) {
		cut--
	}
	return strings.TrimSpace(s[:cut]) + "…"
}

func isRuneStart(b byte) bool { return b&0xC0 != 0x80 }
