package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdminHandler_GetStats(t *testing.T) {
	gin.SetMode(gin.TestMode)
	stats := fakeStatsRepo{movies: 10, tvShows: 9, tracks: 0, audiobooks: 3}
	h := NewAdminHandler("", "", "", "", "", stats, nil, nil)
	r := gin.New()
	r.GET("/admin/stats", h.GetStats)

	w := doJSON(r, http.MethodGet, "/admin/stats", "")
	require.Equal(t, http.StatusOK, w.Code)

	var body struct {
		Movies     int64 `json:"movies"`
		TVShows    int64 `json:"tv_shows"`
		Tracks     int64 `json:"tracks"`
		Audiobooks int64 `json:"audiobooks"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, int64(10), body.Movies)
	assert.Equal(t, int64(9), body.TVShows)
	assert.Equal(t, int64(0), body.Tracks)
	assert.Equal(t, int64(3), body.Audiobooks)
}
