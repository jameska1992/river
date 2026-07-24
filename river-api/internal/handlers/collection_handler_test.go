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

func collectionRouter(cols *fakeCollectionRepo, movies *fakeMovieRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	// shows/audiobooks repos are nil — the tests only use movie-type items.
	svc := services.NewCollectionService(cols, movies, nil, nil)
	h := NewCollectionHandler(svc)
	r := gin.New()
	// Stamp claims so Create (which reads the caller's user id) works.
	r.Use(func(c *gin.Context) { c.Set("claims", &middleware.Claims{UserID: "u1"}) })
	r.GET("/collections/:id", h.Get)
	r.POST("/collections", h.Create)
	r.POST("/collections/:id/items", h.AddItem)
	return r
}

func TestCollectionHandler_Create(t *testing.T) {
	r := collectionRouter(&fakeCollectionRepo{}, &fakeMovieRepo{})

	t.Run("missing name is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/collections", `{}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/collections", `{"name":"Favourites"}`)
		assert.Equal(t, http.StatusCreated, w.Code)
	})
}

func TestCollectionHandler_AddItem(t *testing.T) {
	movie := &models.Movie{Base: models.Base{ID: uuid.New()}, Title: "Metropolis"}
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Classics"}
	cols := &fakeCollectionRepo{cols: []*models.Collection{col}}
	movies := &fakeMovieRepo{movies: []*models.Movie{movie}}
	r := collectionRouter(cols, movies)

	base := "/collections/" + col.ID.String() + "/items"

	t.Run("invalid media type is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, base, `{"media_type":"podcast","media_id":"`+movie.ID.String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing media_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, base, `{"media_type":"movie"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("unknown collection is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/collections/"+uuid.New().String()+"/items",
			`{"media_type":"movie","media_id":"`+movie.ID.String()+`"}`)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("unknown media is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, base, `{"media_type":"movie","media_id":"`+uuid.New().String()+`"}`)
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("valid add is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, base, `{"media_type":"movie","media_id":"`+movie.ID.String()+`"}`)
		assert.Equal(t, http.StatusCreated, w.Code)
	})

	t.Run("duplicate is 409", func(t *testing.T) {
		// The previous subtest already added it; adding again conflicts.
		w := doJSON(r, http.MethodPost, base, `{"media_type":"movie","media_id":"`+movie.ID.String()+`"}`)
		assert.Equal(t, http.StatusConflict, w.Code)
	})
}

func TestCollectionHandler_Get(t *testing.T) {
	col := &models.Collection{Base: models.Base{ID: uuid.New()}, Name: "Classics"}
	r := collectionRouter(&fakeCollectionRepo{cols: []*models.Collection{col}}, &fakeMovieRepo{})

	t.Run("found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/collections/"+col.ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/collections/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})
}
