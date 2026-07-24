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

func audioTrackRouter(repo *fakeAudioTrackRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewAudioTrackHandler(services.NewAudioTrackService(repo))
	r := gin.New()
	r.POST("/audio-tracks", h.Create)
	r.GET("/movies/:id/audio-tracks", h.ListMovieAudioTracks)
	r.DELETE("/audio-tracks/:id", h.Delete)
	return r
}

func TestAudioTrackHandler_Create(t *testing.T) {
	repo := &fakeAudioTrackRepo{}
	r := audioTrackRouter(repo)

	t.Run("missing required fields is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/audio-tracks", `{"media_type":"movie","media_id":"m1"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		body := `{"media_type":"movie","media_id":"m1","language":"en","file_path":"/en.mp4"}`
		w := doJSON(r, http.MethodPost, "/audio-tracks", body)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.tracks, 1)
	})
}

func TestAudioTrackHandler_ListMovie(t *testing.T) {
	movieID := uuid.New().String()
	repo := &fakeAudioTrackRepo{tracks: []*models.AudioTrack{
		{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: movieID, Language: "en"},
		{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: uuid.New().String(), Language: "de"},
	}}
	r := audioTrackRouter(repo)

	w := doJSON(r, http.MethodGet, "/movies/"+movieID+"/audio-tracks", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"en"`)
	assert.NotContains(t, w.Body.String(), `"de"`, "only this movie's audio tracks should be returned")
}

func TestAudioTrackHandler_Delete(t *testing.T) {
	tr := &models.AudioTrack{Base: models.Base{ID: uuid.New()}, MediaType: "movie", MediaID: "m1", Language: "en"}
	repo := &fakeAudioTrackRepo{tracks: []*models.AudioTrack{tr}}
	r := audioTrackRouter(repo)

	t.Run("existing is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/audio-tracks/"+tr.ID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, repo.tracks)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/audio-tracks/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
