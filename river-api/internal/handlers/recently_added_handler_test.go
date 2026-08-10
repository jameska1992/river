package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func recentlyAddedRouter(movies *fakeMovieRepo, shows *fakeShowRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	movieSvc := services.NewMovieService(movies, fakeCleanupRepo{})
	tvSvc := services.NewTVShowService(shows, nil, nil, nil, nil)
	h := NewRecentlyAddedHandler(movieSvc, tvSvc)
	r := gin.New()
	r.GET("/recently-added", h.List)
	return r
}

func at(base time.Time, offset time.Duration) models.Base {
	return models.Base{ID: uuid.New(), CreatedAt: base.Add(offset)}
}

func TestRecentlyAdded_InterleavesAndSortsByAddedDesc(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	movieOld := &models.Movie{Base: at(t0, 1*time.Hour), Title: "Old Movie", PosterPath: "/m1.jpg"}
	movieNew := &models.Movie{Base: at(t0, 3*time.Hour), Title: "New Movie", PosterPath: "/m2.jpg"}
	showMid := &models.TVShow{Base: at(t0, 2*time.Hour), Title: "Mid Show", PosterPath: "/s1.jpg"}

	r := recentlyAddedRouter(
		&fakeMovieRepo{movies: []*models.Movie{movieOld, movieNew}},
		&fakeShowRepo{shows: []*models.TVShow{showMid}},
	)

	w := doJSON(r, http.MethodGet, "/recently-added", "")
	require.Equal(t, http.StatusOK, w.Code)

	var items []struct {
		Title     string `json:"title"`
		MediaType string `json:"media_type"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 3)
	// Newest first: New Movie (3h) > Mid Show (2h) > Old Movie (1h).
	assert.Equal(t, "New Movie", items[0].Title)
	assert.Equal(t, "movie", items[0].MediaType)
	assert.Equal(t, "Mid Show", items[1].Title)
	assert.Equal(t, "tvshow", items[1].MediaType)
	assert.Equal(t, "Old Movie", items[2].Title)
}

func TestRecentlyAdded_SkipsArtlessItems(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	withArt := &models.Movie{Base: at(t0, 2*time.Hour), Title: "Has Poster", PosterPath: "/p.jpg"}
	artless := &models.Movie{Base: at(t0, 1*time.Hour), Title: "No Art"} // no poster, no backdrop

	r := recentlyAddedRouter(&fakeMovieRepo{movies: []*models.Movie{withArt, artless}}, &fakeShowRepo{})

	w := doJSON(r, http.MethodGet, "/recently-added", "")
	require.Equal(t, http.StatusOK, w.Code)

	var items []struct {
		Title string `json:"title"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	require.Len(t, items, 1, "an item with neither poster nor backdrop is skipped")
	assert.Equal(t, "Has Poster", items[0].Title)
}

func TestRecentlyAdded_CapsAtTen(t *testing.T) {
	t0 := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	movies := make([]*models.Movie, 0, 12)
	for i := 0; i < 12; i++ {
		movies = append(movies, &models.Movie{Base: at(t0, time.Duration(i)*time.Hour), Title: "M", PosterPath: "/p.jpg"})
	}
	r := recentlyAddedRouter(&fakeMovieRepo{movies: movies}, &fakeShowRepo{})

	w := doJSON(r, http.MethodGet, "/recently-added", "")
	require.Equal(t, http.StatusOK, w.Code)

	var items []any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &items))
	assert.Len(t, items, 10, "the rail is capped at 10 items")
}
