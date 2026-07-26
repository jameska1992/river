package handlers

import (
	"net/http"
	"testing"

	"river-api/internal/middleware"
	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// progressDeps bundles the fakes a test wants to seed; zero value is fine.
type progressDeps struct {
	progress  *fakeProgressRepo
	movies    *fakeMovieRepo
	episodes  *fakeEpisodeRepo
	seasons   *fakeSeasonRepo
	shows     *fakeShowRepo
	users     *fakeUserRepo
	books     *fakeAudiobookRepo
	chapters  *fakeChapterRepo
	dismissed *fakeDismissedRepo
}

func progressRouter(d progressDeps) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewProgressService(
		valOr(d.progress, &fakeProgressRepo{}),
		valOr(d.movies, &fakeMovieRepo{}),
		valOr(d.episodes, &fakeEpisodeRepo{}),
		valOr(d.seasons, &fakeSeasonRepo{}),
		valOr(d.shows, &fakeShowRepo{}),
		valOr(d.users, &fakeUserRepo{}),
		valOr(d.books, &fakeAudiobookRepo{}),
		valOr(d.chapters, &fakeChapterRepo{}),
		valOr(d.dismissed, &fakeDismissedRepo{}),
	)
	h := NewProgressHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("claims", &middleware.Claims{UserID: "u1"}) })
	r.GET("/progress", h.Get)
	r.DELETE("/progress", h.Delete)
	r.GET("/progress/all", h.GetAll)
	r.GET("/progress/continue-watching", h.ContinueWatching)
	r.GET("/progress/next-up", h.NextUp)
	r.POST("/progress/next-up/:episode_id/dismiss", h.DismissNextUp)
	r.DELETE("/progress/next-up/:episode_id/dismiss", h.UndismissNextUp)
	r.GET("/tvshows/:id/next-episode", h.NextEpisode)
	r.GET("/admin/active-sessions", h.ActiveSessions)
	r.PUT("/progress/completed", h.SetCompleted)
	r.PUT("/progress/show-completed", h.SetShowCompleted)
	r.GET("/progress/show-states", h.ShowStates)
	r.GET("/progress/show-state", h.ShowState)
	return r
}

func valOr[T any](v *T, fallback *T) *T {
	if v == nil {
		return fallback
	}
	return v
}

func TestProgressHandler_Get(t *testing.T) {
	prog := &fakeProgressRepo{rows: []*models.WatchProgress{
		{UserID: "u1", MediaType: "movie", MediaID: "m1", Position: 12},
	}}
	r := progressRouter(progressDeps{progress: prog})

	t.Run("missing params is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/progress", "")
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
	t.Run("found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/progress?media_type=movie&media_id=m1", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("missing row is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/progress?media_type=movie&media_id=nope", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestProgressHandler_GetAll(t *testing.T) {
	r := progressRouter(progressDeps{})
	t.Run("missing media_type is 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, "/progress/all", "").Code)
	})
	t.Run("present is 200", func(t *testing.T) {
		assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/progress/all?media_type=movie", "").Code)
	})
}

func TestProgressHandler_ContinueWatchingAndNextUp(t *testing.T) {
	r := progressRouter(progressDeps{})
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/progress/continue-watching", "").Code)
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/progress/next-up", "").Code)
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/progress/next-up?limit=5", "").Code)
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/progress/next-up?limit=999", "").Code) // clamped
}

func TestProgressHandler_DismissAndUndismiss(t *testing.T) {
	t.Run("dismiss is 204", func(t *testing.T) {
		r := progressRouter(progressDeps{})
		w := doJSON(r, http.MethodPost, "/progress/next-up/ep1/dismiss", "")
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("undismiss success is 204", func(t *testing.T) {
		r := progressRouter(progressDeps{dismissed: &fakeDismissedRepo{}})
		w := doJSON(r, http.MethodDelete, "/progress/next-up/ep1/dismiss", "")
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("undismiss not-dismissed is 404", func(t *testing.T) {
		r := progressRouter(progressDeps{dismissed: &fakeDismissedRepo{deleteErr: services.ErrNotFound}})
		w := doJSON(r, http.MethodDelete, "/progress/next-up/ep1/dismiss", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestProgressHandler_NextEpisode(t *testing.T) {
	showID := uuid.New()
	season := &models.Season{Base: models.Base{ID: uuid.New()}, TVShowID: showID, Number: 1}
	ep := &models.Episode{Base: models.Base{ID: uuid.New()}, SeasonID: season.ID, Number: 1, FilePath: "/1.mp4"}
	r := progressRouter(progressDeps{
		seasons:  &fakeSeasonRepo{seasons: []*models.Season{season}},
		episodes: &fakeEpisodeRepo{episodes: []*models.Episode{ep}},
	})

	t.Run("found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/tvshows/"+showID.String()+"/next-episode", "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
	t.Run("no episodes is 404", func(t *testing.T) {
		empty := progressRouter(progressDeps{})
		w := doJSON(empty, http.MethodGet, "/tvshows/"+uuid.New().String()+"/next-episode", "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}

func TestProgressHandler_ActiveSessions(t *testing.T) {
	r := progressRouter(progressDeps{})
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/admin/active-sessions", "").Code)
}

func TestProgressHandler_SetCompleted(t *testing.T) {
	r := progressRouter(progressDeps{})
	t.Run("valid is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/progress/completed", `{"media_type":"movie","media_id":"m1","completed":true}`)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("bad media_type is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/progress/completed", `{"media_type":"photo","media_id":"m1"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestProgressHandler_SetShowCompleted(t *testing.T) {
	r := progressRouter(progressDeps{})
	t.Run("valid is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/progress/show-completed", `{"show_id":"`+uuid.New().String()+`","completed":true}`)
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
	t.Run("missing show_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPut, "/progress/show-completed", `{"completed":true}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

func TestProgressHandler_ShowStates(t *testing.T) {
	r := progressRouter(progressDeps{})
	assert.Equal(t, http.StatusOK, doJSON(r, http.MethodGet, "/progress/show-states", "").Code)
}

func TestProgressHandler_ShowState(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}}
	r := progressRouter(progressDeps{shows: &fakeShowRepo{shows: []*models.TVShow{show}}})

	t.Run("missing show_id is 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, "/progress/show-state", "").Code)
	})
	t.Run("present is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/progress/show-state?show_id="+show.ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}

func TestProgressHandler_Delete(t *testing.T) {
	prog := &fakeProgressRepo{rows: []*models.WatchProgress{{UserID: "u1", MediaType: "movie", MediaID: "m1"}}}
	r := progressRouter(progressDeps{progress: prog})

	t.Run("missing params is 400", func(t *testing.T) {
		assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodDelete, "/progress", "").Code)
	})
	t.Run("valid is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/progress?media_type=movie&media_id=m1", "")
		assert.Equal(t, http.StatusNoContent, w.Code)
	})
}
