package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

type UploadHandler struct {
	libraries  *services.LibraryService
	scannerURL string
}

func NewUploadHandler(libraries *services.LibraryService, scannerURL string) *UploadHandler {
	return &UploadHandler{libraries: libraries, scannerURL: scannerURL}
}

// Upload accepts a media file via multipart form, writes it to the
// library's first configured path, and triggers a targeted scanner
// rescan so it gets picked up. Multipart fields: file, type
// (movie|episode), library_id, title, [season, episode].
//
// @Summary      Upload media file
// @Tags         admin
// @Accept       multipart/form-data
// @Produce      json
// @Param        file        formData  file    true   "Media file"
// @Param        type        formData  string  true   "movie | episode"
// @Param        library_id  formData  string  true   "Target library"
// @Param        title       formData  string  true   "Used as folder/file name"
// @Param        season      formData  int     false  "Season number (episodes only)"
// @Param        episode     formData  int     false  "Episode number (episodes only)"
// @Success      202  {object}  map[string]string  "{library_id, path}"
// @Failure      400  {object}  map[string]string
// @Failure      500  {object}  map[string]string
// @Security     BearerAuth
// @Router       /admin/upload [post]
func (h *UploadHandler) Upload(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
		return
	}

	mediaType := c.PostForm("type")
	if mediaType != "movie" && mediaType != "episode" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "type must be 'movie' or 'episode'"})
		return
	}

	libraryID := strings.TrimSpace(c.PostForm("library_id"))
	if libraryID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "library_id is required"})
		return
	}

	title := strings.TrimSpace(c.PostForm("title"))
	if title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	lib, err := h.libraries.GetByID(libraryID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "library not found"})
		return
	}

	var paths []string
	if err := json.Unmarshal([]byte(lib.Paths), &paths); err != nil || len(paths) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "library has no configured paths"})
		return
	}
	libraryPath := paths[0]

	ext := filepath.Ext(fh.Filename)

	var destPath, scanDirPath, showPath, showName string
	if mediaType == "movie" {
		destPath = filepath.Join(libraryPath, title, filepath.Base(fh.Filename))
		scanDirPath = filepath.Join(libraryPath, title)
	} else {
		seasonNum, err := strconv.Atoi(strings.TrimSpace(c.PostForm("season")))
		if err != nil || seasonNum < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "season must be a positive integer"})
			return
		}
		episodeNum, err := strconv.Atoi(strings.TrimSpace(c.PostForm("episode")))
		if err != nil || episodeNum < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "episode must be a positive integer"})
			return
		}
		filename := fmt.Sprintf("S%02dE%02d%s", seasonNum, episodeNum, ext)
		showPath = filepath.Join(libraryPath, title)
		showName = title
		scanDirPath = filepath.Join(showPath, fmt.Sprintf("Season %d", seasonNum))
		destPath = filepath.Join(scanDirPath, filename)
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
		return
	}

	src, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open uploaded file"})
		return
	}
	dst, err := os.Create(destPath)
	if err != nil {
		src.Close()
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create destination file"})
		return
	}
	if _, err := io.Copy(dst, src); err != nil {
		dst.Close()
		src.Close()
		os.Remove(destPath)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
		return
	}
	dst.Close()
	src.Close()

	go h.triggerScanDir(string(lib.Type), libraryID, scanDirPath, showPath, showName)

	c.JSON(http.StatusAccepted, gin.H{
		"library_id": libraryID,
		"path":       destPath,
	})
}

func (h *UploadHandler) triggerScanDir(libType, libraryID, dirPath, showPath, showName string) {
	if h.scannerURL == "" {
		return
	}
	body, _ := json.Marshal(map[string]string{
		"library_id":   libraryID,
		"library_type": libType,
		"dir_path":     dirPath,
		"show_path":    showPath,
		"show_name":    showName,
	})
	url := h.scannerURL + "/scan-dir"
	resp, err := http.Post(url, "application/json", bytes.NewReader(body)) //nolint:noctx
	if err != nil {
		log.Printf("WARN upload: scanner scan-dir %s: %v", url, err)
		return
	}
	resp.Body.Close()
}
