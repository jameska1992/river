package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// identifyAudiobookSetup wires the admin handler with a real AudiobookService
// and an httptest stand-in for river-meta-book that records the refresh call.
func identifyAudiobookSetup(t *testing.T, metaConfigured bool) (*gin.Engine, *fakeAudiobookRepo, *string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	var refreshPath string
	meta := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		refreshPath = r.URL.Path
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(meta.Close)

	books := &fakeAudiobookRepo{}
	svc := services.NewAudiobookService(books, &fakeChapterRepo{}, fakeCleanupRepo{})
	metaURL := meta.URL
	if !metaConfigured {
		metaURL = ""
	}
	h := NewAdminHandler("", "", "", metaURL, "", nil, nil, nil, svc)

	r := gin.New()
	r.POST("/audiobooks/:id/identify", h.IdentifyAudiobook)
	return r, books, &refreshPath
}

func TestIdentifyAudiobook_PersistsKeyAndTriggersRefresh(t *testing.T) {
	r, books, refreshPath := identifyAudiobookSetup(t, true)
	bookID := uuid.New()
	books.books = []*models.Audiobook{{Base: models.Base{ID: bookID}, Title: "Dune"}}

	// A bare OLID normalises to the canonical /works/ form.
	w := doJSON(r, http.MethodPost, "/audiobooks/"+bookID.String()+"/identify", `{"open_library_key":"OL45804W"}`)
	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "/works/OL45804W", books.books[0].OpenLibraryKey, "key persisted in canonical form")
	assert.Equal(t, "/refresh/"+bookID.String(), *refreshPath, "meta-book refresh triggered")
}

func TestIdentifyAudiobook_AcceptsWorksPathForm(t *testing.T) {
	r, books, _ := identifyAudiobookSetup(t, true)
	bookID := uuid.New()
	books.books = []*models.Audiobook{{Base: models.Base{ID: bookID}, Title: "Dune"}}

	w := doJSON(r, http.MethodPost, "/audiobooks/"+bookID.String()+"/identify", `{"open_library_key":"/works/OL45804W"}`)
	require.Equal(t, http.StatusAccepted, w.Code)
	assert.Equal(t, "/works/OL45804W", books.books[0].OpenLibraryKey)
}

func TestIdentifyAudiobook_InvalidKeyIs400(t *testing.T) {
	r, books, refreshPath := identifyAudiobookSetup(t, true)
	bookID := uuid.New()
	books.books = []*models.Audiobook{{Base: models.Base{ID: bookID}, Title: "Dune"}}

	w := doJSON(r, http.MethodPost, "/audiobooks/"+bookID.String()+"/identify", `{"open_library_key":"not-a-key"}`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Empty(t, books.books[0].OpenLibraryKey, "record untouched on invalid input")
	assert.Empty(t, *refreshPath, "no refresh on invalid input")
}

func TestIdentifyAudiobook_UnknownBookIs404(t *testing.T) {
	r, _, refreshPath := identifyAudiobookSetup(t, true)
	w := doJSON(r, http.MethodPost, "/audiobooks/"+uuid.New().String()+"/identify", `{"open_library_key":"OL45804W"}`)
	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Empty(t, *refreshPath, "no refresh when the book doesn't exist")
}

func TestIdentifyAudiobook_NoMetaURLIs503(t *testing.T) {
	r, books, _ := identifyAudiobookSetup(t, false)
	bookID := uuid.New()
	books.books = []*models.Audiobook{{Base: models.Base{ID: bookID}, Title: "Dune"}}

	w := doJSON(r, http.MethodPost, "/audiobooks/"+bookID.String()+"/identify", `{"open_library_key":"OL45804W"}`)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}
