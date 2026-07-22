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
	TrailerURL    string  `json:"trailer_url"`
	FilePath      string  `json:"file_path"`
	TMDBID        int     `json:"tmdb_id"`
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
	TrailerURL    string  `json:"trailer_url"`
	FilePath      string  `json:"file_path"`
	TMDBID        int     `json:"tmdb_id"`
}

type CastCredit struct {
	TmdbID      int    `json:"tmdb_id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
	Biography   string `json:"biography"`
	Character   string `json:"character"`
	Order       int    `json:"order"`
}

type CrewCredit struct {
	TmdbID      int    `json:"tmdb_id"`
	Name        string `json:"name"`
	ProfilePath string `json:"profile_path"`
	Biography   string `json:"biography"`
	Job         string `json:"job"`
	Department  string `json:"department"`
}

type CreditsRequest struct {
	Cast []CastCredit `json:"cast"`
	Crew []CrewCredit `json:"crew"`
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

func (c *Client) UpdateMovie(id string, req MovieRequest) (*Movie, error) {
	var result Movie
	return &result, c.do("PUT", "/api/movies/"+id, req, &result)
}

func (c *Client) SetMovieCredits(id string, req CreditsRequest) error {
	return c.do("PUT", "/api/movies/"+id+"/credits", req, nil)
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
