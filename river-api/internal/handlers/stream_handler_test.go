package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mediaCtx(target string, rangeHeader string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, target, nil)
	if rangeHeader != "" {
		c.Request.Header.Set("Range", rangeHeader)
	}
	return c, w
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	return p
}

func TestFirstReadable(t *testing.T) {
	existing := writeTempFile(t, "a.mp4", "x")
	assert.Equal(t, existing, firstReadable("", existing), "skips empty, returns first readable")
	assert.Equal(t, existing, firstReadable(existing, "/nope/other.mp4"))
	assert.Equal(t, "", firstReadable("", "/does/not/exist.mp4"), "none readable → empty")
}

func TestStatReason(t *testing.T) {
	assert.Equal(t, "empty", statReason(""))
	assert.Equal(t, "ok", statReason(writeTempFile(t, "b.mp4", "y")))
	assert.NotContains(t, []string{"ok", "empty"}, statReason("/no/such/file.mp4"), "missing file yields the os error string")
}

func TestServeMedia(t *testing.T) {
	t.Run("empty path is 404", func(t *testing.T) {
		c, w := mediaCtx("/", "")
		serveMedia(c, "", false)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("missing file is 404", func(t *testing.T) {
		c, w := mediaCtx("/", "")
		serveMedia(c, "/no/such/file.mp4", false)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("full file is 200 with body", func(t *testing.T) {
		p := writeTempFile(t, "movie.mp4", "hello-bytes")
		c, w := mediaCtx("/", "")
		serveMediaFile(c, p)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "hello-bytes", w.Body.String())
	})

	t.Run("range request is 206 partial", func(t *testing.T) {
		p := writeTempFile(t, "movie.mp4", "0123456789")
		c, w := mediaCtx("/", "bytes=0-3")
		serveMediaFile(c, p)
		require.Equal(t, http.StatusPartialContent, w.Code)
		assert.Equal(t, "0123", w.Body.String())
		assert.NotEmpty(t, w.Header().Get("Content-Range"))
	})

	t.Run("download sets Content-Disposition", func(t *testing.T) {
		p := writeTempFile(t, "movie.mp4", "data")
		c, w := mediaCtx("/", "")
		serveMediaFileDownload(c, p)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	})
}

func TestServeMediaWithFallback(t *testing.T) {
	primary := writeTempFile(t, "primary.mp4", "PRIMARY")
	fallback := writeTempFile(t, "fallback.mp4", "FALLBACK")

	t.Run("serves primary when present", func(t *testing.T) {
		c, w := mediaCtx("/", "")
		serveMediaWithFallback(c, primary, fallback, false)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "PRIMARY", w.Body.String())
	})

	t.Run("falls back when primary missing", func(t *testing.T) {
		c, w := mediaCtx("/", "")
		serveMediaWithFallback(c, "/no/such/primary.mp4", fallback, false)
		require.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "FALLBACK", w.Body.String())
	})

	t.Run("404 when neither is readable", func(t *testing.T) {
		c, w := mediaCtx("/", "")
		serveMediaWithFallback(c, "", "/no/such/fallback.mp4", false)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
