// Package apiclient reads media records from river-api for embed enrichment.
// It authenticates with a read-only API token (rvat_…) as a Bearer credential
// and requests only the fields the notifier renders.
package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{Timeout: 10 * time.Second}}
}

// Records mirror the river-api JSON, carrying only the fields the embeds use.
// Genres is a JSON-encoded []string on the wire (river-api stores it as a
// string); use ParseGenres to decode it.

type Movie struct {
	ID            string  `json:"id"`
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	Year          int     `json:"year"`
	Description   string  `json:"description"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Certification string  `json:"certification"`
	Runtime       int     `json:"runtime"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
}

type TVShow struct {
	ID            string  `json:"id"`
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	Year          int     `json:"year"`
	Description   string  `json:"description"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Certification string  `json:"certification"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
}

type Album struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	ArtistID  string `json:"artist_id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
	Genre     string `json:"genre"`
	CoverPath string `json:"cover_path"`
}

type Artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Audiobook struct {
	ID          string `json:"id"`
	LibraryID   string `json:"library_id"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Narrator    string `json:"narrator"`
	Description string `json:"description"`
	Year        int    `json:"year"`
	Genre       string `json:"genre"`
	CoverPath   string `json:"cover_path"`
}

func (c *Client) GetMovie(id string) (*Movie, error) {
	var m Movie
	return &m, c.get("/movies/"+id, &m)
}

func (c *Client) GetTVShow(id string) (*TVShow, error) {
	var s TVShow
	return &s, c.get("/tvshows/"+id, &s)
}

func (c *Client) GetAlbum(id string) (*Album, error) {
	var a Album
	return &a, c.get("/albums/"+id, &a)
}

func (c *Client) GetArtist(id string) (*Artist, error) {
	var a Artist
	return &a, c.get("/artists/"+id, &a)
}

func (c *Client) GetAudiobook(id string) (*Audiobook, error) {
	var a Audiobook
	return &a, c.get("/audiobooks/"+id, &a)
}

// get performs an authenticated GET and decodes the JSON body into out.
func (c *Client) get(path string, out any) error {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("GET %s: status %d: %s", path, resp.StatusCode, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// ParseGenres decodes river-api's JSON-encoded genres string into a slice. A
// blank or malformed value yields nil rather than an error — genres are
// cosmetic in the embed.
func ParseGenres(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return nil
	}
	return out
}
