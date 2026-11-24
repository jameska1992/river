package tmdb

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// PersonResult is one hit from a TMDB person search, trimmed to what the
// credits editor's picker needs.
type PersonResult struct {
	TmdbID             int      `json:"tmdb_id"`
	Name               string   `json:"name"`
	ProfilePath        string   `json:"profile_path"`
	KnownForDepartment string   `json:"known_for_department"`
	KnownFor           []string `json:"known_for"`
}

// PersonDetail is a single TMDB person, used to fill in a credit picked from
// search (search results carry no biography).
type PersonDetail struct {
	TmdbID             int    `json:"tmdb_id"`
	Name               string `json:"name"`
	ProfilePath        string `json:"profile_path"`
	Biography          string `json:"biography"`
	KnownForDepartment string `json:"known_for_department"`
	Birthday           string `json:"birthday"`
	PlaceOfBirth       string `json:"place_of_birth"`
	IMDBID             string `json:"imdb_id"`
}

// maxKnownFor caps the "known for" titles carried per search hit.
const maxKnownFor = 3

func (c *Client) profileURL(path string) string {
	if path == "" {
		return ""
	}
	return c.imageBase + path
}

// SearchPeople queries TMDB's person search. Only the first page (20 hits) is
// returned — enough for a type-ahead picker.
func (c *Client) SearchPeople(query string) ([]PersonResult, error) {
	params := url.Values{}
	params.Set("api_key", c.keyFn())
	params.Set("query", query)
	params.Set("include_adult", "false")

	resp, err := c.http.Get(c.apiBase + "/search/person?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("tmdb person search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb person search: status %d", resp.StatusCode)
	}

	var raw struct {
		Results []struct {
			ID                 int    `json:"id"`
			Name               string `json:"name"`
			ProfilePath        string `json:"profile_path"`
			KnownForDepartment string `json:"known_for_department"`
			KnownFor           []struct {
				Title string `json:"title"` // movies
				Name  string `json:"name"`  // tv
			} `json:"known_for"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tmdb person search decode: %w", err)
	}

	out := make([]PersonResult, 0, len(raw.Results))
	for _, r := range raw.Results {
		known := []string{}
		for _, k := range r.KnownFor {
			if len(known) >= maxKnownFor {
				break
			}
			if t := k.Title; t != "" {
				known = append(known, t)
			} else if k.Name != "" {
				known = append(known, k.Name)
			}
		}
		out = append(out, PersonResult{
			TmdbID:             r.ID,
			Name:               r.Name,
			ProfilePath:        c.profileURL(r.ProfilePath),
			KnownForDepartment: r.KnownForDepartment,
			KnownFor:           known,
		})
	}
	return out, nil
}

// GetPerson fetches a single TMDB person. Returns ErrNotFound on a 404.
func (c *Client) GetPerson(id int) (*PersonDetail, error) {
	params := url.Values{}
	params.Set("api_key", c.keyFn())

	resp, err := c.http.Get(fmt.Sprintf("%s/person/%d?%s", c.apiBase, id, params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("tmdb person: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: person %d", ErrNotFound, id)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("tmdb person: status %d", resp.StatusCode)
	}

	var raw struct {
		ID                 int    `json:"id"`
		Name               string `json:"name"`
		ProfilePath        string `json:"profile_path"`
		Biography          string `json:"biography"`
		KnownForDepartment string `json:"known_for_department"`
		Birthday           string `json:"birthday"`
		PlaceOfBirth       string `json:"place_of_birth"`
		IMDBID             string `json:"imdb_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("tmdb person decode: %w", err)
	}
	return &PersonDetail{
		TmdbID:             raw.ID,
		Name:               raw.Name,
		ProfilePath:        c.profileURL(raw.ProfilePath),
		Biography:          raw.Biography,
		KnownForDepartment: raw.KnownForDepartment,
		Birthday:           raw.Birthday,
		PlaceOfBirth:       raw.PlaceOfBirth,
		IMDBID:             raw.IMDBID,
	}, nil
}

// PeopleSearchCache memoises SearchPeople results for a short TTL so a
// type-ahead picker doesn't send TMDB a request for every keystroke it
// revisits (backspacing, re-opening the editor). Bounded: when full it drops
// expired entries, then everything, rather than tracking LRU order.
type PeopleSearchCache struct {
	search  func(string) ([]PersonResult, error)
	ttl     time.Duration
	maxSize int
	now     func() time.Time

	mu      sync.Mutex
	entries map[string]peopleCacheEntry
}

type peopleCacheEntry struct {
	results []PersonResult
	expires time.Time
}

func NewPeopleSearchCache(search func(string) ([]PersonResult, error), ttl time.Duration, maxSize int) *PeopleSearchCache {
	return &PeopleSearchCache{
		search:  search,
		ttl:     ttl,
		maxSize: maxSize,
		now:     time.Now,
		entries: map[string]peopleCacheEntry{},
	}
}

// Search returns cached results for query (case/space-insensitive), or calls
// through and caches a successful response. Errors are never cached.
func (p *PeopleSearchCache) Search(query string) ([]PersonResult, error) {
	key := strings.ToLower(strings.Join(strings.Fields(query), " "))

	p.mu.Lock()
	if e, ok := p.entries[key]; ok && p.now().Before(e.expires) {
		p.mu.Unlock()
		return e.results, nil
	}
	p.mu.Unlock()

	results, err := p.search(query)
	if err != nil {
		return nil, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.entries) >= p.maxSize {
		now := p.now()
		for k, e := range p.entries {
			if !now.Before(e.expires) {
				delete(p.entries, k)
			}
		}
		if len(p.entries) >= p.maxSize {
			clear(p.entries)
		}
	}
	p.entries[key] = peopleCacheEntry{results: results, expires: p.now().Add(p.ttl)}
	return results, nil
}
