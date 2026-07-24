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

func musicRouter(artists *fakeArtistRepo, albums *fakeAlbumRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := services.NewMusicService(artists, albums, nil) // tracks repo unused here
	h := NewMusicHandler(svc)
	r := gin.New()
	r.POST("/artists", h.CreateArtist)
	r.GET("/artists/:id", h.GetArtist)
	r.DELETE("/artists/:id", h.DeleteArtist)
	r.POST("/albums", h.CreateAlbum)
	r.GET("/albums/:id", h.GetAlbum)
	return r
}

func TestMusicHandler_CreateArtist(t *testing.T) {
	repo := &fakeArtistRepo{}
	r := musicRouter(repo, &fakeAlbumRepo{})

	t.Run("missing name is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/artists", `{"library_id":"`+uuid.New().String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("non-uuid library_id is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/artists", `{"library_id":"nope","name":"Miles Davis"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/artists", `{"library_id":"`+uuid.New().String()+`","name":"Miles Davis"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		assert.Len(t, repo.artists, 1)
	})
}

func TestMusicHandler_GetAndDeleteArtist(t *testing.T) {
	artist := &models.Artist{Base: models.Base{ID: uuid.New()}, Name: "Miles Davis"}
	repo := &fakeArtistRepo{artists: []*models.Artist{artist}}
	r := musicRouter(repo, &fakeAlbumRepo{})

	t.Run("get found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/artists/"+artist.ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})

	t.Run("get missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/artists/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("delete is 204 and removes the row", func(t *testing.T) {
		w := doJSON(r, http.MethodDelete, "/artists/"+artist.ID.String(), "")
		require.Equal(t, http.StatusNoContent, w.Code)
		assert.Empty(t, repo.artists)
	})
}

func TestMusicHandler_CreateAndGetAlbum(t *testing.T) {
	albums := &fakeAlbumRepo{}
	r := musicRouter(&fakeArtistRepo{}, albums)

	t.Run("missing title is 400", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/albums", `{"library_id":"`+uuid.New().String()+`"}`)
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("valid is 201", func(t *testing.T) {
		w := doJSON(r, http.MethodPost, "/albums",
			`{"library_id":"`+uuid.New().String()+`","artist_id":"`+uuid.New().String()+`","title":"Kind of Blue"}`)
		require.Equal(t, http.StatusCreated, w.Code)
		require.Len(t, albums.albums, 1)
	})

	t.Run("get missing is 404", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/albums/"+uuid.New().String(), "")
		assert.Equal(t, http.StatusNotFound, w.Code)
	})

	t.Run("get found is 200", func(t *testing.T) {
		w := doJSON(r, http.MethodGet, "/albums/"+albums.albums[0].ID.String(), "")
		assert.Equal(t, http.StatusOK, w.Code)
	})
}
