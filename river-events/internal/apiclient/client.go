// Package apiclient is a thin read-mostly client river-events uses to fetch
// media records from river-api when computing the transcode+enrich join.
package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	baseURL  string
	username string
	password string
	apiKey   string
	service  string
	token    string
	mu       sync.Mutex
	http     *http.Client
}

func New(baseURL, username, password, apiKey, service string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		apiKey:   apiKey,
		service:  service,
		http:     &http.Client{Timeout: 30 * time.Second},
	}
}

// Login authenticates with username/password. A no-op when an API key is
// configured (the key is sent directly as the bearer on every request).
func (c *Client) Login() error {
	if c.apiKey != "" {
		return nil
	}
	body, _ := json.Marshal(map[string]string{"username": c.username, "password": c.password})
	resp, err := c.http.Post(c.baseURL+"/api/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("login: status %d", resp.StatusCode)
	}
	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("login decode: %w", err)
	}
	c.mu.Lock()
	c.token = result.AccessToken
	c.mu.Unlock()
	return nil
}

func (c *Client) Log(level, message string) {
	go func() {
		body := map[string]string{"level": level, "service": c.service, "message": message}
		_ = c.do("POST", "/api/logs", body, nil)
	}()
}

func (c *Client) do(method, path string, body, out interface{}) error {
	return c.doWithRetry(method, path, body, out, true)
}

func (c *Client) doWithRetry(method, path string, body, out interface{}, retry bool) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	token := c.apiKey
	if token == "" {
		c.mu.Lock()
		token = c.token
		c.mu.Unlock()
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && retry && c.apiKey == "" {
		if err := c.Login(); err != nil {
			return fmt.Errorf("re-login: %w", err)
		}
		return c.doWithRetry(method, path, body, out, false)
	}
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s: status %d: %s", method, path, resp.StatusCode, string(b))
	}
	if out != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// Movie is the subset of a movie record river-events needs for the join.
type Movie struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
	FilePath  string `json:"file_path"`
	TMDBID    int    `json:"tmdb_id"`
}

func (c *Client) GetMovie(id string) (*Movie, error) {
	var m Movie
	return &m, c.do("GET", "/api/movies/"+id, nil, &m)
}

// --- TV ---

type TVShow struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
	TMDBID    int    `json:"tmdb_id"`
}

type Season struct {
	ID string `json:"id"`
}

type Episode struct {
	ID       string `json:"id"`
	SeasonID string `json:"season_id"`
	Title    string `json:"title"`
	FilePath string `json:"file_path"`
}

func (c *Client) GetTVShow(id string) (*TVShow, error) {
	var s TVShow
	return &s, c.do("GET", "/api/tvshows/"+id, nil, &s)
}

func (c *Client) ListSeasons(showID string) ([]Season, error) {
	var out []Season
	return out, c.do("GET", "/api/tvshows/"+showID+"/seasons?limit=200", nil, &out)
}

func (c *Client) ListEpisodes(showID, seasonID string) ([]Episode, error) {
	var out []Episode
	return out, c.do("GET", "/api/tvshows/"+showID+"/seasons/"+seasonID+"/episodes?limit=200", nil, &out)
}

// --- Music ---

type Album struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
	CoverPath string `json:"cover_path"`
}

type Track struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	FilePath string `json:"file_path"`
}

func (c *Client) GetAlbum(id string) (*Album, error) {
	var a Album
	return &a, c.do("GET", "/api/albums/"+id, nil, &a)
}

func (c *Client) ListAlbumTracks(albumID string) ([]Track, error) {
	var out []Track
	return out, c.do("GET", "/api/albums/"+albumID+"/tracks?limit=200", nil, &out)
}

// --- Audiobooks ---

type Audiobook struct {
	ID             string `json:"id"`
	LibraryID      string `json:"library_id"`
	Title          string `json:"title"`
	OpenLibraryKey string `json:"open_library_key"`
}

type Chapter struct {
	ID       string `json:"id"`
	FilePath string `json:"file_path"`
}

func (c *Client) GetAudiobook(id string) (*Audiobook, error) {
	var a Audiobook
	return &a, c.do("GET", "/api/audiobooks/"+id, nil, &a)
}

func (c *Client) ListChapters(audiobookID string) ([]Chapter, error) {
	var out []Chapter
	return out, c.do("GET", "/api/audiobooks/"+audiobookID+"/chapters?limit=200", nil, &out)
}

// --- Webhooks ---

// ActiveWebhook mirrors river-api's delivery-facing shape (includes the HMAC
// secret so river-events can sign deliveries).
type ActiveWebhook struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Secret string   `json:"secret"`
	Events []string `json:"events"`
}

func (c *Client) GetActiveWebhooks() ([]ActiveWebhook, error) {
	var out []ActiveWebhook
	return out, c.do("GET", "/api/webhooks/active", nil, &out)
}

// DeliveryOutcome is the terminal result river-events reports per delivery.
type DeliveryOutcome struct {
	WebhookID    string `json:"webhook_id"`
	Event        string `json:"event"`
	MediaID      string `json:"media_id"`
	Status       string `json:"status"` // "delivered" | "failed"
	Attempts     int    `json:"attempts"`
	ResponseCode int    `json:"response_code,omitempty"`
	Error        string `json:"error,omitempty"`
}

func (c *Client) RecordDelivery(o DeliveryOutcome) error {
	return c.do("POST", "/api/webhooks/deliveries", o, nil)
}
