package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"river-api/internal/repository"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	scanURL      string
	metaMovieURL string
	metaTVURL    string
	metaBookURL  string
	metaMusicURL string
	http         *http.Client
	stats        repository.StatsRepository
	movieSvc     *services.MovieService
	tvShowSvc    *services.TVShowService
}

func NewAdminHandler(scanURL, metaMovieURL, metaTVURL, metaBookURL, metaMusicURL string, stats repository.StatsRepository, movieSvc *services.MovieService, tvShowSvc *services.TVShowService) *AdminHandler {
	return &AdminHandler{
		scanURL:      scanURL,
		metaMovieURL: metaMovieURL,
		metaTVURL:    metaTVURL,
		metaBookURL:  metaBookURL,
		metaMusicURL: metaMusicURL,
		http:         &http.Client{Timeout: 30 * time.Second},
		stats:        stats,
		movieSvc:     movieSvc,
		tvShowSvc:    tvShowSvc,
	}
}

// GetStats returns aggregate counts for each media type in the library.
//
// @Summary      Library counts
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]int  "{movies, tv_shows, tracks, audiobooks}"
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/stats [get]
func (h *AdminHandler) GetStats(c *gin.Context) {
	movies, err := h.stats.CountMovies()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tvShows, err := h.stats.CountTVShows()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	tracks, err := h.stats.CountTracks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	audiobooks, err := h.stats.CountAudiobooks()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"movies":     movies,
		"tv_shows":   tvShows,
		"tracks":     tracks,
		"audiobooks": audiobooks,
	})
}

// TriggerScan asks river-scan to run a full library scan.
//
// @Summary      Trigger library scan
// @Tags         admin
// @Produce      json
// @Success      202  {object}  map[string]string
// @Failure      502  {object}  map[string]string  "river-scan unreachable"
// @Failure      503  {object}  map[string]string  "RIVER_SCAN_URL not configured"
// @Security     BearerAuth
// @Router       /admin/scan [post]
func (h *AdminHandler) TriggerScan(c *gin.Context) {
	if h.scanURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_SCAN_URL is not configured"})
		return
	}
	resp, err := h.http.Post(h.scanURL+"/trigger", "application/json", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach scanner: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "scan triggered"})
}

// RequeueUntranscoded asks the scanner to find media rows whose source file
// was discovered but never transcoded (source_path set, file_path empty)
// and republish media_discovered events for each. Used to recover from
// the RabbitMQ queues being drained mid-flight. The scanner runs it
// asynchronously and logs the result counts; the response here is the
// usual 202 "kicked off" shape.
//
// @Summary      Requeue untranscoded media
// @Tags         admin
// @Produce      json
// @Success      202  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/requeue-untranscoded [post]
func (h *AdminHandler) RequeueUntranscoded(c *gin.Context) {
	if h.scanURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_SCAN_URL is not configured"})
		return
	}
	resp, err := h.http.Post(h.scanURL+"/requeue-untranscoded", "application/json", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach scanner: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "requeue triggered"})
}

// ScannerState returns the current contents of the scanner's state file
// (the per-directory content-hash map and the show-folder → show-id map).
// Used by the admin "Scanner State" page to surface entries an admin can
// then forget so they get rediscovered on the next scan.
//
// @Summary      Read scanner state
// @Tags         admin
// @Produce      json
// @Success      200  {object}  map[string]any
// @Failure      502  {object}  map[string]string  "river-scan unreachable"
// @Failure      503  {object}  map[string]string  "RIVER_SCAN_URL not configured"
// @Security     BearerAuth
// @Router       /admin/scanner-state [get]
func (h *AdminHandler) ScannerState(c *gin.Context) {
	if h.scanURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_SCAN_URL is not configured"})
		return
	}
	resp, err := h.http.Get(h.scanURL + "/state")
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach scanner: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	// Stream the response through so we don't have to keep the state map
	// shape in sync between two services. river-scan returns
	// {directories: {...}, shows: {...}} and that's what the client sees.
	c.Status(resp.StatusCode)
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		c.Header("Content-Type", ct)
	}
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		log.Printf("WARN scanner-state: copy response: %v", err)
	}
}

// ForgetScannerState passes a forget request through to river-scan. The
// body shape mirrors river-scan's POST /forget exactly:
//
//	{ paths?: string[], prefixes?: string[], shows?: string[] }
//
// "paths" target exact Directories keys, "prefixes" forget every entry at
// or beneath a parent folder, and "shows" clear cached folder → show-id
// mappings. All three are optional.
//
// @Summary      Forget scanner-state entries
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        body  body      object  true  "{paths?, prefixes?, shows?}"
// @Success      204
// @Failure      400   {object}  map[string]string
// @Failure      502   {object}  map[string]string
// @Failure      503   {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/scanner-state/forget [post]
func (h *AdminHandler) ForgetScannerState(c *gin.Context) {
	if h.scanURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_SCAN_URL is not configured"})
		return
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid body"})
		return
	}
	resp, err := h.http.Post(h.scanURL+"/forget", "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach scanner: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.Status(resp.StatusCode)
}

// IdentifyMovie lets an admin override a movie's title/year/IMDb id and then
// re-run metadata enrichment. Title/year (if provided) are persisted to the
// movie record up-front so they survive even if TMDB returns no match for
// the IMDb id. The IMDb id is forwarded to river-meta-movie as a one-shot
// hint that skips title-similarity search via TMDB's /find endpoint.
// IdentifyMovie applies an admin-supplied title/year/IMDb hint and asks
// river-meta-movie to re-fetch TMDB metadata using the hint.
//
// @Summary      Identify movie
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Movie ID"
// @Param        body  body  object  true  "{title?, year?, imdb_id?}"
// @Success      202   {object}  map[string]string
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      502   {object}  map[string]string
// @Failure      503   {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/identify [post]
func (h *AdminHandler) IdentifyMovie(c *gin.Context) {
	if h.metaMovieURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_MOVIE_URL is not configured"})
		return
	}
	id := c.Param("id")

	var req struct {
		Title  *string `json:"title"`
		Year   *int    `json:"year"`
		IMDBID string  `json:"imdb_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Apply title/year overrides locally before triggering enrichment, so a
	// later failed match doesn't leave the user's edits unsaved.
	if req.Title != nil || req.Year != nil {
		existing, err := h.movieSvc.GetByID(id)
		if err != nil {
			c.JSON(serviceStatus(err), gin.H{"error": "movie not found"})
			return
		}
		title := existing.Title
		if req.Title != nil {
			title = *req.Title
		}
		year := existing.Year
		if req.Year != nil {
			year = *req.Year
		}
		if _, err := h.movieSvc.Update(id, services.MovieInput{
			LibraryID:     existing.LibraryID,
			Title:         title,
			OriginalTitle: existing.OriginalTitle,
			Description:   existing.Description,
			Year:          year,
			Genres:        existing.Genres,
			Rating:        existing.Rating,
			Runtime:       existing.Runtime,
			PosterPath:    existing.PosterPath,
			BackdropPath:  existing.BackdropPath,
			TrailerURL:    existing.TrailerURL,
			FilePath:      existing.FilePath,
		}); err != nil {
			c.JSON(serviceStatus(err), gin.H{"error": "failed to update movie: " + err.Error()})
			return
		}
		// Drop the resolved TMDB id so the refresh below actually re-runs
		// title+year search rather than fetching the previous (now
		// incorrect) record by id. The IMDb hint, when present, beats
		// the stored id anyway, but it's cleaner to reset uniformly so
		// the rename-only path works too.
		if err := h.movieSvc.ClearTMDBID(id); err != nil {
			log.Printf("WARN identify-movie: clear tmdb id for %s: %v", id, err)
		}
	}

	// Forward the IMDb hint to the metadata enhancer. Empty body is fine —
	// the meta service falls back to the (now-updated) title+year search.
	body, _ := json.Marshal(map[string]string{"imdb_id": req.IMDBID})
	resp, err := h.http.Post(h.metaMovieURL+"/refresh/"+id, "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "movie identified, metadata refresh triggered"})
}

// IdentifyTVShow mirrors IdentifyMovie for TV shows. Title/year are persisted
// locally so they survive a no-TMDB-match outcome; the IMDb id is a one-shot
// hint forwarded to river-meta-tv's /find lookup.
// IdentifyTVShow applies an admin-supplied title/year/IMDb hint, asks
// river-meta-tv to re-fetch TMDB metadata, and triggers a folder rescan
// to pick up newly-added episodes.
//
// @Summary      Identify TV show
// @Tags         admin
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "TV show ID"
// @Param        body  body  object  true  "{title?, year?, imdb_id?}"
// @Success      202   {object}  map[string]string
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Failure      502   {object}  map[string]string
// @Failure      503   {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/identify [post]
func (h *AdminHandler) IdentifyTVShow(c *gin.Context) {
	if h.metaTVURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_TV_URL is not configured"})
		return
	}
	id := c.Param("id")

	var req struct {
		Title  *string `json:"title"`
		Year   *int    `json:"year"`
		IMDBID string  `json:"imdb_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Title != nil || req.Year != nil {
		existing, err := h.tvShowSvc.GetShow(id)
		if err != nil {
			c.JSON(serviceStatus(err), gin.H{"error": "tvshow not found"})
			return
		}
		title := existing.Title
		if req.Title != nil {
			title = *req.Title
		}
		year := existing.Year
		if req.Year != nil {
			year = *req.Year
		}
		if _, err := h.tvShowSvc.UpdateShow(id, services.TVShowInput{
			LibraryID:     existing.LibraryID,
			Title:         title,
			OriginalTitle: existing.OriginalTitle,
			Description:   existing.Description,
			Year:          year,
			Status:        existing.Status,
			Genres:        existing.Genres,
			Rating:        existing.Rating,
			PosterPath:    existing.PosterPath,
			BackdropPath:  existing.BackdropPath,
			TrailerURL:    existing.TrailerURL,
		}); err != nil {
			c.JSON(serviceStatus(err), gin.H{"error": "failed to update tvshow: " + err.Error()})
			return
		}
		// Drop the resolved TMDB id so the refresh below actually re-runs
		// title+year search rather than fetching the previous (now
		// incorrect) record by id. The IMDb hint, when present, beats
		// the stored id anyway, but it's cleaner to reset uniformly so
		// the rename-only path works too.
		if err := h.tvShowSvc.ClearTMDBID(id); err != nil {
			log.Printf("WARN identify-tvshow: clear tmdb id for %s: %v", id, err)
		}
	}

	body, _ := json.Marshal(map[string]string{"imdb_id": req.IMDBID})
	resp, err := h.http.Post(h.metaTVURL+"/refresh/"+id, "application/json", bytes.NewReader(body))
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return
	}
	defer resp.Body.Close()

	// Fire-and-forget: also trigger a filesystem rescan of the show folder
	// so episodes added since the last scan get picked up. Skips silently
	// when the show row has no folder_path yet (pre-migration data) or
	// RIVER_SCAN_URL isn't configured — identify still succeeds, metadata
	// refresh still runs, just no episode pickup.
	go h.triggerShowRescan(id)

	c.JSON(http.StatusAccepted, gin.H{"message": "tvshow identified, metadata refresh and rescan triggered"})
}

func (h *AdminHandler) triggerShowRescan(showID string) {
	if h.scanURL == "" {
		return
	}
	show, err := h.tvShowSvc.GetShow(showID)
	if err != nil {
		log.Printf("WARN identify-tvshow: get show %s for rescan: %v", showID, err)
		return
	}

	// If folder_path is recorded, scan just that directory with force=true
	// so the content-hash short-circuit doesn't suppress events for seasons
	// the user expects us to re-check.
	if show.FolderPath != "" {
		body, _ := json.Marshal(map[string]any{
			"library_id":   show.LibraryID.String(),
			"library_type": "tvshow",
			"dir_path":     show.FolderPath,
			// Empty show_path → scanner takes the show-level branch of
			// ScanDir (walk every season subdir) rather than treating the
			// folder as a single season.
			"show_name": filepath.Base(show.FolderPath),
			"force":     true,
		})
		resp, err := h.http.Post(h.scanURL+"/scan-dir", "application/json", bytes.NewReader(body))
		if err != nil {
			log.Printf("WARN identify-tvshow: scan-dir trigger for %s: %v", showID, err)
			return
		}
		resp.Body.Close()
		return
	}

	// folder_path isn't set yet — likely a show created before that field
	// existed. Fall back to a full library scan: the scanner's resolveShow
	// will backfill folder_path as a side effect, and subsequent identifies
	// take the targeted path above. The full scan is slower but unblocks
	// the user without requiring them to run anything manually.
	log.Printf("INFO identify-tvshow: show %s has no folder_path; triggering full library scan to backfill", showID)
	resp, err := h.http.Post(h.scanURL+"/trigger", "application/json", nil)
	if err != nil {
		log.Printf("WARN identify-tvshow: full-scan trigger for %s: %v", showID, err)
		return
	}
	resp.Body.Close()
}

// Unidentified lists media records the metadata enhancer hasn't populated
// (empty poster_path). The admin dashboard uses this to surface a "fix me"
// list with per-item Identify buttons.
type unidentifiedItem struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "movie" | "tvshow"
	Title     string `json:"title"`
	Year      int    `json:"year"`
	LibraryID string `json:"library_id"`
	FilePath  string `json:"file_path,omitempty"`
}

// Unidentified lists movies + TV shows the metadata enhancer hasn't
// populated yet (no poster). Used by the admin "fix me" workflow.
//
// @Summary      List unidentified media
// @Tags         admin
// @Produce      json
// @Success      200  {array}   handlers.unidentifiedItem
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/unidentified [get]
func (h *AdminHandler) Unidentified(c *gin.Context) {
	movies, err := h.movieSvc.ListUnidentified()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list unidentified movies"})
		return
	}
	shows, err := h.tvShowSvc.ListUnidentifiedShows()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list unidentified tvshows"})
		return
	}
	out := make([]unidentifiedItem, 0, len(movies)+len(shows))
	for _, m := range movies {
		out = append(out, unidentifiedItem{
			ID: m.ID.String(), Type: "movie",
			Title: m.Title, Year: m.Year,
			LibraryID: m.LibraryID.String(), FilePath: m.FilePath,
		})
	}
	for _, s := range shows {
		out = append(out, unidentifiedItem{
			ID: s.ID.String(), Type: "tvshow",
			Title: s.Title, Year: s.Year,
			LibraryID: s.LibraryID.String(),
		})
	}
	c.JSON(http.StatusOK, out)
}

// RefreshMovieMetadata asks river-meta-movie to re-fetch TMDB data for a movie.
//
// @Summary      Refresh movie metadata
// @Tags         admin
// @Produce      json
// @Param        id  path  string  true  "Movie ID"
// @Success      202  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/refresh-metadata [post]
func (h *AdminHandler) RefreshMovieMetadata(c *gin.Context) {
	if h.metaMovieURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_MOVIE_URL is not configured"})
		return
	}
	id := c.Param("id")
	resp, err := h.http.Post(h.metaMovieURL+"/refresh/"+id, "application/json", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "metadata refresh triggered"})
}

// RefreshTVShowMetadata asks river-meta-tv to re-fetch TMDB data for a show.
//
// @Summary      Refresh TV show metadata
// @Tags         admin
// @Produce      json
// @Param        id  path  string  true  "TV show ID"
// @Success      202  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/refresh-metadata [post]
func (h *AdminHandler) RefreshTVShowMetadata(c *gin.Context) {
	if h.metaTVURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_TV_URL is not configured"})
		return
	}
	id := c.Param("id")
	resp, err := h.http.Post(h.metaTVURL+"/refresh/"+id, "application/json", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "metadata refresh triggered"})
}

// RefreshAudiobookMetadata asks river-meta-book to re-fetch Open Library
// data for an audiobook.
//
// @Summary      Refresh audiobook metadata
// @Tags         admin
// @Produce      json
// @Param        id  path  string  true  "Audiobook ID"
// @Success      202  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id}/refresh-metadata [post]
func (h *AdminHandler) RefreshAudiobookMetadata(c *gin.Context) {
	if h.metaBookURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_BOOK_URL is not configured"})
		return
	}
	id := c.Param("id")
	resp, err := h.http.Post(h.metaBookURL+"/refresh/"+id, "application/json", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "metadata refresh triggered"})
}

// RefreshArtistMetadata asks river-meta-music to re-fetch metadata for an artist.
//
// @Summary      Refresh artist metadata
// @Tags         admin
// @Produce      json
// @Param        id  path  string  true  "Artist ID"
// @Success      202  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /artists/{id}/refresh-metadata [post]
func (h *AdminHandler) RefreshArtistMetadata(c *gin.Context) {
	if h.metaMusicURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_MUSIC_URL is not configured"})
		return
	}
	id := c.Param("id")
	resp, err := h.http.Post(h.metaMusicURL+"/refresh/artist/"+id, "application/json", nil)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	c.JSON(http.StatusAccepted, gin.H{"message": "metadata refresh triggered"})
}
