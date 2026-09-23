package embed

import (
	"strings"
	"testing"

	"river-discord/internal/apiclient"
)

func TestMovie_RichEmbed(t *testing.T) {
	m := apiclient.Movie{
		Title: "Inception", Year: 2010, Description: "A thief who steals secrets.",
		Genres: `["Action","Sci-Fi"]`, Rating: 8.4, Certification: "12A",
		PosterPath: "https://img/p.jpg", BackdropPath: "https://img/b.jpg",
	}
	msg := Movie(m)

	if !strings.Contains(msg.Content, "Inception") || !strings.Contains(msg.Content, "ready to watch") {
		t.Fatalf("content = %q", msg.Content)
	}
	if len(msg.Embeds) != 1 {
		t.Fatalf("want 1 embed, got %d", len(msg.Embeds))
	}
	e := msg.Embeds[0]
	if e.Title != "Inception (2010)" {
		t.Fatalf("title = %q", e.Title)
	}
	if e.Color != colorMovie {
		t.Fatalf("color = %d", e.Color)
	}
	if e.Thumbnail == nil || e.Thumbnail.URL != "https://img/p.jpg" {
		t.Fatalf("thumbnail = %+v", e.Thumbnail)
	}
	if e.Image == nil || e.Image.URL != "https://img/b.jpg" {
		t.Fatalf("image = %+v", e.Image)
	}
	if !hasField(e, "Genre", "Action, Sci-Fi") {
		t.Fatalf("missing genre field: %+v", e.Fields)
	}
	if !hasField(e, "Rated", "12A") {
		t.Fatalf("missing certification field: %+v", e.Fields)
	}
}

func TestMovie_OmitsEmptyFieldsAndLocalImages(t *testing.T) {
	// No year, no genres/rating/cert, and a local (non-http) poster path.
	m := apiclient.Movie{Title: "Untitled", PosterPath: "/local/poster.jpg"}
	e := Movie(m).Embeds[0]

	if e.Title != "Untitled" {
		t.Fatalf("title = %q (year should be omitted)", e.Title)
	}
	if e.Thumbnail != nil {
		t.Fatal("local poster path must not become an image ref")
	}
	if len(e.Fields) != 0 {
		t.Fatalf("expected no fields, got %+v", e.Fields)
	}
}

func TestTVShow_IncludesEpisode(t *testing.T) {
	s := apiclient.TVShow{Title: "Severance", Year: 2022, PosterPath: "https://img/s.jpg"}
	msg := TVShow(s, "Good News About Hell")
	if !strings.Contains(msg.Content, "Severance") {
		t.Fatalf("content = %q", msg.Content)
	}
	if !hasField(msg.Embeds[0], "Episode", "• Good News About Hell") {
		t.Fatalf("missing episode field: %+v", msg.Embeds[0].Fields)
	}
}

func TestTVShowEpisodes_BatchDigest(t *testing.T) {
	s := apiclient.TVShow{Title: "Severance", Year: 2022}
	msg := TVShowEpisodes(s, []string{"Ep1", "Ep2", "Ep3"})
	if !strings.Contains(msg.Content, "3 new episodes ready") {
		t.Fatalf("content = %q", msg.Content)
	}
	e := msg.Embeds[0]
	val := ""
	for _, f := range e.Fields {
		if f.Name == "Episodes" {
			val = f.Value
		}
	}
	for _, want := range []string{"• Ep1", "• Ep2", "• Ep3"} {
		if !strings.Contains(val, want) {
			t.Fatalf("episode list %q missing %q", val, want)
		}
	}
}

func TestTVShowEpisodes_ManyTruncated(t *testing.T) {
	titles := make([]string, 15)
	for i := range titles {
		titles[i] = "E" + string(rune('a'+i))
	}
	msg := TVShowEpisodes(apiclient.TVShow{Title: "Show"}, titles)
	val := msg.Embeds[0].Fields[0].Value
	if !strings.Contains(val, "and 5 more") {
		t.Fatalf("expected summary of overflow, got %q", val)
	}
}

func TestMusic_WithAndWithoutArtist(t *testing.T) {
	a := apiclient.Album{Title: "Random Access Memories", Year: 2013, CoverPath: "https://img/c.jpg"}
	withArtist := Music(a, "Daft Punk")
	if !strings.Contains(withArtist.Content, "by Daft Punk") {
		t.Fatalf("content = %q", withArtist.Content)
	}
	if !hasField(withArtist.Embeds[0], "Artist", "Daft Punk") {
		t.Fatalf("missing artist field")
	}
	noArtist := Music(a, "")
	if strings.Contains(noArtist.Content, " by ") {
		t.Fatalf("should omit 'by' when artist unknown: %q", noArtist.Content)
	}
}

func TestAudiobook_Embed(t *testing.T) {
	b := apiclient.Audiobook{Title: "Project Hail Mary", Author: "Andy Weir", Narrator: "Ray Porter", Year: 2021}
	msg := Audiobook(b)
	if !strings.Contains(msg.Content, "by Andy Weir") {
		t.Fatalf("content = %q", msg.Content)
	}
	e := msg.Embeds[0]
	if !hasField(e, "Author", "Andy Weir") || !hasField(e, "Narrator", "Ray Porter") {
		t.Fatalf("missing author/narrator fields: %+v", e.Fields)
	}
}

func TestTruncate(t *testing.T) {
	long := strings.Repeat("a", 600)
	got := truncate(long, 500)
	if len([]rune(got)) > 500 {
		t.Fatalf("truncate exceeded limit: %d runes", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("expected ellipsis suffix")
	}
	if truncate("short", 500) != "short" {
		t.Fatal("short strings unchanged")
	}
}

func hasField(e Embed, name, value string) bool {
	for _, f := range e.Fields {
		if f.Name == name && f.Value == value {
			return true
		}
	}
	return false
}
