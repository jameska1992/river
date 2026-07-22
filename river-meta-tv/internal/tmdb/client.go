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

type ShowMetadata struct {
	TMDBShowID    int
	Title         string
	OriginalTitle string
	Description   string
	Year          int
	Status        string
	Genres        []string
	Rating        float32
	PosterURL     string
	BackdropURL   string
	TrailerURL    string
	Cast          []CastCredit
	Crew          []CrewCredit
}

type SeasonMetadata struct {
	Number      int
	Title       string
	Description string
	Year        int
	PosterURL   string
	Episodes    []EpisodeMetadata
}

type EpisodeMetadata struct {
	Number      int
	Title       string
	Description string
	Runtime     int
	AiredAt     string // YYYY-MM-DD
}

func New(apiKey, imageBase string) *Client {
	return &Client{
		apiKey:    apiKey,
		imageBase: imageBase,
		http:      &http.Client{},
	}
}

// FetchShowMetadata resolves a show by name, optionally biased toward a
// release year. Pass year=0 when not known. Year is critical for shows
// with multiple eras (Doctor Who 1963 vs 2005, Battlestar Galactica 1978
// vs 2004) — without it the admin's reidentified year gets overwritten
// by whichever entry TMDB ranks highest by popularity.
func (c *Client) FetchShowMetadata(name string, year int) (*ShowMetadata, error) {
	id, err := c.searchShow(name, year)
	if err != nil {
		return nil, err
	}
	return c.getShowDetails(id)
}

// FetchShowByTMDBID skips search and fetches details for a known TMDB show id.
func (c *Client) FetchShowByTMDBID(id int) (*ShowMetadata, error) {
	return c.getShowDetails(id)
}

// FetchShowByIMDBID resolves an IMDb id (tt...) to a TMDB show id via
// /find?external_source=imdb_id. Returns ErrNotFound if TMDB has no matching
// show record.
func (c *Client) FetchShowByIMDBID(imdbID string) (*ShowMetadata, error) {
	id, err := c.findShowByExternalID(imdbID, "imdb_id")
	if err != nil {
		return nil, err
	}
	return c.getShowDetails(id)
}

func (c *Client) findShowByExternalID(externalID, source string) (int, error) {
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
		TVResults []struct {
			ID int `json:"id"`
		} `json:"tv_results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("tmdb find decode: %w", err)
	}
	if len(result.TVResults) == 0 {
		return 0, fmt.Errorf("%w: %s", ErrNotFound, externalID)
	}
	return result.TVResults[0].ID, nil
}

func (c *Client) FetchSeasonMetadata(tmdbShowID, seasonNumber int) (*SeasonMetadata, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	resp, err := c.http.Get(fmt.Sprintf(
		"https://api.themoviedb.org/3/tv/%d/season/%d?%s",
		tmdbShowID, seasonNumber, params.Encode(),
	))
	if err != nil {
		return nil, fmt.Errorf("tmdb season: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb season: status %d", resp.StatusCode)
	}

	var d struct {
		Name         string `json:"name"`
		Overview     string `json:"overview"`
		AirDate      string `json:"air_date"`
		SeasonNumber int    `json:"season_number"`
		PosterPath   string `json:"poster_path"`
		Episodes     []struct {
			EpisodeNumber int    `json:"episode_number"`
			Name          string `json:"name"`
			Overview      string `json:"overview"`
			Runtime       int    `json:"runtime"`
			AirDate       string `json:"air_date"`
		} `json:"episodes"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, fmt.Errorf("tmdb season decode: %w", err)
	}

	meta := &SeasonMetadata{
		Number:      d.SeasonNumber,
		Title:       d.Name,
		Description: d.Overview,
		Year:        parseYear(d.AirDate),
	}
	if d.PosterPath != "" {
		meta.PosterURL = c.imageBase + d.PosterPath
	}
	for _, e := range d.Episodes {
		meta.Episodes = append(meta.Episodes, EpisodeMetadata{
			Number:      e.EpisodeNumber,
			Title:       e.Name,
			Description: e.Overview,
			Runtime:     e.Runtime,
			AiredAt:     e.AirDate,
		})
	}
	return meta, nil
}

// searchShow picks the best TMDB match for a show by title and optional
// year. Same scored selection as the movie path: title similarity × year
// proximity, with a minimum threshold so wildly-different candidates fail
// loudly rather than bind to the wrong show. When year is 0, title
// similarity dominates; when set (e.g. parsed from the folder name or
// supplied by the admin Identify flow), it disambiguates same-title
// reboots like Doctor Who 1963 vs 2005.
func (c *Client) searchShow(name string, year int) (int, error) {
	candidates, err := c.searchShowCandidates(name)
	if err != nil {
		return 0, err
	}
	if len(candidates) == 0 {
		return 0, fmt.Errorf("%w: %q", ErrNotFound, name)
	}
	best, score := bestMatch(name, year, candidates)
	if score < matchThreshold {
		return 0, fmt.Errorf("%w: %q (best %q, score %.2f)", ErrNotFound, name, best.Title, score)
	}
	return best.ID, nil
}

func (c *Client) searchShowCandidates(name string) ([]searchCandidate, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	params.Set("query", name)

	resp, err := c.http.Get("https://api.themoviedb.org/3/search/tv?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("tmdb search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb search: status %d", resp.StatusCode)
	}

	var raw struct {
		Results []struct {
			ID           int     `json:"id"`
			Name         string  `json:"name"`
			OriginalName string  `json:"original_name"`
			FirstAirDate string  `json:"first_air_date"`
			Popularity   float64 `json:"popularity"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tmdb search decode: %w", err)
	}

	out := make([]searchCandidate, 0, len(raw.Results))
	for _, r := range raw.Results {
		out = append(out, searchCandidate{
			ID:            r.ID,
			Title:         r.Name,
			OriginalTitle: r.OriginalName,
			Year:          parseYear(r.FirstAirDate),
			Popularity:    r.Popularity,
		})
	}
	return out, nil
}

func (c *Client) getShowDetails(id int) (*ShowMetadata, error) {
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/tv/%d?%s", id, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("tmdb details: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb details: status %d", resp.StatusCode)
	}

	var d struct {
		ID           int     `json:"id"`
		Name         string  `json:"name"`
		OriginalName string  `json:"original_name"`
		Overview     string  `json:"overview"`
		FirstAirDate string  `json:"first_air_date"`
		Status       string  `json:"status"`
		VoteAverage  float32 `json:"vote_average"`
		PosterPath   string  `json:"poster_path"`
		BackdropPath string  `json:"backdrop_path"`
		Genres       []struct {
			Name string `json:"name"`
		} `json:"genres"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return nil, fmt.Errorf("tmdb details decode: %w", err)
	}

	genres := make([]string, len(d.Genres))
	for i, g := range d.Genres {
		genres[i] = g.Name
	}

	meta := &ShowMetadata{
		TMDBShowID:    d.ID,
		Title:         d.Name,
		OriginalTitle: d.OriginalName,
		Description:   d.Overview,
		Year:          parseYear(d.FirstAirDate),
		Status:        d.Status,
		Genres:        genres,
		Rating:        d.VoteAverage,
	}
	if d.PosterPath != "" {
		meta.PosterURL = c.imageBase + d.PosterPath
	}
	if d.BackdropPath != "" {
		meta.BackdropURL = c.imageBase + d.BackdropPath
	}

	if key := c.fetchTrailerKey(d.ID); key != "" {
		meta.TrailerURL = "https://www.youtube.com/embed/" + key
	}

	if err := c.fetchShowCredits(d.ID, meta); err != nil {
		fmt.Printf("WARN tmdb credits for show %d: %v\n", d.ID, err)
	}
	return meta, nil
}

func (c *Client) fetchTrailerKey(id int) string {
	params := url.Values{}
	params.Set("api_key", c.apiKey)
	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/videos?%s", id, params.Encode()))
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
	for _, pass := range []bool{true, false} {
		for _, v := range result.Results {
			if v.Site == "YouTube" && v.Type == "Trailer" && v.Official == pass {
				return v.Key
			}
		}
	}
	return ""
}

func (c *Client) fetchShowCredits(id int, m *ShowMetadata) error {
	params := url.Values{}
	params.Set("api_key", c.apiKey)

	resp, err := c.http.Get(fmt.Sprintf("https://api.themoviedb.org/3/tv/%d/credits?%s", id, params.Encode()))
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

func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	var y int
	fmt.Sscanf(date[:4], "%d", &y)
	return y
}
