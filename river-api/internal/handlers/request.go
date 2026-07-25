package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

// RequestHandler proxies to Radarr/Sonarr. Configuration lives in the DB
// (via SettingsService) and is read per-request, so an admin can point
// River at a different *arr instance without a restart.
type RequestHandler struct {
	settings *services.SettingsService
	http     *http.Client
}

func NewRequestHandler(settings *services.SettingsService) *RequestHandler {
	return &RequestHandler{settings: settings, http: &http.Client{}}
}

type MovieSearchResult struct {
	TmdbID   int    `json:"tmdbId"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Overview string `json:"overview"`
	Poster   string `json:"poster"`
	Added    bool   `json:"added"`
}

type ShowSearchResult struct {
	TvdbID   int    `json:"tvdbId"`
	Title    string `json:"title"`
	Year     int    `json:"year"`
	Overview string `json:"overview"`
	Poster   string `json:"poster"`
	Added    bool   `json:"added"`
}

// arrImage is the shared Radarr/Sonarr image shape (poster, banner, fanart…).
type arrImage struct {
	CoverType string `json:"coverType"`
	RemoteURL string `json:"remoteUrl"`
	URL       string `json:"url"`
}

// CalendarItem is a unified upcoming-release entry, combining Radarr movie
// releases and Sonarr episode air dates into a single shape the calendar UI
// renders directly.
type CalendarItem struct {
	Type           string `json:"type"`                   // "movie" | "episode"
	Title          string `json:"title"`                  // movie title, or series title for episodes
	EpisodeTitle   string `json:"episodeTitle,omitempty"` // episode name (episodes only)
	SeasonNumber   int    `json:"seasonNumber,omitempty"`
	EpisodeNumber  int    `json:"episodeNumber,omitempty"`
	ReleaseType    string `json:"releaseType,omitempty"`    // "digital"|"physical"|"cinema" (movies only)
	Date           string `json:"date"`                     // RFC3339 date the event lands on (grid placement)
	DigitalRelease string `json:"digitalRelease,omitempty"` // RFC3339 digital release, when known (movies only)
	Overview       string `json:"overview"`
	Poster         string `json:"poster"`
	HasFile        bool   `json:"hasFile"`
}

// SearchMovies queries Radarr's movie lookup endpoint.
//
// @Summary      Search movies (Radarr)
// @Tags         request
// @Produce      json
// @Param        q  query  string  true  "Search term"
// @Success      200  {array}   handlers.MovieSearchResult
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "Radarr not configured"
// @Security     BearerAuth
// @Router       /request/movies [get]
func (h *RequestHandler) SearchMovies(c *gin.Context) {
	radarrURL, radarrKey, radarrEnabled := h.settings.RadarrConfig()
	if !radarrEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Radarr not configured"})
		return
	}
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	var raw []struct {
		ID           int    `json:"id"`
		TmdbID       int    `json:"tmdbId"`
		Title        string `json:"title"`
		Year         int    `json:"year"`
		Overview     string `json:"overview"`
		RemotePoster string `json:"remotePoster"`
	}
	endpoint := fmt.Sprintf("%s/api/v3/movie/lookup?term=%s", radarrURL, url.QueryEscape(q))
	if err := h.arrGet(endpoint, radarrKey, &raw); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	results := make([]MovieSearchResult, 0, len(raw))
	for _, r := range raw {
		results = append(results, MovieSearchResult{
			TmdbID:   r.TmdbID,
			Title:    r.Title,
			Year:     r.Year,
			Overview: r.Overview,
			Poster:   r.RemotePoster,
			Added:    r.ID > 0,
		})
	}
	c.JSON(http.StatusOK, results)
}

// SearchShows queries Sonarr's series lookup endpoint.
//
// @Summary      Search TV shows (Sonarr)
// @Tags         request
// @Produce      json
// @Param        q  query  string  true  "Search term"
// @Success      200  {array}   handlers.ShowSearchResult
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "Sonarr not configured"
// @Security     BearerAuth
// @Router       /request/shows [get]
func (h *RequestHandler) SearchShows(c *gin.Context) {
	sonarrURL, sonarrKey, sonarrEnabled := h.settings.SonarrConfig()
	if !sonarrEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Sonarr not configured"})
		return
	}
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}

	var raw []struct {
		ID           int    `json:"id"`
		TvdbID       int    `json:"tvdbId"`
		Title        string `json:"title"`
		Year         int    `json:"year"`
		Overview     string `json:"overview"`
		RemotePoster string `json:"remotePoster"`
	}
	endpoint := fmt.Sprintf("%s/api/v3/series/lookup?term=%s", sonarrURL, url.QueryEscape(q))
	if err := h.arrGet(endpoint, sonarrKey, &raw); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	results := make([]ShowSearchResult, 0, len(raw))
	for _, r := range raw {
		results = append(results, ShowSearchResult{
			TvdbID:   r.TvdbID,
			Title:    r.Title,
			Year:     r.Year,
			Overview: r.Overview,
			Poster:   r.RemotePoster,
			Added:    r.ID > 0,
		})
	}
	c.JSON(http.StatusOK, results)
}

type addMovieReq struct {
	TmdbID int    `json:"tmdbId" binding:"required"`
	Title  string `json:"title"  binding:"required"`
	Year   int    `json:"year"`
}

// AddMovie sends a Radarr "add" request for a previously-searched movie.
//
// @Summary      Add movie via Radarr
// @Tags         request
// @Accept       json
// @Produce      json
// @Param        body  body  addMovieReq  true  "{tmdbId, title, year}"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /request/movies [post]
func (h *RequestHandler) AddMovie(c *gin.Context) {
	radarrURL, radarrKey, radarrEnabled := h.settings.RadarrConfig()
	if !radarrEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Radarr not configured"})
		return
	}
	var req addMovieReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rootFolder, qualityID, err := h.fetchDefaults(radarrURL, radarrKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	payload := map[string]any{
		"title":            req.Title,
		"year":             req.Year,
		"tmdbId":           req.TmdbID,
		"qualityProfileId": qualityID,
		"rootFolderPath":   rootFolder,
		"monitored":        true,
		"addOptions":       map[string]any{"searchForMovie": true},
	}
	if err := h.arrPost(radarrURL+"/api/v3/movie", radarrKey, payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

type addShowReq struct {
	TvdbID int    `json:"tvdbId" binding:"required"`
	Title  string `json:"title"  binding:"required"`
	Year   int    `json:"year"`
}

// AddShow sends a Sonarr "add" request for a previously-searched show.
//
// @Summary      Add TV show via Sonarr
// @Tags         request
// @Accept       json
// @Produce      json
// @Param        body  body  addShowReq  true  "{tvdbId, title, year}"
// @Success      204
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /request/shows [post]
func (h *RequestHandler) AddShow(c *gin.Context) {
	sonarrURL, sonarrKey, sonarrEnabled := h.settings.SonarrConfig()
	if !sonarrEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Sonarr not configured"})
		return
	}
	var req addShowReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rootFolder, qualityID, err := h.fetchDefaults(sonarrURL, sonarrKey)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	// Sonarr v3 requires languageProfileId; v4 removed language profiles entirely.
	// Fetch and include only when profiles exist.
	var langProfiles []struct {
		ID int `json:"id"`
	}
	_ = h.arrGet(sonarrURL+"/api/v3/languageprofile", sonarrKey, &langProfiles)

	payload := map[string]any{
		"title":            req.Title,
		"year":             req.Year,
		"tvdbId":           req.TvdbID,
		"qualityProfileId": qualityID,
		"rootFolderPath":   rootFolder,
		"monitored":        true,
		"seasonFolder":     true,
		"addOptions": map[string]any{
			"searchForMissingEpisodes": true,
			"monitor":                  "all",
		},
	}
	if len(langProfiles) > 0 {
		payload["languageProfileId"] = langProfiles[0].ID
	}

	if err := h.arrPost(sonarrURL+"/api/v3/series", sonarrKey, payload); err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// Calendar returns a combined, date-sorted list of upcoming movie releases
// (Radarr) and episode air dates (Sonarr) within [start, end]. Sources that
// aren't configured are skipped; a configured source that errors is only fatal
// when it leaves no data at all — one flaky service shouldn't blank the whole
// calendar.
//
// @Summary      Combined Radarr + Sonarr calendar
// @Tags         request
// @Produce      json
// @Param        start  query  string  true  "Range start (YYYY-MM-DD)"
// @Param        end    query  string  true  "Range end (YYYY-MM-DD)"
// @Success      200  {array}   handlers.CalendarItem
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string  "Neither Radarr nor Sonarr configured"
// @Security     BearerAuth
// @Router       /request/calendar [get]
func (h *RequestHandler) Calendar(c *gin.Context) {
	radarrURL, radarrKey, radarrEnabled := h.settings.RadarrConfig()
	sonarrURL, sonarrKey, sonarrEnabled := h.settings.SonarrConfig()
	if !radarrEnabled && !sonarrEnabled {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Neither Radarr nor Sonarr is configured"})
		return
	}
	start := c.Query("start")
	end := c.Query("end")
	if start == "" || end == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start and end are required"})
		return
	}

	items := make([]CalendarItem, 0)
	var firstErr error

	if radarrEnabled {
		var raw []struct {
			Title           string     `json:"title"`
			Overview        string     `json:"overview"`
			InCinemas       string     `json:"inCinemas"`
			PhysicalRelease string     `json:"physicalRelease"`
			DigitalRelease  string     `json:"digitalRelease"`
			HasFile         bool       `json:"hasFile"`
			Images          []arrImage `json:"images"`
		}
		endpoint := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s", radarrURL, url.QueryEscape(start), url.QueryEscape(end))
		if err := h.arrGet(endpoint, radarrKey, &raw); err != nil {
			firstErr = fmt.Errorf("radarr: %w", err)
		} else {
			for _, m := range raw {
				date, relType := pickMovieDate(m.DigitalRelease, m.PhysicalRelease, m.InCinemas, start, end)
				if date == "" {
					continue
				}
				items = append(items, CalendarItem{
					Type:           "movie",
					Title:          m.Title,
					ReleaseType:    relType,
					Date:           date,
					DigitalRelease: m.DigitalRelease,
					Overview:       m.Overview,
					Poster:         posterURL(m.Images),
					HasFile:        m.HasFile,
				})
			}
		}
	}

	if sonarrEnabled {
		var raw []struct {
			Title         string `json:"title"`
			Overview      string `json:"overview"`
			SeasonNumber  int    `json:"seasonNumber"`
			EpisodeNumber int    `json:"episodeNumber"`
			AirDateUtc    string `json:"airDateUtc"`
			HasFile       bool   `json:"hasFile"`
			Series        struct {
				Title  string     `json:"title"`
				Images []arrImage `json:"images"`
			} `json:"series"`
		}
		endpoint := fmt.Sprintf("%s/api/v3/calendar?start=%s&end=%s&includeSeries=true", sonarrURL, url.QueryEscape(start), url.QueryEscape(end))
		if err := h.arrGet(endpoint, sonarrKey, &raw); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("sonarr: %w", err)
			}
		} else {
			for _, e := range raw {
				if e.AirDateUtc == "" {
					continue
				}
				items = append(items, CalendarItem{
					Type:          "episode",
					Title:         e.Series.Title,
					EpisodeTitle:  e.Title,
					SeasonNumber:  e.SeasonNumber,
					EpisodeNumber: e.EpisodeNumber,
					Date:          e.AirDateUtc,
					Overview:      e.Overview,
					Poster:        posterURL(e.Series.Images),
					HasFile:       e.HasFile,
				})
			}
		}
	}

	// Surface an upstream failure only when it left us with nothing to show.
	if len(items) == 0 && firstErr != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": firstErr.Error()})
		return
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Date < items[j].Date })
	c.JSON(http.StatusOK, items)
}

// Availability reports whether Radarr/Sonarr are configured, so any
// authenticated user's client can show or hide the Requests / Calendar
// features. Returns only booleans — no URLs or keys.
//
// @Summary      Requests feature availability
// @Tags         request
// @Produce      json
// @Success      200  {object}  map[string]bool  "{radarr, sonarr, enabled}"
// @Security     BearerAuth
// @Router       /request/availability [get]
func (h *RequestHandler) Availability(c *gin.Context) {
	_, _, radarr := h.settings.RadarrConfig()
	_, _, sonarr := h.settings.SonarrConfig()
	c.JSON(http.StatusOK, gin.H{
		"radarr":  radarr,
		"sonarr":  sonarr,
		"enabled": radarr || sonarr,
	})
}

type testConnectionRequest struct {
	Service string `json:"service" binding:"required,oneof=radarr sonarr"`
}

// TestConnection pings the saved Radarr/Sonarr instance's system/status
// endpoint and reports whether it is reachable with the current settings.
// Always returns 200 with {ok, message} so the admin UI can render the
// outcome uniformly; ok=false covers "not configured" and upstream errors.
//
// @Summary      Test a Radarr/Sonarr connection
// @Tags         settings
// @Accept       json
// @Produce      json
// @Param        body  body      testConnectionRequest  true  "{service}"
// @Success      200  {object}  map[string]interface{}  "{ok, message}"
// @Failure      400  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/settings/integrations/test [post]
func (h *RequestHandler) TestConnection(c *gin.Context) {
	var req testConnectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var arrURL, arrKey string
	switch req.Service {
	case "radarr":
		arrURL, arrKey, _ = h.settings.RadarrConfig()
	case "sonarr":
		arrURL, arrKey, _ = h.settings.SonarrConfig()
	}
	if arrURL == "" {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": req.Service + " is not configured"})
		return
	}
	var status struct {
		Version string `json:"version"`
	}
	if err := h.arrGet(arrURL+"/api/v3/system/status", arrKey, &status); err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": false, "message": err.Error()})
		return
	}
	msg := "Connected"
	if status.Version != "" {
		msg = "Connected — version " + status.Version
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "message": msg})
}

// posterURL returns the best poster URL from an image set, preferring the
// remote (absolute) URL over the local one.
func posterURL(images []arrImage) string {
	for _, img := range images {
		if img.CoverType == "poster" {
			if img.RemoteURL != "" {
				return img.RemoteURL
			}
			return img.URL
		}
	}
	return ""
}

// pickMovieDate chooses which of a movie's release dates to place on the
// calendar, preferring one that actually falls within [start, end] and, among
// those, digital > physical > cinema.
func pickMovieDate(digital, physical, cinema, start, end string) (date, relType string) {
	cands := []struct{ date, typ string }{
		{digital, "digital"},
		{physical, "physical"},
		{cinema, "cinema"},
	}
	for _, c := range cands {
		if c.date != "" && dateInRange(c.date, start, end) {
			return c.date, c.typ
		}
	}
	for _, c := range cands {
		if c.date != "" {
			return c.date, c.typ
		}
	}
	return "", ""
}

// dateInRange compares the leading YYYY-MM-DD of each timestamp so an RFC3339
// release date can be tested against date-only range bounds.
func dateInRange(date, start, end string) bool {
	if len(date) < 10 || len(start) < 10 || len(end) < 10 {
		return false
	}
	d := date[:10]
	return d >= start[:10] && d <= end[:10]
}

func (h *RequestHandler) fetchDefaults(baseURL, apiKey string) (rootFolder string, qualityID int, err error) {
	var folders []struct {
		Path string `json:"path"`
	}
	if err = h.arrGet(baseURL+"/api/v3/rootfolder", apiKey, &folders); err != nil {
		return
	}
	if len(folders) == 0 {
		err = fmt.Errorf("no root folders configured")
		return
	}
	rootFolder = folders[0].Path

	var profiles []struct {
		ID int `json:"id"`
	}
	if err = h.arrGet(baseURL+"/api/v3/qualityprofile", apiKey, &profiles); err != nil {
		return
	}
	if len(profiles) == 0 {
		err = fmt.Errorf("no quality profiles configured")
		return
	}
	qualityID = profiles[0].ID
	return
}

func (h *RequestHandler) arrGet(endpoint, apiKey string, out any) error {
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", apiKey)
	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		return fmt.Errorf("non-JSON response from %s (Content-Type: %s) — verify the URL and URL Base setting", endpoint, ct)
	}
	return json.Unmarshal(b, out)
}

func (h *RequestHandler) arrPost(endpoint, apiKey string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", endpoint, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("X-Api-Key", apiKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := h.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upstream %d: %s", resp.StatusCode, string(b))
	}
	return nil
}
