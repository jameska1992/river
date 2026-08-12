package openlib

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

var ErrNotFound = errors.New("openlib: not found")

type Client struct {
	http *http.Client
	// baseURL is the Open Library origin; overridable in tests. Cover images
	// live on a separate host (covers.openlibrary.org) and are only formatted
	// into a URL, never fetched here, so they're not affected by this.
	baseURL string
}

type Metadata struct {
	Title       string
	Author      string
	Description string
	Year        int
	Genre       string
	CoverURL    string
	// WorkKey is the Open Library work key (e.g. "/works/OL45804W") — the
	// stable identifier stored on the record for sticky re-enrichment.
	WorkKey string
	// ISBNs are the ISBNs Open Library lists for the work (across editions);
	// often empty for audiobooks.
	ISBNs []string
}

func New() *Client {
	return &Client{http: &http.Client{}, baseURL: "https://openlibrary.org"}
}

func (c *Client) FetchMetadata(title string) (*Metadata, error) {
	params := url.Values{}
	params.Set("title", title)
	params.Set("fields", "key,title,author_name,first_publish_year,subject,cover_i,isbn")
	params.Set("limit", "1")

	resp, err := c.http.Get(c.baseURL + "/search.json?" + params.Encode())
	if err != nil {
		return nil, fmt.Errorf("openlib search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openlib search: status %d", resp.StatusCode)
	}

	var result struct {
		Docs []struct {
			Key              string   `json:"key"`
			Title            string   `json:"title"`
			AuthorName       []string `json:"author_name"`
			FirstPublishYear int      `json:"first_publish_year"`
			Subject          []string `json:"subject"`
			CoverI           int      `json:"cover_i"`
			ISBN             []string `json:"isbn"`
		} `json:"docs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("openlib search decode: %w", err)
	}
	if len(result.Docs) == 0 {
		return nil, fmt.Errorf("%w: %q", ErrNotFound, title)
	}

	doc := result.Docs[0]
	meta := &Metadata{
		Title:   doc.Title,
		Year:    doc.FirstPublishYear,
		WorkKey: doc.Key,
		ISBNs:   doc.ISBN,
	}
	if len(doc.AuthorName) > 0 {
		meta.Author = doc.AuthorName[0]
	}
	if len(doc.Subject) > 0 {
		meta.Genre = doc.Subject[0]
	}
	if doc.CoverI > 0 {
		meta.CoverURL = coverURL(doc.CoverI)
	}
	if doc.Key != "" {
		meta.Description = c.fetchDescription(doc.Key)
	}

	return meta, nil
}

// FetchByWorkKey resolves metadata directly from a known Open Library work key
// (e.g. "/works/OL45804W") instead of a title search. This is how re-enrichment
// stays pinned to the originally-matched work rather than drifting to whatever
// a fresh title search returns first.
//
// The work endpoint carries title, description, subjects and cover ids; the
// author is a reference we resolve with a second call. It does NOT carry a
// publish year or ISBNs (those are edition-level), so those come back zero/nil
// — the caller coalesces them against the existing record so they're preserved,
// not blanked.
func (c *Client) FetchByWorkKey(workKey string) (*Metadata, error) {
	resp, err := c.http.Get(c.baseURL + workKey + ".json")
	if err != nil {
		return nil, fmt.Errorf("openlib work: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, workKey)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openlib work: status %d", resp.StatusCode)
	}

	var w struct {
		Title       string          `json:"title"`
		Description json.RawMessage `json:"description"`
		Subjects    []string        `json:"subjects"`
		Covers      []int           `json:"covers"`
		Authors     []struct {
			Author struct {
				Key string `json:"key"`
			} `json:"author"`
		} `json:"authors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&w); err != nil {
		return nil, fmt.Errorf("openlib work decode: %w", err)
	}

	meta := &Metadata{
		Title:       w.Title,
		Description: parseDescription(w.Description),
		WorkKey:     workKey,
	}
	if len(w.Subjects) > 0 {
		meta.Genre = w.Subjects[0]
	}
	if len(w.Covers) > 0 && w.Covers[0] > 0 {
		meta.CoverURL = coverURL(w.Covers[0])
	}
	if len(w.Authors) > 0 && w.Authors[0].Author.Key != "" {
		meta.Author = c.fetchAuthorName(w.Authors[0].Author.Key)
	}
	return meta, nil
}

func coverURL(id int) string {
	return fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", id)
}

// fetchAuthorName resolves an author reference key ("/authors/OL…A") to a
// display name. Best-effort: any failure yields "" and the caller preserves
// the existing author.
func (c *Client) fetchAuthorName(authorKey string) string {
	resp, err := c.http.Get(c.baseURL + authorKey + ".json")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}
	var a struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		return ""
	}
	return strings.TrimSpace(a.Name)
}

// fetchDescription fetches the description from the Open Library works endpoint.
// The description field can be either a plain string or an object with a "value" key.
func (c *Client) fetchDescription(workKey string) string {
	resp, err := c.http.Get(c.baseURL + workKey + ".json")
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return ""
	}

	var d struct {
		Description json.RawMessage `json:"description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return ""
	}
	return parseDescription(d.Description)
}

// parseDescription normalises Open Library's description, which is either a
// plain string or an object with a "value" key.
func parseDescription(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strings.TrimSpace(s)
	}
	var obj struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(raw, &obj); err == nil {
		return strings.TrimSpace(obj.Value)
	}
	return ""
}
