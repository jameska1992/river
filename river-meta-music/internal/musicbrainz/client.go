package musicbrainz

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var ErrNotFound = errors.New("musicbrainz: not found")

// MusicBrainz requires 1 req/s for unauthenticated clients.
const minInterval = 1100 * time.Millisecond

const userAgent = "river-meta-music/1.0 ( https://github.com/user/river )"

type Client struct {
	http    *http.Client
	lastReq time.Time
	mu      sync.Mutex
}

type ArtistMeta struct {
	MBID string
	Bio  string
}

type AlbumMeta struct {
	ReleaseGroupMBID string
	Title            string
	Year             int
	Genre            string
	CoverURL         string
}

func New() *Client {
	return &Client{http: &http.Client{Timeout: 15 * time.Second}}
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if elapsed := time.Since(c.lastReq); elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	c.lastReq = time.Now()
}

func (c *Client) get(rawURL string, out interface{}) error {
	c.throttle()
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return ErrNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// SearchArtist finds the best matching artist by name and returns metadata including bio.
func (c *Client) SearchArtist(name string) (*ArtistMeta, error) {
	params := url.Values{}
	params.Set("query", fmt.Sprintf("artist:%s", name))
	params.Set("limit", "1")
	params.Set("fmt", "json")

	var result struct {
		Artists []struct {
			ID string `json:"id"`
		} `json:"artists"`
	}
	if err := c.get("https://musicbrainz.org/ws/2/artist?"+params.Encode(), &result); err != nil {
		return nil, fmt.Errorf("search artist %q: %w", name, err)
	}
	if len(result.Artists) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, name)
	}

	meta := &ArtistMeta{MBID: result.Artists[0].ID}
	meta.Bio = c.fetchArtistBio(meta.MBID)
	return meta, nil
}

// fetchArtistBio resolves a Wikipedia bio via MusicBrainz URL relations.
func (c *Client) fetchArtistBio(mbid string) string {
	var result struct {
		Relations []struct {
			Type string `json:"type"`
			URL  struct {
				Resource string `json:"resource"`
			} `json:"url"`
		} `json:"relations"`
	}

	u := fmt.Sprintf("https://musicbrainz.org/ws/2/artist/%s?inc=url-rels&fmt=json", mbid)
	if err := c.get(u, &result); err != nil {
		return ""
	}

	var wikiTitle string
	for _, rel := range result.Relations {
		if rel.Type == "wikipedia" {
			parts := strings.Split(rel.URL.Resource, "/wiki/")
			if len(parts) == 2 {
				wikiTitle = parts[1]
			}
			break
		}
	}
	if wikiTitle == "" {
		return ""
	}

	params := url.Values{}
	params.Set("action", "query")
	params.Set("prop", "extracts")
	params.Set("exintro", "true")
	params.Set("explaintext", "true")
	params.Set("titles", wikiTitle)
	params.Set("format", "json")

	c.throttle()
	req, _ := http.NewRequest("GET", "https://en.wikipedia.org/w/api.php?"+params.Encode(), nil)
	req.Header.Set("User-Agent", userAgent)
	resp, err := c.http.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	var wikiResult struct {
		Query struct {
			Pages map[string]struct {
				Extract string `json:"extract"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&wikiResult); err != nil {
		return ""
	}
	for _, page := range wikiResult.Query.Pages {
		return strings.TrimSpace(page.Extract)
	}
	return ""
}

// FetchAlbums returns release-group metadata for all albums by the given artist MBID.
func (c *Client) FetchAlbums(artistMBID string) ([]AlbumMeta, error) {
	params := url.Values{}
	params.Set("artist", artistMBID)
	params.Set("type", "album")
	params.Set("inc", "tags")
	params.Set("limit", "100")
	params.Set("fmt", "json")

	var result struct {
		ReleaseGroups []struct {
			ID               string `json:"id"`
			Title            string `json:"title"`
			FirstReleaseDate string `json:"first-release-date"`
			Tags             []struct {
				Name  string `json:"name"`
				Count int    `json:"count"`
			} `json:"tags"`
		} `json:"release-groups"`
	}

	if err := c.get("https://musicbrainz.org/ws/2/release-group?"+params.Encode(), &result); err != nil {
		return nil, fmt.Errorf("fetch albums for artist %s: %w", artistMBID, err)
	}

	albums := make([]AlbumMeta, 0, len(result.ReleaseGroups))
	for _, rg := range result.ReleaseGroups {
		meta := AlbumMeta{
			ReleaseGroupMBID: rg.ID,
			Title:            rg.Title,
			Year:             parseYear(rg.FirstReleaseDate),
		}
		if len(rg.Tags) > 0 {
			meta.Genre = rg.Tags[0].Name
		}
		meta.CoverURL = c.fetchCoverURL(rg.ID)
		albums = append(albums, meta)
	}
	return albums, nil
}

// fetchCoverURL retrieves the front cover image URL from the Cover Art Archive.
func (c *Client) fetchCoverURL(releaseGroupMBID string) string {
	u := fmt.Sprintf("https://coverartarchive.org/release-group/%s", releaseGroupMBID)
	var result struct {
		Images []struct {
			Image string `json:"image"`
			Front bool   `json:"front"`
		} `json:"images"`
	}
	if err := c.get(u, &result); err != nil {
		return ""
	}
	for _, img := range result.Images {
		if img.Front {
			return img.Image
		}
	}
	if len(result.Images) > 0 {
		return result.Images[0].Image
	}
	return ""
}

// parseYear extracts the year from a MusicBrainz date string (YYYY, YYYY-MM, or YYYY-MM-DD).
func parseYear(date string) int {
	if len(date) < 4 {
		return 0
	}
	year := 0
	for _, ch := range date[:4] {
		if ch < '0' || ch > '9' {
			return 0
		}
		year = year*10 + int(ch-'0')
	}
	return year
}
