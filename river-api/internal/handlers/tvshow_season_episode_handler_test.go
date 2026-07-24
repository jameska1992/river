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

func tvNestedRouter(shows *fakeShowRepo, seasons *fakeSeasonRepo, episodes *fakeEpisodeRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewTVShowService(shows, seasons, episodes, fakeCleanupRepo{})
	h := NewTVShowHandler(svc, "", "")
	r := gin.New()
	r.POST("/tvshows/:id/seasons", h.CreateSeason)
	r.POST("/tvshows/:id/seasons/:seasonId/episodes", h.CreateEpisode)
	return r
}

func TestTVShowHandler_CreateSeason(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Dragnet"}
	seasons := &fakeSeasonRepo{}
	r := tvNestedRouter(&fakeShowRepo{shows: []*models.TVShow{show}}, seasons, &fakeEpisodeRepo{})

	t.Run("unknown show is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/tvshows/"+uuid.New().String()+"/seasons", `{"number":1}`)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("valid under existing show is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/tvshows/"+show.ID.String()+"/seasons", `{"number":1,"title":"Season 1"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, seasons.seasons, 1)
	})
}

func TestTVShowHandler_CreateEpisode(t *testing.T) {
	showID := uuid.New()
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 1}
	show := &models.TVShow{Base: models.Base{ID: showID}, Title: "Dragnet"}
	episodes := &fakeEpisodeRepo{}
	r := tvNestedRouter(
		&fakeShowRepo{shows: []*models.TVShow{show}},
		&fakeSeasonRepo{seasons: []*models.Season{season}},
		episodes,
	)
	base := "/tvshows/" + showID.String() + "/seasons/" + season.ID.String() + "/episodes"

	t.Run("missing number is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, base, `{"title":"Pilot"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown season is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPost,
			"/tvshows/"+showID.String()+"/seasons/"+uuid.New().String()+"/episodes",
			`{"number":1,"title":"Pilot"}`)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("valid under existing season is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, base, `{"number":1,"title":"Pilot"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, episodes.episodes, 1)
	})
}
