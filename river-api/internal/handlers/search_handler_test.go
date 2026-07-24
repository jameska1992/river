package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func searchRouter(repo *fakeSearchRepo) *gin.Engine {
	gin.SetMode(gin.TestMode)
	h := NewSearchHandler(services.NewSearchService(repo))
	r := gin.New()
	r.GET("/search", h.Search)
	return r
}

func TestSearchHandler_EmptyQueryShortCircuits(t *testing.T) {
	// Seed results that must NOT appear — with no q and no genre the
	// handler returns empty without calling the service.
	repo := &fakeSearchRepo{movies: []models.Movie{{Base: models.Base{ID: uuid.New()}, Title: "Should not show"}}}
	r := searchRouter(repo)

	w := doJSON(r, http.MethodGet, "/search", "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Libraries []any `json:"libraries"`
		People    []any `json:"people"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Empty(t, body.Libraries)
	assert.Empty(t, body.People)
}

func TestSearchHandler_ReturnsGroupedResults(t *testing.T) {
	repo := &fakeSearchRepo{movies: []models.Movie{{
		Base: models.Base{ID: uuid.New()}, LibraryID: uuid.New(),
		Library: models.Library{Name: "Films", Type: models.LibraryType("movie")}, Title: "Metropolis",
	}}}
	r := searchRouter(repo)

	w := doJSON(r, http.MethodGet, "/search?q=metro", "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Libraries []struct {
			LibraryName string `json:"library_name"`
			Items       []struct {
				Title     string `json:"title"`
				MediaType string `json:"media_type"`
			} `json:"items"`
		} `json:"libraries"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Libraries, 1)
	assert.Equal(t, "Films", body.Libraries[0].LibraryName)
	require.Len(t, body.Libraries[0].Items, 1)
	assert.Equal(t, "Metropolis", body.Libraries[0].Items[0].Title)
	assert.Equal(t, "movie", body.Libraries[0].Items[0].MediaType)
}
