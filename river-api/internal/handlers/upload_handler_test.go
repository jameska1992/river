package handlers

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// multipartBody builds a multipart/form-data body with the text fields
// first and the file LAST (the order the streaming handler requires).
func multipartBody(fields map[string]string, fileName string, content []byte) (*bytes.Buffer, string) {
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	for _, k := range []string{"type", "library_id", "title", "season", "episode"} {
		if v, ok := fields[k]; ok {
			_ = mw.WriteField(k, v)
		}
	}
	if fileName != "" {
		fw, _ := mw.CreateFormFile("file", fileName)
		_, _ = fw.Write(content)
	}
	_ = mw.Close()
	return buf, mw.FormDataContentType()
}

func uploadHandler(t *testing.T) (*UploadHandler, *models.Library, string) {
	t.Helper()
	dir := t.TempDir()
	lib := &models.Library{Base: models.Base{ID: uuid.New()}, Name: "Movies", Type: "movie", Paths: `["` + dir + `"]`}
	repo := &fakeLibraryRepo{libs: []*models.Library{lib}}
	// empty scannerURL → triggerScanDir is a no-op.
	return NewUploadHandler(services.NewLibraryService(repo), ""), lib, dir
}

func doUpload(h *UploadHandler, body *bytes.Buffer, contentType string) *httptest.ResponseRecorder {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/upload", body)
	c.Request.Header.Set("Content-Type", contentType)
	h.Upload(c)
	return w
}

func TestUploadHandler_Movie_StreamsToDisk(t *testing.T) {
	h, lib, dir := uploadHandler(t)
	body, ct := multipartBody(map[string]string{
		"type": "movie", "library_id": lib.ID.String(), "title": "The Matrix",
	}, "matrix.mp4", []byte("fake-video-bytes"))

	w := doUpload(h, body, ct)
	require.Equal(t, http.StatusAccepted, w.Code)

	got := filepath.Join(dir, "The Matrix", "matrix.mp4")
	data, err := os.ReadFile(got)
	require.NoError(t, err, "file should be written to <library>/<title>/<filename>")
	assert.Equal(t, "fake-video-bytes", string(data))
}

func TestUploadHandler_Episode_StreamsToDisk(t *testing.T) {
	h, lib, dir := uploadHandler(t)
	body, ct := multipartBody(map[string]string{
		"type": "episode", "library_id": lib.ID.String(), "title": "Dragnet",
		"season": "1", "episode": "2",
	}, "clip.mkv", []byte("ep-bytes"))

	w := doUpload(h, body, ct)
	require.Equal(t, http.StatusAccepted, w.Code)

	got := filepath.Join(dir, "Dragnet", "Season 1", "S01E02.mkv")
	data, err := os.ReadFile(got)
	require.NoError(t, err, "episode should be written to <show>/Season N/SxxExx.ext")
	assert.Equal(t, "ep-bytes", string(data))
}

func TestUploadHandler_Validation(t *testing.T) {
	h, lib, _ := uploadHandler(t)

	t.Run("missing title is 400", func(t *testing.T) {
		body, ct := multipartBody(map[string]string{"type": "movie", "library_id": lib.ID.String()}, "x.mp4", []byte("x"))
		assert.Equal(t, http.StatusBadRequest, doUpload(h, body, ct).Code)
	})

	t.Run("invalid type is 400", func(t *testing.T) {
		body, ct := multipartBody(map[string]string{"type": "photo", "library_id": lib.ID.String(), "title": "X"}, "x.mp4", []byte("x"))
		assert.Equal(t, http.StatusBadRequest, doUpload(h, body, ct).Code)
	})

	t.Run("episode without season is 400", func(t *testing.T) {
		body, ct := multipartBody(map[string]string{"type": "episode", "library_id": lib.ID.String(), "title": "X", "episode": "1"}, "x.mkv", []byte("x"))
		assert.Equal(t, http.StatusBadRequest, doUpload(h, body, ct).Code)
	})

	t.Run("no file is 400", func(t *testing.T) {
		body, ct := multipartBody(map[string]string{"type": "movie", "library_id": lib.ID.String(), "title": "X"}, "", nil)
		assert.Equal(t, http.StatusBadRequest, doUpload(h, body, ct).Code)
	})

	t.Run("unknown library is 400", func(t *testing.T) {
		body, ct := multipartBody(map[string]string{"type": "movie", "library_id": uuid.New().String(), "title": "X"}, "x.mp4", []byte("x"))
		assert.Equal(t, http.StatusBadRequest, doUpload(h, body, ct).Code)
	})
}
