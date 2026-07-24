package handlers

import (
	"net/http"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func audiobookRouter(books *fakeAudiobookRepo, chapters *fakeChapterRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewAudiobookService(books, chapters, fakeCleanupRepo{})
	h := NewAudiobookHandler(svc, "", "")
	r := gin.New()
	r.POST("/audiobooks", h.Create)
	r.GET("/audiobooks/:id", h.Get)
	r.POST("/audiobooks/:id/chapters", h.CreateChapter)
	return r
}

func TestAudiobookHandler_Create(t *testing.T) {
	repo := &fakeAudiobookRepo{}
	r := audiobookRouter(repo, &fakeChapterRepo{})

	t.Run("missing title is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audiobooks", `{"library_id":"`+uuid.New().String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non-uuid library_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audiobooks", `{"library_id":"nope","title":"Dracula"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audiobooks", `{"library_id":"`+uuid.New().String()+`","title":"Dracula"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.books, 1)
	})
}

func TestAudiobookHandler_Get(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dracula"}
	r := audiobookRouter(&fakeAudiobookRepo{books: []*models.Audiobook{book}}, &fakeChapterRepo{})

	t.Run("found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/audiobooks/"+book.ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/audiobooks/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestAudiobookHandler_CreateChapter(t *testing.T) {
	book := &models.Audiobook{Base: models.Base{ID: uuid.New()}, Title: "Dracula"}
	chapters := &fakeChapterRepo{}
	r := audiobookRouter(&fakeAudiobookRepo{books: []*models.Audiobook{book}}, chapters)

	valid := `{"number":1,"title":"Chapter 1","file_path":"/ch1.m4a"}`

	t.Run("missing required fields is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audiobooks/"+book.ID.String()+"/chapters", `{"title":"Chapter 1"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown audiobook is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audiobooks/"+uuid.New().String()+"/chapters", valid)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("valid under existing book is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audiobooks/"+book.ID.String()+"/chapters", valid)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, chapters.chapters, 1)
	})
}
