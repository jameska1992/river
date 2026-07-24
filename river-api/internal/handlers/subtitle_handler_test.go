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

func subtitleRouter(repo *fakeSubtitleRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSubtitleHandler(services.NewSubtitleService(repo))
	r := gin.New()
	r.POST("/subtitles", h.Create)
	r.GET("/movies/:id/subtitles", h.ListMovieSubtitles)
	r.DELETE("/subtitles/:id", h.Delete)
	return r
}

func TestSubtitleHandler_Create(t *testing.T) {
	repo := &fakeSubtitleRepo{}
	r := subtitleRouter(repo)

	t.Run("missing required fields is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/subtitles", `{"media_type":"movie","media_id":"m1"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		body := `{"media_type":"movie","media_id":"m1","language":"en","file_path":"/en.vtt"}`
		w := doJSON(r, http.MethodPost, "/subtitles", body)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.subs, 1)
	})
}

func TestSubtitleHandler_ListMovie(t *testing.T) {
	movieID := uuid.New().String()
	repo := &fakeSubtitleRepo{subs: []*models.Subtitle{
		{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: movieID, Language: "en"},
		{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: movieID, Language: "fr"},
		{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: uuid.New().String(), Language: "de"},
	}}
	r := subtitleRouter(repo)

	w := doJSON(r, http.MethodGet, "/movies/"+movieID+"/subtitles", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"en"`)
	assert.NotContains(t, w.Body.String(), `"de"`, "only this movie's subtitles should be returned")
}

func TestSubtitleHandler_Delete(t *testing.T) {
	sub := &models.Subtitle{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: "m1", Language: "en"}
	repo := &fakeSubtitleRepo{subs: []*models.Subtitle{sub}}
	r := subtitleRouter(repo)

	t.Run("existing is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/subtitles/"+sub.ID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, repo.subs)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/subtitles/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
