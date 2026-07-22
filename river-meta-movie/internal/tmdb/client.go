package tmdb

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
)

var ErrNotFound = errors.New("tmdb: not found")

type Client struct {
	apiKey    string
	imageBase string
	http      *http.Client
}

type CastCredit struct {
	TmdbID      int
	Name        string
	ProfilePath string
	Biography   string
	Character   string
	Order       int
}

type CrewCredit struct {
	TmdbID      int
	Name        string
	ProfilePath string
	Biography   string
	Job         string
	Department  string
}

type Metadata struct {
	TMDBID        int
	Title         string
	OriginalTitle string
	Description   string
	Year          int
	Genres        []string
	Rating        float32
	Runtime       int
	PosterURL     string
	BackdropURL   string
	TrailerURL    string
	Cast          []CastCredit
	Crew          []CrewCredit
}

func New(apiKey, imageBase string) *Client {
	return &Client{
		apiKey:    apiKey,
		imageBase: imageBase,
		http:      &http.Client{},
	}
}

func (c *Client) FetchMetadata(title string, year int) (*Metadata, error) {
	id, err := c.searchMovie(title, year)
	if err != nil {
		return nil, err
	}
	return c.getMovieDetails(id)
}

// FetchByTMDBID skips search and fetches details for a known TMDB movie ID.
func (c *Client) FetchByTMDBID(id int) (*Metadata, error) {
	return c.getMovieDetails(id)
}

// FetchByIMDBID resolves an IMDb ID (ttNNNNNNN) to a TMDB movie ID via the
// /find endpoint, then fetches details. Returns ErrNotFound if TMDB has no
// matching movie record.
func (c *Client) FetchByIMDBID(imdbID string) (*Metadata, error) {
	id, err := c.findByExternalID(imdbID, "imdb_id")
	if err != nil {
		return nil, err
	}
	return c.getMovieDetails(id)
}

func (c *Client) findByExternalID(externalID, source string) (int, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("external_source", source)

	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/find/%s?%s", url.PathEscape(externalID), params.Encode()))
	if err != nil {
		return 0, fmt.Errorf("tmdb find: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("tmdb find: status %d", resp.StatusCode)
	}
	var result struct {
		MovieResults []struct {
			ID int `json:"id"`
		} `json:"movie_results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("tmdb find decode: %w", err)
	}
	if len(result.MovieResults) == 0 {
		return 0, fmt.Errorf("%w: %s", ErrNotFound, externalID)
	}
	return result.MovieResults[0].ID, nil
}

// searchMovie picks the best TMDB match for title/year. It tries a
// year-bounded query first (TMDB's year filter is exact-match) and falls
// back to an unbounded query so off-by-one release dates still resolve.
// The candidate list is then scored by title similarity × year proximity;
// if nothing clears matchThreshold we return ErrNotFound rather than
// silently bind to a wrong record.
func (c *Client) searchMovie(title string, year int) (int, error) {
	candidates, err := c.searchCandidates(title, year)
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 && year > 0 {
		candidates, err = c.searchCandidates(title, 0)
		if err != nil {
			return 0, err
		}
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("%w: %q", ErrNotFound, title)
	}
	best, score := bestMatch(title, year, candidates)
	if score < matchThreshold {
		return 0, fmt.Errorf("%w: %q (best %q (%d), score %.2f)", ErrNotFound, title, best.Title, best.Year, score)
	}
	return best.ID, nil
}

func (c *Client) searchCandidates(title string, year int) ([]searchCandidate, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", title)
	if year > 0 {
		params.Set("year", fmt.Sprintf("%d", year))
	}

	resp, err := c.http.Get("https://api.themoviedb.org/3/search/movie?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("tmdb search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb search: status %d", resp.StatusCode)
	}

	var raw struct {
		Results []struct {
			ID            int     `json:"id"`
			Title         string  `json:"title"`
			OriginalTitle string  `json:"original_title"`
			ReleaseDate   string  `json:"release_date"`
			Popularity    float64 `json:"popularity"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tmdb search decode: %w", err)
	}

	out := make([]searchCandidate, 0, len(raw.Results))
	for _, r := range raw.Results {
		out = append(out, searchCandidate{
			ID:            r.ID,
			Title:         r.Title,
			OriginalTitle: r.OriginalTitle,
			Year:          yearFromReleaseDate(r.ReleaseDate),
			Popularity:    r.Popularity,
		})
	}
	return out, nil
}

func yearFromReleaseDate(s string) int {
	if len(s) < 4 {
		return 0
	}
	var y int
	if _, err := fmt.Sscanf(s[:4], "%d", &y); err != nil {
		return 0
	}
	return y
}

func (c *Client) getMovieDetails(id int) (*Metadata, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/movie/%d?%s", id, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("tmdb details: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb details: status %d", resp.StatusCode)
	}

	var d struct {
		Title         string  `json:"title"`
		OriginalTitle string  `json:"original_title"`
		Overview      string  `json:"overview"`
		ReleaseDate   string  `json:"release_date"`
		PosterPath    string  `json:"poster_path"`
		BackdropPath  string  `json:"backdrop_path"`
		VoteAverage   float32 `json:"vote_average"`
		Runtime       int     `json:"runtime"`
		Genres        []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, fmt.Errorf("tmdb details decode: %w", err)
	}

	year := 0
	if len(d.ReleaseDate) >= 4 {
		fmt.Sscanf(d.ReleaseDate[:4], "%d", &year)
	}

	genres := make([]string, len(d.Genres))
	for i, g := range d.Genres {
		genres[i] = g.Name
	}

	m := &Metadata{
		TMDBID:        id,
		Title:         d.Title,
		OriginalTitle: d.OriginalTitle,
		Description:   d.Overview,
		Year:          year,
		Genres:        genres,
		Rating:        d.VoteAverage,
		Runtime:       d.Runtime,
	}
	if d.PosterPath != "" {
		m.PosterURL = c.imageBase + d.PosterPath
	}
	if d.BackdropPath != "" {
		m.BackdropURL = c.imageBase + d.BackdropPath
	}

	if key := c.fetchTrailerKey(id); key != "" {
		m.TrailerURL = "https://www.youtube.com/embed/" + key
	}

	if err := c.fetchCredits(id, m); err != nil {
		// Non-fatal: credits are supplemental.
		fmt.Printf("WARN tmdb credits for movie %d: %v\n", id, err)
	}

	return m, nil
}

func (c *Client) fetchTrailerKey(id int) string {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/videos?%s", id, params.Encode()))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var result struct {
		Results []struct {
			Key      string `json:"key"`
			Site     string `json:"site"`
			Type     string `json:"type"`
			Official bool   `json:"official"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return ""
	}
	// Prefer official YouTube trailers, fall back to any YouTube trailer.
	for _, pass := range []bool{true, false} {
		for _, v := range result.Results {
			if v.Site == "YouTube" && v.Type == "Trailer" && v.Official == pass {
				return v.Key
			}
		}
	}
	return ""
}

func (c *Client) fetchCredits(id int, m *Metadata) error {
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/movie/%d/credits?%s", id, params.Encode()))
	if err != nil {
		return fmt.Errorf("tmdb credits: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("tmdb credits: status %d", resp.StatusCode)
	}

	var cr struct {
		Cast []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Character   string `json:"character"`
			Order       int    `json:"order"`
			ProfilePath string `json:"profile_path"`
		} `json:"cast"`
		Crew []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Job         string `json:"job"`
			Department  string `json:"department"`
			ProfilePath string `json:"profile_path"`
		} `json:"crew"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return fmt.Errorf("tmdb credits decode: %w", err)
	}

	profileURL := func(path string) string {
		if path == "" {
			return ""
		}
		return c.imageBase + path
	}

	const maxCast = 10
	for i, p := range cr.Cast {
		if i >= maxCast {
			break
		}
		m.Cast = append(m.Cast, CastCredit{
			TmdbID: p.ID, Name: p.Name, ProfilePath: profileURL(p.ProfilePath),
			Biography: c.fetchPersonBio(p.ID),
			Character: p.Character, Order: p.Order,
		})
	}
	for _, p := range cr.Crew {
		m.Crew = append(m.Crew, CrewCredit{
			TmdbID: p.ID, Name: p.Name, ProfilePath: profileURL(p.ProfilePath),
			Biography: c.fetchPersonBio(p.ID),
			Job: p.Job, Department: p.Department,
		})
	}
	return nil
}

func (c *Client) fetchPersonBio(id int) string {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/person/%d?%s", id, params.Encode()))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var d struct {
		Biography string `json:"biography"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return ""
	}
	return d.Biography
}
