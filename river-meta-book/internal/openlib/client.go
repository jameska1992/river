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
}

type Metadata struct {
	Title       string
	Author      string
	Description string
	Year        int
	Genre       string
	CoverURL    string
}

func New() *Client {
	return &Client{http: &http.Client{}}
}

func (c *Client) FetchMetadata(title string) (*Metadata, error) {
	params := url.Values{}
	params.Set("title", title)
	params.Set("fields", "key,title,author_name,first_publish_year,subject,cover_i")
	params.Set("limit", "1")

	resp, err := c.http.Get("https://openlibrary.org/search.json?" + params.Encode())
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
		Title: doc.Title,
		Year:  doc.FirstPublishYear,
	}
	if len(doc.AuthorName) > 0 {
		meta.Author = doc.AuthorName[0]
	}
	if len(doc.Subject) > 0 {
		meta.Genre = doc.Subject[0]
	}
	if doc.CoverI > 0 {
		meta.CoverURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-L.jpg", doc.CoverI)
	}
	if doc.Key != "" {
		meta.Description = c.fetchDescription(doc.Key)
	}

	return meta, nil
}

// fetchDescription fetches the description from the Open Library works endpoint.
// The description field can be either a plain string or an object with a "value" key.
func (c *Client) fetchDescription(workKey string) string {
	resp, err := c.http.Get("https://openlibrary.org" + workKey + ".json")
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
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil || d.Description == nil {
		return ""
	}

	var s string
	if err := json.Unmarshal(d.Description, &s); err == nil {
		return strings.TrimSpace(s)
	}

	var obj struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(d.Description, &obj); err == nil {
		return strings.TrimSpace(obj.Value)
	}

	return ""
}
