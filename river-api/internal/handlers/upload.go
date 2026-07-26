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

// resolvedUpload is the destination + metadata computed from the form
// fields, ready for the file bytes to be streamed to destPath.
type resolvedUpload struct {
	destPath    string
	scanDirPath string
	showPath    string
	showName    string
	libType     string
	libraryID   string
}

// resolveUpload validates the collected text fields against the uploaded
// file name and computes the destination + scan paths. A non-zero status
// signals a validation failure (with an error message).
func (h *UploadHandler) resolveUpload(fields map[string]string, fileName string) (*resolvedUpload, int, string) {
	mediaType := fields["type"]
	if mediaType != "movie" && mediaType != "episode" {
		return nil, http.StatusBadRequest, "type must be 'movie' or 'episode'"
	}
	libraryID := strings.TrimSpace(fields["library_id"])
	if libraryID == "" {
		return nil, http.StatusBadRequest, "library_id is required"
	}
	title := strings.TrimSpace(fields["title"])
	if title == "" {
		return nil, http.StatusBadRequest, "title is required"
	}
	if fileName == "" {
		return nil, http.StatusBadRequest, "file is required"
	}

	lib, err := h.libraries.GetByID(libraryID)
	if err != nil {
		return nil, http.StatusBadRequest, "library not found"
	}
	var paths []string
	if err := json.Unmarshal([]byte(lib.Paths), &paths); err != nil || len(paths) == 0 {
		return nil, http.StatusBadRequest, "library has no configured paths"
	}
	libraryPath := paths[0]

	r := &resolvedUpload{libType: string(lib.Type), libraryID: libraryID}
	ext := filepath.Ext(fileName)
	if mediaType == "movie" {
		r.destPath = filepath.Join(libraryPath, title, filepath.Base(fileName))
		r.scanDirPath = filepath.Join(libraryPath, title)
	} else {
		seasonNum, err := strconv.Atoi(strings.TrimSpace(fields["season"]))
		if err != nil || seasonNum < 1 {
			return nil, http.StatusBadRequest, "season must be a positive integer"
		}
		episodeNum, err := strconv.Atoi(strings.TrimSpace(fields["episode"]))
		if err != nil || episodeNum < 1 {
			return nil, http.StatusBadRequest, "episode must be a positive integer"
		}
		r.showPath = filepath.Join(libraryPath, title)
		r.showName = title
		r.scanDirPath = filepath.Join(r.showPath, fmt.Sprintf("Season %d", seasonNum))
		r.destPath = filepath.Join(r.scanDirPath, fmt.Sprintf("S%02dE%02d%s", seasonNum, episodeNum, ext))
	}
	return r, 0, ""
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
	// Stream the multipart body part-by-part rather than ParseMultipartForm,
	// which buffers the whole file to a temp file (often a RAM-backed /tmp)
	// before we ever touch it. The client sends the `file` field last, so by
	// the time we reach it every text field is known and we can stream the
	// bytes straight to their destination — constant memory, a single write.
	reader, err := c.Request.MultipartReader()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected multipart/form-data"})
		return
	}

	fields := map[string]string{}
	var res *resolvedUpload

	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart form"})
			return
		}

		if part.FormName() != "file" {
			// Small text field — read it fully (capped as a safety net).
			b, err := io.ReadAll(io.LimitReader(part, 1<<20))
			part.Close()
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid form field"})
				return
			}
			fields[part.FormName()] = string(b)
			continue
		}

		// File part.
		r, status, msg := h.resolveUpload(fields, part.FileName())
		if status != 0 {
			part.Close()
			c.JSON(status, gin.H{"error": msg})
			return
		}
		if err := os.MkdirAll(filepath.Dir(r.destPath), 0o755); err != nil {
			part.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create directory"})
			return
		}
		dst, err := os.Create(r.destPath)
		if err != nil {
			part.Close()
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create destination file"})
			return
		}
		if _, err := io.Copy(dst, part); err != nil {
			dst.Close()
			part.Close()
			os.Remove(r.destPath)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to write file"})
			return
		}
		dst.Close()
		part.Close()
		res = r
	}

	if res == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}

	go h.triggerScanDir(res.libType, res.libraryID, res.scanDirPath, res.showPath, res.showName)

	c.JSON(http.StatusAccepted, gin.H{
		"library_id": res.libraryID,
		"path":       res.destPath,
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
