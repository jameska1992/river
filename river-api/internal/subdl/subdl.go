// Package subdl is a thin client for the SubDL subtitle API
// (https://subdl.com). It covers the two operations River needs: searching for
// subtitles by TMDB id, and downloading a chosen result (SubDL serves each as a
// .zip containing an .srt, which we normalise to WebVTT — the format the
// players already parse).
package subdl

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIBase = "https://api.subdl.com/api/v1/subtitles"
	// defaultDownloadHost is the fixed host SubDL serves subtitle archives
	// from. The API returns a path; we only ever append it to this host, so a
	// malicious/garbled url can't redirect the fetch elsewhere (SSRF guard).
	defaultDownloadHost = "https://dl.subdl.com"

	maxSubsPerPage   = 30
	maxDownloadBytes = 8 << 20 // 8 MiB cap on the downloaded archive
	maxSubtitleBytes = 8 << 20 // 8 MiB cap on a single extracted subtitle
)

// Client talks to the SubDL HTTP API. Safe for concurrent use. The endpoint
// hosts are fields (defaulted in NewClient) so tests can point them at a stub.
type Client struct {
	http    *http.Client
	apiBase string
	dlHost  string
}

func NewClient() *Client {
	return &Client{
		http:    &http.Client{Timeout: 20 * time.Second},
		apiBase: defaultAPIBase,
		dlHost:  defaultDownloadHost,
	}
}

// Subtitle is one search result. Field names mirror the SubDL response.
type Subtitle struct {
	ReleaseName string `json:"release_name"`
	Name        string `json:"name"`
	Lang        string `json:"lang"`     // language code, e.g. "EN"
	Language    string `json:"language"` // display name, e.g. "English"
	Author      string `json:"author"`
	URL         string `json:"url"` // path under downloadHost; points to a .zip
	Season      int    `json:"season"`
	Episode     int    `json:"episode"`
	HI          bool   `json:"hi"` // hearing-impaired
}

// SearchParams are the query inputs. Zero-valued optionals are omitted.
type SearchParams struct {
	APIKey        string
	TmdbID        int
	Type          string // "movie" | "tv"
	Languages     string // comma-separated codes, e.g. "EN,FR" (empty = all)
	SeasonNumber  int
	EpisodeNumber int
	SubsPerPage   int
}

type apiResponse struct {
	Status    bool       `json:"status"`
	Error     string     `json:"error"`
	Subtitles []Subtitle `json:"subtitles"`
}

// Search queries SubDL and returns the matching subtitles. A well-formed
// "no results" response comes back as an empty slice, not an error; only a
// transport failure, a non-200 status, or an API-reported error (e.g. a bad
// key) is returned as an error.
func (c *Client) Search(ctx context.Context, p SearchParams) ([]Subtitle, error) {
	q := url.Values{}
	q.Set("api_key", p.APIKey)
	if p.TmdbID > 0 {
		q.Set("tmdb_id", strconv.Itoa(p.TmdbID))
	}
	if p.Type != "" {
		q.Set("type", p.Type)
	}
	if p.Languages != "" {
		q.Set("languages", p.Languages)
	}
	if p.SeasonNumber > 0 {
		q.Set("season_number", strconv.Itoa(p.SeasonNumber))
	}
	if p.EpisodeNumber > 0 {
		q.Set("episode_number", strconv.Itoa(p.EpisodeNumber))
	}
	perPage := p.SubsPerPage
	if perPage <= 0 || perPage > maxSubsPerPage {
		perPage = maxSubsPerPage
	}
	q.Set("subs_per_page", strconv.Itoa(perPage))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.apiBase+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("subdl search: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subdl search: status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var out apiResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("subdl search: decode response: %w", err)
	}
	if !out.Status {
		// SubDL reports "no results" via status=false. Treat an empty result
		// set as success; surface anything else (e.g. an invalid api key).
		if len(out.Subtitles) == 0 && isNoResults(out.Error) {
			return []Subtitle{}, nil
		}
		if out.Error != "" {
			return nil, fmt.Errorf("subdl search: %s", out.Error)
		}
	}
	return out.Subtitles, nil
}

// isNoResults reports whether an API error string is the benign
// "nothing matched" case rather than a real failure.
func isNoResults(msg string) bool {
	m := strings.ToLower(msg)
	return m == "" || strings.Contains(m, "no results") || strings.Contains(m, "not found")
}

// Download fetches the chosen subtitle archive and returns the first .srt/.vtt
// it contains, normalised to WebVTT. subURL is the path from a search result's
// URL field; it must be a rooted path (it's appended to the fixed download
// host).
func (c *Client) Download(ctx context.Context, subURL string) ([]byte, error) {
	if !strings.HasPrefix(subURL, "/") {
		return nil, fmt.Errorf("subdl download: unexpected url %q", subURL)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.dlHost+subURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("subdl download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subdl download: status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		return nil, fmt.Errorf("subdl download: read: %w", err)
	}
	if len(data) > maxDownloadBytes {
		return nil, fmt.Errorf("subdl download: archive exceeds %d bytes", maxDownloadBytes)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("subdl download: not a zip archive: %w", err)
	}
	for _, f := range zr.File {
		name := strings.ToLower(f.Name)
		if !strings.HasSuffix(name, ".srt") && !strings.HasSuffix(name, ".vtt") {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("subdl download: open %q: %w", f.Name, err)
		}
		content, err := io.ReadAll(io.LimitReader(rc, maxSubtitleBytes))
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("subdl download: read %q: %w", f.Name, err)
		}
		return ToVTT(content, f.Name), nil
	}
	return nil, fmt.Errorf("subdl download: archive has no .srt/.vtt file")
}

// ToVTT normalises a subtitle payload to WebVTT. Content already in VTT is
// returned unchanged; SRT is converted by prepending the WEBVTT header and
// switching the millisecond separator in timing lines from comma to dot. A
// leading UTF-8 BOM is stripped either way.
func ToVTT(content []byte, filename string) []byte {
	s := strings.TrimPrefix(string(content), "\ufeff")
	s = strings.ReplaceAll(s, "\r\n", "\n")

	if strings.HasSuffix(strings.ToLower(filename), ".vtt") || strings.HasPrefix(strings.TrimSpace(s), "WEBVTT") {
		if !strings.HasPrefix(strings.TrimSpace(s), "WEBVTT") {
			s = "WEBVTT\n\n" + s
		}
		return []byte(s)
	}

	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "-->") {
			lines[i] = strings.ReplaceAll(ln, ",", ".")
		}
	}
	return []byte("WEBVTT\n\n" + strings.Join(lines, "\n"))
}
