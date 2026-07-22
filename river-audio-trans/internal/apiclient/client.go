package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
)

type Client struct {
	baseURL  string
	username string
	password string
	service  string
	token    string
	mu       sync.Mutex
	http     *http.Client
}

func New(baseURL, username, password, service string) *Client {
	return &Client{
		baseURL:  baseURL,
		username: username,
		password: password,
		service:  service,
		http:     &http.Client{},
	}
}

func (c *Client) Log(level, message string) {
	go func() {
		body := map[string]string{"level": level, "service": c.service, "message": message}
		_ = c.do("POST", "/api/logs", body, nil)
	}()
}

func (c *Client) Login() error {
	body, _ := json.Marshal(map[string]string{
		"username": c.username,
		"password": c.password,
	})
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
	c.mu.Lock()
	token := c.token
	c.mu.Unlock()
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized && retry {
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

// --- Artists ---

type Artist struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Name      string `json:"name"`
}

type ArtistRequest struct {
	LibraryID string `json:"library_id"`
	Name      string `json:"name"`
}

func (c *Client) ListArtists(libraryID string) ([]Artist, error) {
	return paginateAll(func(page, limit int) ([]Artist, error) {
		var result []Artist
		path := fmt.Sprintf("/api/artists?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateArtist(req ArtistRequest) (*Artist, error) {
	var result Artist
	return &result, c.do("POST", "/api/artists", req, &result)
}

// --- Albums ---

type Album struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	ArtistID  string `json:"artist_id"`
	Title     string `json:"title"`
	Year      int    `json:"year"`
}

type AlbumRequest struct {
	LibraryID string `json:"library_id"`
	ArtistID  string `json:"artist_id,omitempty"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
}

func (c *Client) ListAlbums(libraryID string) ([]Album, error) {
	return paginateAll(func(page, limit int) ([]Album, error) {
		var result []Album
		path := fmt.Sprintf("/api/albums?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateAlbum(req AlbumRequest) (*Album, error) {
	var result Album
	return &result, c.do("POST", "/api/albums", req, &result)
}

// --- Tracks ---

type Track struct {
	ID       string `json:"id"`
	AlbumID  string `json:"album_id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
	FilePath string `json:"file_path"`
}

type TrackRequest struct {
	LibraryID  string `json:"library_id"`
	AlbumID    string `json:"album_id"`
	ArtistID   string `json:"artist_id,omitempty"`
	Title      string `json:"title"`
	Number     int    `json:"number,omitempty"`
	DiscNumber int    `json:"disc_number,omitempty"`
	Duration   int    `json:"duration,omitempty"`
	FilePath   string `json:"file_path"`
}

func (c *Client) ListAlbumTracks(albumID string) ([]Track, error) {
	return paginateAll(func(page, limit int) ([]Track, error) {
		var result []Track
		path := fmt.Sprintf("/api/albums/%s/tracks?page=%d&limit=%d", albumID, page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateTrack(req TrackRequest) (*Track, error) {
	var result Track
	return &result, c.do("POST", "/api/tracks", req, &result)
}

// --- Audiobooks ---

type Audiobook struct {
	ID        string `json:"id"`
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
}

type AudiobookRequest struct {
	LibraryID string `json:"library_id"`
	Title     string `json:"title"`
	Year      int    `json:"year,omitempty"`
}

func (c *Client) ListAudiobooks(libraryID string) ([]Audiobook, error) {
	return paginateAll(func(page, limit int) ([]Audiobook, error) {
		var result []Audiobook
		path := fmt.Sprintf("/api/audiobooks?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateAudiobook(req AudiobookRequest) (*Audiobook, error) {
	var result Audiobook
	return &result, c.do("POST", "/api/audiobooks", req, &result)
}

// --- Chapters ---

type Chapter struct {
	ID          string `json:"id"`
	AudiobookID string `json:"audiobook_id"`
	Number      int    `json:"number"`
	Title       string `json:"title"`
	FilePath    string `json:"file_path"`
}

type ChapterRequest struct {
	Number   int    `json:"number"`
	Title    string `json:"title,omitempty"`
	Duration int    `json:"duration,omitempty"`
	FilePath string `json:"file_path"`
}

func (c *Client) ListChapters(audiobookID string) ([]Chapter, error) {
	var result []Chapter
	return result, c.do("GET", "/api/audiobooks/"+audiobookID+"/chapters", nil, &result)
}

func (c *Client) CreateChapter(audiobookID string, req ChapterRequest) (*Chapter, error) {
	var result Chapter
	return &result, c.do("POST", "/api/audiobooks/"+audiobookID+"/chapters", req, &result)
}

func paginateAll[T any](fetch func(page, limit int) ([]T, error)) ([]T, error) {
	const limit = 200
	var all []T
	for page := 1; ; page++ {
		items, err := fetch(page, limit)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		if len(items) < limit {
			break
		}
	}
	return all, nil
}
