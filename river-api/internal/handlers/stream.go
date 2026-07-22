package handlers

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// serveMediaFile serves a file with full support for HTTP Range requests (seeking).
func serveMediaFile(c *gin.Context, filePath string) {
	serveMedia(c, filePath, false)
}

// serveMediaFileDownload serves a file as an attachment so the browser saves it.
func serveMediaFileDownload(c *gin.Context, filePath string) {
	serveMedia(c, filePath, true)
}

// serveMediaWithFallback streams from primary if present, otherwise from
// fallback. Lets the UI offer playback before the transcode/copy pipeline
// has settled a canonical FilePath: primary = post-transcode location,
// fallback = the original source the scanner discovered.
func serveMediaWithFallback(c *gin.Context, primary, fallback string, download bool) {
	if path := firstReadable(primary, fallback); path != "" {
		serveMedia(c, path, download)
		return
	}
	// Log enough to distinguish "DB row has no paths recorded" from "paths
	// recorded but unreadable" (mount mismatch, permission, deleted file).
	// Either case 404s the client, but the operator needs different fixes.
	log.Printf("WARN stream 404: no readable file — primary=%q (%s) fallback=%q (%s)",
		primary, statReason(primary), fallback, statReason(fallback))
	c.JSON(http.StatusNotFound, gin.H{"error": "media file not available"})
}

// statReason returns a short status string for a path: "empty" when the
// path itself is blank, "ok" when the file exists, or the os.Stat error
// otherwise. Used only in the 404 log line above.
func statReason(path string) string {
	if path == "" {
		return "empty"
	}
	if _, err := os.Stat(path); err == nil {
		return "ok"
	} else {
		return err.Error()
	}
}

// firstReadable returns the first non-empty path whose file exists and can
// be opened. Returns "" when neither is usable.
func firstReadable(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func serveMedia(c *gin.Context, filePath string, download bool) {
	if filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "media file not available"})
		return
	}
	file, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			c.JSON(http.StatusNotFound, gin.H{"error": "media file not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to open media file"})
		}
		return
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to stat media file"})
		return
	}

	name := filepath.Base(filePath)
	if download {
		c.Header("Content-Disposition", `attachment; filename="`+name+`"`)
	}
	http.ServeContent(c.Writer, c.Request, name, info.ModTime(), file)
}
