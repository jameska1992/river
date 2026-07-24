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

func movieRouter(movies *fakeMovieRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewMovieService(movies, fakeCleanupRepo{})
	h := NewMovieHandler(svc, "", "") // empty scanURL/mediaBase — no external calls in these paths
	r := gin.New()
	r.POST("/movies", h.Create)
	r.GET("/movies/:id", h.Get)
	r.DELETE("/movies/:id", h.Delete)
	return r
}

func TestMovieHandler_Create(t *testing.T) {
	repo := &fakeMovieRepo{}
	r := movieRouter(repo)

	t.Run("missing title is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/movies", `{"library_id":"`+uuid.New().String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non-uuid library_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/movies", `{"library_id":"not-a-uuid","title":"X"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/movies", `{"library_id":"`+uuid.New().String()+`","title":"Metropolis"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.movies, 1)
	})
}

func TestMovieHandler_Get(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis"}
	r := movieRouter(&fakeMovieRepo{movies: []*models.Movie{movie}})

	t.Run("found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/movies/"+movie.ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/movies/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestMovieHandler_Delete(t *testing.T) {
	// SourcePath empty so Delete doesn't fire a scan-notify HTTP call.
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Gone"}
	repo := &fakeMovieRepo{movies: []*models.Movie{movie}}
	r := movieRouter(repo)

	t.Run("existing is 204 and removes the row", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/movies/"+movie.ID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, repo.movies)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/movies/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
