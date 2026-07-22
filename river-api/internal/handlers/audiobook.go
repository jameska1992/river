package handlers

import (
	"net/http"
	"path/filepath"
	"strconv"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type AudiobookHandler struct {
	svc       *services.AudiobookService
	scan      *scanNotifier
	mediaBase string
}

func NewAudiobookHandler(svc *services.AudiobookService, scanURL, mediaBase string) *AudiobookHandler {
	return &AudiobookHandler{
		svc:       svc,
		scan:      newScanNotifier(scanURL),
		mediaBase: mediaBase,
	}
}

type audiobookRequest struct {
	LibraryID   string `json:"library_id" binding:"required,uuid"`
	Title       string `json:"title" binding:"required"`
	Author      string `json:"author"`
	Narrator    string `json:"narrator"`
	Description string `json:"description"`
	Year        int    `json:"year"`
	Genre       string `json:"genre"`
	CoverPath   string `json:"cover_path"`
	Duration    int    `json:"duration"`
}

// List returns a paginated audiobook list.
//
// @Summary      List audiobooks
// @Tags         audiobooks
// @Produce      json
// @Param        library_id  query  string  false  "Filter by library"
// @Param        page        query  int     false  "Page number"
// @Param        limit       query  int     false  "Page size"
// @Param        sort        query  string  false  "title | author | year | added"
// @Param        order       query  string  false  "asc | desc"
// @Success      200  {array}   models.Audiobook
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks [get]
func (h *AudiobookHandler) List(c *gin.Context) {
	page, limit := parsePaginationQuery(c)
	libraryID := c.Query("library_id")
	books, err := h.svc.List(services.AudiobookFilter{
		LibraryID: libraryID, Page: page, Limit: limit,
		Sort:  c.Query("sort"),
		Order: c.Query("order"),
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch audiobooks"})
		return
	}
	if total, err := h.svc.Count(libraryID); err == nil {
		c.Header("X-Total-Count", strconv.FormatInt(total, 10))
	}
	c.JSON(http.StatusOK, books)
}

// Create adds a new audiobook.
//
// @Summary      Create audiobook
// @Tags         audiobooks
// @Accept       json
// @Produce      json
// @Param        body  body      audiobookRequest  true  "Audiobook fields"
// @Success      201   {object}  models.Audiobook
// @Failure      400   {object}  map[string]string
// @Failure      500   {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks [post]
func (h *AudiobookHandler) Create(c *gin.Context) {
	var req audiobookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	book, err := h.svc.Create(services.AudiobookInput{
		LibraryID: libID, Title: req.Title, Author: req.Author, Narrator: req.Narrator,
		Description: req.Description, Year: req.Year, Genre: req.Genre,
		CoverPath: req.CoverPath, Duration: req.Duration,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create audiobook"})
		return
	}
	c.JSON(http.StatusCreated, book)
}

// Get returns a single audiobook.
//
// @Summary      Get audiobook
// @Tags         audiobooks
// @Produce      json
// @Param        id  path  string  true  "Audiobook ID"
// @Success      200  {object}  models.Audiobook
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id} [get]
func (h *AudiobookHandler) Get(c *gin.Context) {
	book, err := h.svc.GetByID(c.Param("id"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "audiobook not found"})
		return
	}
	c.JSON(http.StatusOK, book)
}

// Similar returns audiobooks sharing a Genre with the given one.
//
// @Summary      Similar audiobooks
// @Tags         audiobooks
// @Produce      json
// @Param        id     path   string  true   "Audiobook ID"
// @Param        limit  query  int     false  "1..50, default 16"
// @Success      200  {array}  services.SimilarItem
// @Failure      404  {object} map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id}/similar [get]
func (h *AudiobookHandler) Similar(c *gin.Context) {
	limit := parseSimilarLimit(c.Query("limit"))
	items, err := h.svc.Similar(c.Param("id"), limit)
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "audiobook not found"})
		return
	}
	c.JSON(http.StatusOK, items)
}

// Update replaces audiobook metadata.
//
// @Summary      Update audiobook
// @Tags         audiobooks
// @Accept       json
// @Produce      json
// @Param        id    path      string            true  "Audiobook ID"
// @Param        body  body      audiobookRequest  true  "Audiobook fields"
// @Success      200   {object}  models.Audiobook
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id} [put]
func (h *AudiobookHandler) Update(c *gin.Context) {
	var req audiobookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	libID, err := uuid.Parse(req.LibraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid library_id"})
		return
	}
	book, err := h.svc.Update(c.Param("id"), services.AudiobookInput{
		LibraryID: libID, Title: req.Title, Author: req.Author, Narrator: req.Narrator,
		Description: req.Description, Year: req.Year, Genre: req.Genre,
		CoverPath: req.CoverPath, Duration: req.Duration,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, book)
}

// Delete removes the audiobook (cascades chapters) and asks
// river-scan to forget the content-hash entries for any folder under
// the book on disk so a subsequent scan rediscovers it instead of
// silently skipping a folder whose hash still matches the cache.
//
// We derive the on-disk folder from chapter file paths since the
// Audiobook model itself doesn't record a FolderPath. We send each
// distinct parent directory *and its prefix* to the scanner: the
// parent covers the canonical "one folder per book" layout, the
// prefix covers disc-subfoldered books ("Book/Disc 1/01.mp3") plus
// the case where some chapters never got enriched and left the state
// cache entry orphaned.
//
// @Summary      Delete audiobook
// @Tags         audiobooks
// @Param        id  path  string  true  "Audiobook ID"
// @Success      204
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id} [delete]
func (h *AudiobookHandler) Delete(c *gin.Context) {
	id := c.Param("id")

	// Walk chapters before deleting so we still have file paths to work
	// from. If listing fails we proceed with the delete anyway — the
	// scan-state forget is best-effort.
	bookDirs := map[string]struct{}{}
	if chapters, err := h.svc.ListChapters(id); err == nil {
		for _, ch := range chapters {
			if ch.FilePath == "" {
				continue
			}
			bookDirs[filepath.Dir(ch.FilePath)] = struct{}{}
		}
	}

	if err := h.svc.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete audiobook"})
		return
	}

	paths := make([]string, 0, len(bookDirs))
	prefixes := make([]string, 0, len(bookDirs))
	for d := range bookDirs {
		paths = append(paths, d)
		prefixes = append(prefixes, d)
	}
	h.scan.Forget(paths, nil, prefixes)

	c.JSON(http.StatusNoContent, nil)
}

// --- Chapters ---

type chapterRequest struct {
	Number   int    `json:"number" binding:"required"`
	Title    string `json:"title"`
	Duration int    `json:"duration"`
	FilePath string `json:"file_path" binding:"required"`
}

// ListChapters returns all chapters of an audiobook.
//
// @Summary      List chapters
// @Tags         audiobooks
// @Produce      json
// @Param        id  path  string  true  "Audiobook ID"
// @Success      200  {array}   models.AudiobookChapter
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id}/chapters [get]
func (h *AudiobookHandler) ListChapters(c *gin.Context) {
	chapters, err := h.svc.ListChapters(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch chapters"})
		return
	}
	c.JSON(http.StatusOK, chapters)
}

// CreateChapter adds a chapter to an audiobook.
//
// @Summary      Create chapter
// @Tags         audiobooks
// @Accept       json
// @Produce      json
// @Param        id    path      string          true  "Audiobook ID"
// @Param        body  body      chapterRequest  true  "Chapter fields"
// @Success      201   {object}  models.AudiobookChapter
// @Failure      400   {object}  map[string]string
// @Failure      404   {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id}/chapters [post]
func (h *AudiobookHandler) CreateChapter(c *gin.Context) {
	var req chapterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	chapter, err := h.svc.CreateChapter(c.Param("id"), services.ChapterInput{
		Number: req.Number, Title: req.Title, Duration: req.Duration, FilePath: req.FilePath,
	})
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, chapter)
}

// StreamChapter serves a chapter audio file with Range support.
//
// @Summary      Stream chapter
// @Tags         audiobooks
// @Produce      audio/mp4
// @Param        id          path   string  true   "Audiobook ID"
// @Param        chapterId   path   string  true   "Chapter ID"
// @Param        token       query  string  false  "Stream JWT"
// @Success      200
// @Success      206
// @Failure      404  {object}  map[string]string
// @Security     BearerAuth
// @Router       /audiobooks/{id}/chapters/{chapterId}/stream [get]
func (h *AudiobookHandler) StreamChapter(c *gin.Context) {
	chapter, err := h.svc.GetChapter(c.Param("chapterId"))
	if err != nil {
		c.JSON(serviceStatus(err), gin.H{"error": "chapter not found"})
		return
	}
	serveMediaFile(c, chapter.FilePath)
}
