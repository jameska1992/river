package handlers

import (
	"net/http"
	"strconv"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type MovieHandler struct {
	svc       *services.MovieService
	scan      *scanNotifier
	mediaBase string
}

func NewMovieHandler(svc *services.MovieService, scanURL, mediaBase string) *MovieHandler {
	return &MovieHandler{
		svc:       svc,
		scan:      newScanNotifier(scanURL),
		mediaBase: mediaBase,
	}
}

type movieRequest struct {
	LibraryID     string  `json:"library_id" binding:"required,uuid"`
	Title         string  `json:"title" binding:"required"`
	OriginalTitle string  `json:"original_title"`
	Description   string  `json:"description"`
	Year          int     `json:"year"`
	Genres        string  `json:"genres"`
	Rating        float32 `json:"rating"`
	Runtime       int     `json:"runtime"`
	PosterPath    string  `json:"poster_path"`
	BackdropPath  string  `json:"backdrop_path"`
	TrailerURL    string  `json:"trailer_url"`
	TMDBID        int     `json:"tmdb_id"`
	FilePath      string  `json:"file_path"`
	SourcePath    string  `json:"source_path"`
}

// List returns a paginated, optionally library-filtered movie list.
//
// @Summary      List movies
// @Tags         movies
// @Produce      json
// @Param        library_id  query  string  false  "Filter by library"
// @Param        page        query  int     false  "Page number (default 1)"
// @Param        limit       query  int     false  "Page size (default 50, max 200)"
// @Param        sort        query  string  false  "title | year | rating | added"
// @Param        order       query  string  false  "asc | desc"
// @Success      200  {array}   models.Movie
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies [get]
func (h *MovieHandler) List(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	libraryID := c.Query("library_id")
	movies, err := h.svc.List(services.MovieFilter{
		LibraryID: libraryID,
		Page:      page,
		Limit:     limit,
		Sort:      c.Query("sort"),
		Order:     c.Query("order"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch movies"})
		return
	}
	// X-Total-Count lets the UI render proper page navigation. Header
	// rather than wrapped body so existing clients (mobile, tv) keep
	// reading the array unchanged.
	if total, err := h.svc.Count(libraryID); err == nil {
		c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	}
	c.JSON(http.StatusOK, movies)
}

// Create adds a new movie row. Admin only — usually called by sibling
// services rather than humans.
//
// @Summary      Create movie
// @Tags         movies
// @Accept       json
// @Produce      json
// @Param        body  body      movieRequest  true  "Movie fields"
// @Success      201   {object}  models.Movie
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies [post]
func (h *MovieHandler) Create(c *gin.Context) {
	var req movieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	movie, err := h.svc.Create(services.MovieInput{
		LibraryID: libID, Title: req.Title, OriginalTitle: req.OriginalTitle,
		Description: req.Description, Year: req.Year, Genres: req.Genres,
		Rating: req.Rating, Runtime: req.Runtime, PosterPath: req.PosterPath,
		BackdropPath: req.BackdropPath, TrailerURL: req.TrailerURL,
		TMDBID: req.TMDBID, FilePath: req.FilePath, SourcePath: req.SourcePath,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create movie"})
		return
	}
	c.JSON(http.StatusCreated, movie)
}

// Get returns a single movie by ID.
//
// @Summary      Get movie
// @Tags         movies
// @Produce      json
// @Param        id  path  string  true  "Movie ID"
// @Success      200  {object}  models.Movie
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id} [get]
func (h *MovieHandler) Get(c *gin.Context) {
	movie, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "movie not found"})
		return
	}
	c.JSON(http.StatusOK, movie)
}

// Similar returns up to `limit` movies whose genres overlap with the
// given movie's. See services.MovieService.Similar for the ranking.
//
// @Summary      Similar movies
// @Tags         movies
// @Produce      json
// @Param        id     path   string  true   "Movie ID"
// @Param        limit  query  int     false  "1..50, default 16"
// @Success      200   {array}  services.SimilarItem
// @Failure      404   {object} map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/similar [get]
func (h *MovieHandler) Similar(c *gin.Context) {
	limit := parseSimilarLimit(c.Query("limit"))
	items, err := h.svc.Similar(c.Param("id"), limit)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "movie not found"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// Update replaces a movie's full metadata. Admin only.
//
// @Summary      Update movie
// @Tags         movies
// @Accept       json
// @Produce      json
// @Param        id    path      string        true  "Movie ID"
// @Param        body  body      movieRequest  true  "Movie fields"
// @Success      200   {object}  models.Movie
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id} [put]
func (h *MovieHandler) Update(c *gin.Context) {
	var req movieRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	movie, err := h.svc.Update(c.Param("id"), services.MovieInput{
		LibraryID: libID, Title: req.Title, OriginalTitle: req.OriginalTitle,
		Description: req.Description, Year: req.Year, Genres: req.Genres,
		Rating: req.Rating, Runtime: req.Runtime, PosterPath: req.PosterPath,
		BackdropPath: req.BackdropPath, TrailerURL: req.TrailerURL,
		TMDBID: req.TMDBID, FilePath: req.FilePath, SourcePath: req.SourcePath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, movie)
}

// UpdateFilePath sets just the transcoded file_path. Used by the
// transcoder to avoid racing meta updates on other fields.
//
// @Summary      Patch movie file path
// @Tags         movies
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Movie ID"
// @Param        body  body  object  true  "{file_path}"
// @Success      200   {object}  models.Movie
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/file-path [patch]
func (h *MovieHandler) UpdateFilePath(c *gin.Context) {
	var req struct {
		FilePath string `json:"file_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	movie, err := h.svc.UpdateFilePath(c.Param("id"), req.FilePath)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, movie)
}

// UpdateSourcePath sets just the on-disk source_path. Used by the
// scanner to backfill legacy rows.
//
// @Summary      Patch movie source path
// @Tags         movies
// @Accept       json
// @Produce      json
// @Param        id    path  string  true  "Movie ID"
// @Param        body  body  object  true  "{source_path}"
// @Success      200   {object}  models.Movie
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/source-path [patch]
func (h *MovieHandler) UpdateSourcePath(c *gin.Context) {
	var req struct {
		SourcePath string `json:"source_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	movie, err := h.svc.UpdateSourcePath(c.Param("id"), req.SourcePath)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, movie)
}

// Delete removes a movie row, asks river-scan to forget its source path
// (so the next scan re-discovers if the file is still there), and — when
// ?delete_files=true — also removes the source + transcoded files on
// disk. Files are deleted *before* the DB row so a failure leaves the
// row intact and the admin can see what went wrong; conversely the
// scanner forget is best-effort after the DB delete so a stale state
// hash never blocks recovery.
//
// @Summary      Delete movie
// @Tags         movies
// @Param        id            path   string  true   "Movie ID"
// @Param        delete_files  query  bool    false  "Also remove source/transcoded files"
// @Success      204
// @Failure      404  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id} [delete]
func (h *MovieHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	deleteFiles := parseBoolQuery(c.Query("delete_files"))

	movie, err := h.svc.GetByID(id)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "movie not found"})
		return
	}

	if deleteFiles {
		// Source first — that's the file the scanner re-discovers from. If
		// transcoded output lives outside MEDIA_BASE_PATH (typical when
		// OUTPUT_DIR is mounted separately) the guard will skip it; that's
		// fine — orphaned outputs don't break re-scan.
		if err := removeUnderBase(movie.SourcePath, h.mediaBase); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete source file: " + err.Error()})
			return
		}
		if movie.FilePath != "" && movie.FilePath != movie.SourcePath {
			if err := removeUnderBase(movie.FilePath, h.mediaBase); err != nil {
				// Non-fatal — log and continue. Source is gone, DB row
				// will follow; the transcoded output is recoverable
				// later if needed.
				c.Header("X-Delete-Warning", "failed to delete transcoded file: "+err.Error())
			}
		}
	}

	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete movie"})
		return
	}

	if movie.SourcePath != "" {
		h.scan.Forget([]string{movie.SourcePath}, nil, nil)
	}

	c.JSON(http.StatusNoContent, nil)
}

// Stream serves the transcoded movie file with HTTP Range support so
// <video> elements can seek without downloading the whole thing. Falls
// back to source_path when file_path doesn't exist yet (transcode not
// finished). Accepts the stream token via Authorization header *or*
// ?token= query param since <video src> can't send headers.
//
// @Summary      Stream movie
// @Tags         movies
// @Produce      video/mp4
// @Param        id     path   string  true   "Movie ID"
// @Param        token  query  string  false  "Stream JWT (alternative to Authorization header)"
// @Success      200
// @Success      206
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/stream [get]
func (h *MovieHandler) Stream(c *gin.Context) {
	movie, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "movie not found"})
		return
	}
	serveMediaWithFallback(c, movie.FilePath, movie.SourcePath, false)
}

// Download serves the movie as an attachment (Content-Disposition).
//
// @Summary      Download movie
// @Tags         movies
// @Produce      video/mp4
// @Param        id     path   string  true   "Movie ID"
// @Param        token  query  string  false  "Stream JWT"
// @Success      200
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /movies/{id}/download [get]
func (h *MovieHandler) Download(c *gin.Context) {
	movie, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "movie not found"})
		return
	}
	serveMediaWithFallback(c, movie.FilePath, movie.SourcePath, true)
}
