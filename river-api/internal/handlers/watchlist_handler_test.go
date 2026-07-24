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
	"github.com/stretchr/testify/require"
)

func watchlistRouter(wl *fakeWatchlistRepo, movies *fakeMovieRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewWatchlistService(wl, movies, nil, nil) // movie-type items only
	h := NewWatchlistHandler(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) { c.Set("claims", &middleware.Claims{UserID: "u1"}) })
	r.POST("/watchlist", h.Add)
	r.DELETE("/watchlist/:id", h.Remove)
	return r
}

func TestWatchlistHandler_Add(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Nosferatu"}
	r := watchlistRouter(&fakeWatchlistRepo{}, &fakeMovieRepo{movies: []*models.Movie{movie}})

	t.Run("invalid media type is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/watchlist", `{"media_type":"podcast","media_id":"`+movie.ID.String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing media_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/watchlist", `{"media_type":"movie"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid add is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/watchlist", `{"media_type":"movie","media_id":"`+movie.ID.String()+`"}`)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestWatchlistHandler_Remove(t *testing.T) {
	item := &models.WatchlistItem{Base: models.Base{ID: uuid.New()}, UserID: "u1", MediaType: "movie", MediaID: uuid.New().String()}
	repo := &fakeWatchlistRepo{items: []*models.WatchlistItem{item}}
	r := watchlistRouter(repo, &fakeMovieRepo{})

	t.Run("existing is 204", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/watchlist/"+item.ID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, repo.items)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/watchlist/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
