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

func (c *Client) do(method, path string, body interface{}, out interface{}) error {
	return c.doWithRetry(method, path, body, out, true)
}

func (c *Client) doWithRetry(method, path string, body interface{}, out interface{}, retry bool) error {
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

// --- Movies ---

type Movie struct {
	ID            string  `json:"id"`
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Runtime       int     `json:"runtime"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	FilePath      string  `json:"file_path"`
}

type MovieRequest struct {
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Runtime       int     `json:"runtime"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	FilePath      string  `json:"file_path"`
	SourcePath    string  `json:"source_path,omitempty"`
}

func (c *Client) GetMovie(id string) (*Movie, error) {
	var result Movie
	return &result, c.do("GET", "/api/movies/"+id, nil, &result)
}

func (c *Client) ListMovies(libraryID string) ([]Movie, error) {
	return paginateAll(func(page, limit int) ([]Movie, error) {
		var result []Movie
		path := fmt.Sprintf("/api/movies?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateMovie(req MovieRequest) (*Movie, error) {
	var result Movie
	return &result, c.do("POST", "/api/movies", req, &result)
}

func (c *Client) UpdateMovie(id string, req MovieRequest) (*Movie, error) {
	var result Movie
	return &result, c.do("PUT", "/api/movies/"+id, req, &result)
}

// UpdateMovieFilePath targets only the FilePath field so a long-running
// transcode can't clobber metadata that river-meta-movie wrote concurrently.
func (c *Client) UpdateMovieFilePath(id, path string) error {
	body := map[string]string{"file_path": path}
	return c.do("PATCH", "/api/movies/"+id+"/file-path", body, nil)
}

// --- TV Shows ---

type TVShow struct {
	ID            string  `json:"id"`
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Status        string  `json:"status"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
}

type TVShowRequest struct {
	LibraryID     string  `json:"library_id"`
	Title         string  `json:"title"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Status        string  `json:"status"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
}

func (c *Client) GetTVShow(id string) (*TVShow, error) {
	var result TVShow
	return &result, c.do("GET", "/api/tvshows/"+id, nil, &result)
}

func (c *Client) ListTVShows(libraryID string) ([]TVShow, error) {
	return paginateAll(func(page, limit int) ([]TVShow, error) {
		var result []TVShow
		path := fmt.Sprintf("/api/tvshows?library_id=%s&page=%d&limit=%d",
			url.QueryEscape(libraryID), page, limit)
		return result, c.do("GET", path, nil, &result)
	})
}

func (c *Client) CreateTVShow(req TVShowRequest) (*TVShow, error) {
	var result TVShow
	return &result, c.do("POST", "/api/tvshows", req, &result)
}

// --- Seasons ---

type Season struct {
	ID       string `json:"id"`
	TVShowID string `json:"tv_show_id"`
	Number   int    `json:"number"`
	Title    string `json:"title"`
}

type SeasonRequest struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Year        int    `json:"year"`
}

func (c *Client) ListSeasons(showID string) ([]Season, error) {
	var result []Season
	return result, c.do("GET", "/api/tvshows/"+showID+"/seasons", nil, &result)
}

func (c *Client) CreateSeason(showID string, req SeasonRequest) (*Season, error) {
	var result Season
	return &result, c.do("POST", "/api/tvshows/"+showID+"/seasons", req, &result)
}

// --- Episodes ---

type Episode struct {
	ID         string `json:"id"`
	SeasonID   string `json:"season_id"`
	TVShowID   string `json:"tv_show_id"`
	Number     int    `json:"number"`
	Title      string `json:"title"`
	FilePath   string `json:"file_path"`
	SourcePath string `json:"source_path"`
	IsSpecial  bool   `json:"is_special"`
}

type EpisodeRequest struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Runtime     int    `json:"runtime"`
	FilePath    string `json:"file_path"`
	SourcePath  string `json:"source_path,omitempty"`
	IsSpecial   bool   `json:"is_special,omitempty"`
}

func (c *Client) ListEpisodes(showID, seasonID string) ([]Episode, error) {
	var result []Episode
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes", showID, seasonID)
	return result, c.do("GET", path, nil, &result)
}

func (c *Client) CreateEpisode(showID, seasonID string, req EpisodeRequest) (*Episode, error) {
	var result Episode
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes", showID, seasonID)
	return &result, c.do("POST", path, req, &result)
}

func (c *Client) UpdateEpisode(showID, seasonID, episodeID string, req EpisodeRequest) (*Episode, error) {
	var result Episode
	path := fmt.Sprintf("/api/tvshows/%s/seasons/%s/episodes/%s", showID, seasonID, episodeID)
	return &result, c.do("PUT", path, req, &result)
}

// --- Audio tracks ---

type AudioTrackRequest struct {
	MediaType   string `json:"media_type"`
	MediaID     string `json:"media_id"`
	Language    string `json:"language"`
	Label       string `json:"label"`
	StreamIndex int    `json:"stream_index"`
	FilePath    string `json:"file_path"`
}

type AudioTrack struct {
	ID          string `json:"id"`
	MediaType   string `json:"media_type"`
	MediaID     string `json:"media_id"`
	Language    string `json:"language"`
	Label       string `json:"label"`
	StreamIndex int    `json:"stream_index"`
}

func (c *Client) CreateAudioTrack(req AudioTrackRequest) (*AudioTrack, error) {
	var result AudioTrack
	return &result, c.do("POST", "/api/audio-tracks", req, &result)
}

// --- Subtitles ---

type SubtitleRequest struct {
	MediaType string `json:"media_type"`
	MediaID   string `json:"media_id"`
	Language  string `json:"language"`
	Label     string `json:"label"`
	FilePath  string `json:"file_path"`
}

type Subtitle struct {
	ID        string `json:"id"`
	MediaType string `json:"media_type"`
	MediaID   string `json:"media_id"`
	Language  string `json:"language"`
	Label     string `json:"label"`
	FilePath  string `json:"file_path"`
}

func (c *Client) CreateSubtitle(req SubtitleRequest) (*Subtitle, error) {
	var result Subtitle
	return &result, c.do("POST", "/api/subtitles", req, &result)
}

// paginateAll fetches pages until the response is smaller than the page size.
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

