package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type TVShowHandler struct {
	svc       *services.TVShowService
	scan      *scanNotifier
	mediaBase string
}

func NewTVShowHandler(svc *services.TVShowService, scanURL, mediaBase string) *TVShowHandler {
	return &TVShowHandler{
		svc:       svc,
		scan:      newScanNotifier(scanURL),
		mediaBase: mediaBase,
	}
}

// --- TV Shows ---

type tvShowRequest struct {
	LibraryID     string  `json:"library_id" binding:"required,uuid"`
	Title         string  `json:"title" binding:"required"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Status        string  `json:"status"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	TrailerURL    string  `json:"trailer_url"`
	TMDBID        int     `json:"tmdb_id"`
	FolderPath    string  `json:"folder_path"`
}

// ListShows returns a paginated, optionally library-filtered TV show list.
//
// @Summary      List TV shows
// @Tags         tvshows
// @Produce      json
// @Param        library_id  query  string  false  "Filter by library"
// @Param        page        query  int     false  "Page number"
// @Param        limit       query  int     false  "Page size"
// @Param        sort        query  string  false  "title | year | rating | added"
// @Param        order       query  string  false  "asc | desc"
// @Success      200  {array}   models.TVShow
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows [get]
func (h *TVShowHandler) ListShows(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	libraryID := c.Query("library_id")
	shows, err := h.svc.ListShows(services.TVShowFilter{
		LibraryID: libraryID,
		Page:      page, Limit: limit,
		Sort:  c.Query("sort"),
		Order: c.Query("order"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch shows"})
		return
	}
	if total, err := h.svc.CountShows(libraryID); err == nil {
		c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	}
	c.JSON(http.StatusOK, shows)
}

// CreateShow adds a new TV show row.
//
// @Summary      Create TV show
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        body  body      tvShowRequest  true  "Show fields"
// @Success      201   {object}  models.TVShow
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows [post]
func (h *TVShowHandler) CreateShow(c *gin.Context) {
	var req tvShowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	show, err := h.svc.CreateShow(services.TVShowInput{
		LibraryID: libID, Title: req.Title, OriginalTitle: req.OriginalTitle,
		Description: req.Description, Year: req.Year, Status: req.Status,
		Genres: req.Genres, Rating: req.Rating, PosterPath: req.PosterPath,
		BackdropPath: req.BackdropPath, TrailerURL: req.TrailerURL,
		TMDBID: req.TMDBID, FolderPath: req.FolderPath,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create show"})
		return
	}
	c.JSON(http.StatusCreated, show)
}

// GetShow returns a single TV show.
//
// @Summary      Get TV show
// @Tags         tvshows
// @Produce      json
// @Param        id  path  string  true  "TV show ID"
// @Success      200  {object}  models.TVShow
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id} [get]
func (h *TVShowHandler) GetShow(c *gin.Context) {
	show, err := h.svc.GetShow(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "show not found"})
		return
	}
	c.JSON(http.StatusOK, show)
}

// Similar returns shows whose genres overlap with the given show.
//
// @Summary      Similar TV shows
// @Tags         tvshows
// @Produce      json
// @Param        id     path   string  true   "TV show ID"
// @Param        limit  query  int     false  "1..50, default 16"
// @Success      200  {array}  services.SimilarItem
// @Failure      404  {object} map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/similar [get]
func (h *TVShowHandler) Similar(c *gin.Context) {
	limit := parseSimilarLimit(c.Query("limit"))
	items, err := h.svc.SimilarShows(c.Param("id"), limit)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "show not found"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// UpdateShow replaces a TV show's metadata.
//
// @Summary      Update TV show
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id    path      string         true  "TV show ID"
// @Param        body  body      tvShowRequest  true  "Show fields"
// @Success      200   {object}  models.TVShow
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id} [put]
func (h *TVShowHandler) UpdateShow(c *gin.Context) {
	var req tvShowRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	show, err := h.svc.UpdateShow(c.Param("id"), services.TVShowInput{
		LibraryID: libID, Title: req.Title, OriginalTitle: req.OriginalTitle,
		Description: req.Description, Year: req.Year, Status: req.Status,
		Genres: req.Genres, Rating: req.Rating, PosterPath: req.PosterPath,
		BackdropPath: req.BackdropPath, TrailerURL: req.TrailerURL,
		TMDBID: req.TMDBID, FolderPath: req.FolderPath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, show)
}

// UpdateFolderPath sets only the show's folder_path. Used by river-scan to
// backfill the disk location on existing rows so the admin "identify" flow
// can target a re-scan at the right directory.
// UpdateFolderPath patches only the show's folder_path. Used by the
// scanner to backfill on legacy rows without touching metadata.
//
// @Summary      Patch TV show folder path
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "TV show ID"
// @Param        body  body  object  true  "{folder_path}"
// @Success      200   {object}  models.TVShow
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/folder-path [patch]
func (h *TVShowHandler) UpdateFolderPath(c *gin.Context) {
	var req struct {
		FolderPath string `json:"folder_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	show, err := h.svc.UpdateFolderPath(c.Param("id"), req.FolderPath)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, show)
}

// DeleteShow removes a show row (cascades seasons/episodes), asks
// river-scan to forget the per-season content hashes and the
// folder-path → show-id cache entry, and — when ?delete_files=true —
// also recursively removes the show's folder on disk.
//
// The scan-state forget is what makes a re-scan rediscover the content
// instead of skipping it as "already known". Without it, a delete on a
// merged/wrong show would never reappear, even after fixing the
// underlying file layout.
//
// @Summary      Delete TV show
// @Tags         tvshows
// @Param        id            path   string  true   "TV show ID"
// @Param        delete_files  query  bool    false  "Also remove show folder on disk"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id} [delete]
func (h *TVShowHandler) DeleteShow(c *gin.Context) {
	id := c.Param("id")
	deleteFiles := parseBoolQuery(c.Query("delete_files"))

	show, err := h.svc.GetShow(id)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "show not found"})
		return
	}

	// Collect every distinct season-directory path *before* deleting,
	// so we can forget all the per-season content hashes the scanner
	// recorded. The state key for TV shows is the season directory
	// (`river-scan/internal/scanner/scanner.go:scanSeason`), so each
	// distinct parent of an episode source path is one key.
	seasonPaths := map[string]struct{}{}
	if seasons, err := h.svc.ListSeasons(id); err == nil {
		for _, season := range seasons {
			eps, err := h.svc.ListEpisodes(season.ID.String())
			if err != nil {
				continue
			}
			for _, ep := range eps {
				if ep.SourcePath != "" {
					seasonPaths[filepath.Dir(ep.SourcePath)] = struct{}{}
				}
			}
		}
	}

	if deleteFiles {
		// Recursively remove the show's folder on disk. removeUnderBase
		// rejects anything that isn't strictly inside MEDIA_BASE_PATH so
		// a missing/malformed FolderPath can't take out an unrelated
		// directory.
		if show.FolderPath != "" {
			if err := removeUnderBase(show.FolderPath, h.mediaBase); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete show folder: " + err.Error()})
				return
			}
		}
	}

	if err := h.svc.DeleteShow(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete show"})
		return
	}

	paths := make([]string, 0, len(seasonPaths))
	for p := range seasonPaths {
		paths = append(paths, p)
	}
	// Also send FolderPath as a prefix. Seasons that never received an
	// episode (e.g. enrichment failed to parse filenames) leave no
	// per-season path in `paths` above, so without this their state-cache
	// entries would survive the delete and the next scan would skip them.
	var (
		shows    []string
		prefixes []string
	)
	if show.FolderPath != "" {
		shows = []string{show.FolderPath}
		prefixes = []string{show.FolderPath}
	}
	h.scan.Forget(paths, shows, prefixes)

	c.JSON(http.StatusNoContent, nil)
}

// --- Seasons ---

type seasonRequest struct {
	// Number is optional in the binding (no `required`) because 0 is a
	// valid season number — it's the "Specials" season under Plex/TMDB
	// convention. The `required` tag on a Go int would treat 0 as missing
	// and reject the request.
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Year        int    `json:"year"`
	PosterPath  string `json:"poster_path"`
}

// ListSeasons returns all seasons for a show.
//
// @Summary      List seasons
// @Tags         tvshows
// @Produce      json
// @Param        id  path  string  true  "TV show ID"
// @Success      200  {array}   models.Season
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons [get]
func (h *TVShowHandler) ListSeasons(c *gin.Context) {
	seasons, err := h.svc.ListSeasons(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch seasons"})
		return
	}
	c.JSON(http.StatusOK, seasons)
}

// CreateSeason adds a season under a show.
//
// @Summary      Create season
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id    path      string         true  "TV show ID"
// @Param        body  body      seasonRequest  true  "Season fields"
// @Success      201   {object}  models.Season
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons [post]
func (h *TVShowHandler) CreateSeason(c *gin.Context) {
	var req seasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	season, err := h.svc.CreateSeason(c.Param("id"), services.SeasonInput{
		Number: req.Number, Title: req.Title, Description: req.Description,
		Year: req.Year, PosterPath: req.PosterPath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, season)
}

// updateSeasonRequest mirrors seasonRequest but every field is optional so
// the admin can patch a subset (e.g. just the season number) without having
// to repost every field.
type updateSeasonRequest struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Year        int    `json:"year"`
	PosterPath  string `json:"poster_path"`
}

// UpdateSeason patches a season's metadata. Empty fields are skipped.
//
// @Summary      Update season
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id        path      string         true  "TV show ID"
// @Param        seasonId  path      string         true  "Season ID"
// @Param        body      body      seasonRequest  true  "Season fields"
// @Success      200       {object}  models.Season
// @Failure      400       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId} [put]
func (h *TVShowHandler) UpdateSeason(c *gin.Context) {
	var req updateSeasonRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	season, err := h.svc.UpdateSeason(c.Param("id"), c.Param("seasonId"), services.SeasonInput{
		Number: req.Number, Title: req.Title, Description: req.Description,
		Year: req.Year, PosterPath: req.PosterPath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, season)
}

// --- Episodes ---

type episodeRequest struct {
	Number      int    `json:"number" binding:"required"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Runtime     int    `json:"runtime"`
	FilePath    string `json:"file_path"`
	SourcePath  string `json:"source_path"`
	AiredAt     string `json:"aired_at"` // RFC3339
	IsSpecial   bool   `json:"is_special"`
}

type updateEpisodeRequest struct {
	Number      int    `json:"number"`
	SeasonID    string `json:"season_id"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Runtime     int    `json:"runtime"`
	FilePath    string `json:"file_path"`
	SourcePath  string `json:"source_path"`
	AiredAt     string `json:"aired_at"`
}

// ListEpisodes returns all episodes for a season.
//
// @Summary      List episodes
// @Tags         tvshows
// @Produce      json
// @Param        id        path  string  true  "TV show ID"
// @Param        seasonId  path  string  true  "Season ID"
// @Success      200       {array}   models.Episode
// @Failure      500       {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes [get]
func (h *TVShowHandler) ListEpisodes(c *gin.Context) {
	episodes, err := h.svc.ListEpisodes(c.Param("seasonId"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch episodes"})
		return
	}
	c.JSON(http.StatusOK, episodes)
}

// CreateEpisode adds an episode to a season. If the season already has
// an episode at this number, that episode is updated instead.
//
// @Summary      Create episode
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id        path      string          true  "TV show ID"
// @Param        seasonId  path      string          true  "Season ID"
// @Param        body      body      episodeRequest  true  "Episode fields"
// @Success      201       {object}  models.Episode
// @Failure      400       {object}  map[string]string
// @Failure      404       {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes [post]
func (h *TVShowHandler) CreateEpisode(c *gin.Context) {
	var req episodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	episode, err := h.svc.CreateEpisode(c.Param("id"), c.Param("seasonId"), services.EpisodeInput{
		Number: req.Number, Title: req.Title, Description: req.Description,
		Runtime: req.Runtime, FilePath: req.FilePath, SourcePath: req.SourcePath, AiredAt: req.AiredAt,
		IsSpecial: req.IsSpecial,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, episode)
}

// UpdateEpisode patches episode metadata. Empty fields are skipped.
// A non-empty season_id reparents the episode (verified to belong to
// the same show first).
//
// @Summary      Update episode
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id          path      string          true  "TV show ID"
// @Param        seasonId    path      string          true  "Season ID"
// @Param        episodeId   path      string          true  "Episode ID"
// @Param        body        body      episodeRequest  true  "Episode fields"
// @Success      200         {object}  models.Episode
// @Failure      400         {object}  map[string]string
// @Failure      404         {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId} [put]
func (h *TVShowHandler) UpdateEpisode(c *gin.Context) {
	var req updateEpisodeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	episode, err := h.svc.UpdateEpisode(c.Param("episodeId"), services.EpisodeInput{
		Number: req.Number, SeasonID: req.SeasonID,
		Title: req.Title, Description: req.Description,
		Runtime: req.Runtime, FilePath: req.FilePath, SourcePath: req.SourcePath, AiredAt: req.AiredAt,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, episode)
}

// UpdateEpisodeSourcePath sets only the episode's source_path. Used by
// river-scan to backfill rows missing the field so the stream endpoint's
// fallback path works.
// UpdateEpisodeSourcePath patches only source_path. Backfill helper for
// the scanner.
//
// @Summary      Patch episode source path
// @Tags         tvshows
// @Accept       json
// @Produce      json
// @Param        id         path  string  true  "TV show ID"
// @Param        seasonId   path  string  true  "Season ID"
// @Param        episodeId  path  string  true  "Episode ID"
// @Param        body       body  object  true  "{source_path}"
// @Success      200        {object}  models.Episode
// @Failure      400        {object}  map[string]string
// @Failure      404        {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/source-path [patch]
func (h *TVShowHandler) UpdateEpisodeSourcePath(c *gin.Context) {
	var req struct {
		SourcePath string `json:"source_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	episode, err := h.svc.UpdateEpisodeSourcePath(c.Param("episodeId"), req.SourcePath)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, episode)
}

// DeleteEpisode removes a single episode row, optionally deletes the
// transcoded + source files from disk, and asks river-scan to forget
// the per-season content hash so the next scan re-publishes the
// season (re-creating the row if the source is still on disk).
//
// @Summary      Delete episode
// @Tags         tvshows
// @Param        id            path   string  true   "TV show ID"
// @Param        seasonId      path   string  true   "Season ID"
// @Param        episodeId     path   string  true   "Episode ID"
// @Param        delete_files  query  bool    false  "Also remove the episode files on disk"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId} [delete]
func (h *TVShowHandler) DeleteEpisode(c *gin.Context) {
	episodeID := c.Param("episodeId")
	deleteFiles := parseBoolQuery(c.Query("delete_files"))

	episode, err := h.svc.GetEpisode(episodeID)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "episode not found"})
		return
	}

	// Snapshot the on-disk paths *before* mutating anything — once the
	// row is deleted we can't recover them.
	filePath := episode.FilePath
	sourcePath := episode.SourcePath

	if deleteFiles {
		// Both the transcoded copy (FilePath) and the original
		// (SourcePath) are eligible for removal. removeUnderBase
		// rejects anything outside MEDIA_BASE_PATH so a missing /
		// empty value can't take out the wrong file. Hitting a
		// not-found is fine — that path may already be gone.
		if filePath != "" {
			if err := removeUnderBase(filePath, h.mediaBase); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete file_path: " + err.Error()})
				return
			}
		}
		if sourcePath != "" && sourcePath != filePath {
			if err := removeUnderBase(sourcePath, h.mediaBase); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete source_path: " + err.Error()})
				return
			}
		}
	}

	if err := h.svc.DeleteEpisode(episodeID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete episode"})
		return
	}

	// Forget the season directory's content hash so a re-scan won't
	// short-circuit on a still-matching value. The scanner keys TV
	// state at the season-directory level (scanSeason: state.Record
	// uses seasonPath), and the on-disk season dir is the parent of
	// the episode's source path.
	if sourcePath != "" {
		seasonDir := filepath.Dir(sourcePath)
		h.scan.Forget([]string{seasonDir}, nil, nil)
	}

	c.JSON(http.StatusNoContent, nil)
}

// StreamEpisode serves the transcoded episode with Range support.
//
// @Summary      Stream episode
// @Tags         tvshows
// @Produce      video/mp4
// @Param        id         path   string  true   "TV show ID"
// @Param        seasonId   path   string  true   "Season ID"
// @Param        episodeId  path   string  true   "Episode ID"
// @Param        token      query  string  false  "Stream JWT"
// @Success      200
// @Success      206
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/stream [get]
func (h *TVShowHandler) StreamEpisode(c *gin.Context) {
	episode, err := h.svc.GetEpisode(c.Param("episodeId"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "episode not found"})
		return
	}
	serveMediaWithFallback(c, episode.FilePath, episode.SourcePath, false)
}

// DownloadEpisode serves the episode as an attachment.
//
// @Summary      Download episode
// @Tags         tvshows
// @Produce      video/mp4
// @Param        id         path   string  true   "TV show ID"
// @Param        seasonId   path   string  true   "Season ID"
// @Param        episodeId  path   string  true   "Episode ID"
// @Param        token      query  string  false  "Stream JWT"
// @Success      200
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /tvshows/{id}/seasons/{seasonId}/episodes/{episodeId}/download [get]
func (h *TVShowHandler) DownloadEpisode(c *gin.Context) {
	episode, err := h.svc.GetEpisode(c.Param("episodeId"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "episode not found"})
		return
	}
	serveMediaWithFallback(c, episode.FilePath, episode.SourcePath, true)
}
