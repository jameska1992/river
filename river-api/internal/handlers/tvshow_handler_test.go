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

func tvShowRouter(shows *fakeShowRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	// seasons/episodes/cleanup nil — CreateShow/GetShow only touch `shows`.
	svc := services.NewTVShowService(shows, nil, nil, nil)
	h := NewTVShowHandler(svc, "", "")
	r := gin.New()
	r.POST("/tvshows", h.CreateShow)
	r.GET("/tvshows/:id", h.GetShow)
	return r
}

func TestTVShowHandler_CreateShow(t *testing.T) {
	repo := &fakeShowRepo{}
	r := tvShowRouter(repo)

	t.Run("missing title is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/tvshows", `{"library_id":"`+uuid.New().String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non-uuid library_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/tvshows", `{"library_id":"nope","title":"X"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/tvshows", `{"library_id":"`+uuid.New().String()+`","title":"Dragnet"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.shows, 1)
	})
}

func TestTVShowHandler_GetShow(t *testing.T) {
	show := &models.TVShow{Base: models.Base{ID: uuid.New()}, Title: "Dragnet"}
	r := tvShowRouter(&fakeShowRepo{shows: []*models.TVShow{show}})

	t.Run("found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/tvshows/"+show.ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/tvshows/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
