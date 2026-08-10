package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"river-api/internal/models"
	"river-api/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubScan stands in for river-scan: it records the last request path + body
// and replies 202, so the proxy handlers can be asserted without a real
// scanner or RabbitMQ.
type stubScan struct {
	srv  *httptest.Server
	path string
	body map[string]any
}

func newStubScan(t *testing.T) *stubScan {
	s := &stubScan{}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.path = r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.body)
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func TestReTranscodeMovie_ProxiesForceEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	scan := newStubScan(t)

	movieID, libID := uuid.New(), uuid.New()
	movies := &fakeMovieRepo{movies: []*models.Movie{{
		Base: models.Base{ID: movieID}, LibraryID: libID, Title: "Foo",
		SourcePath: "/media/movies/Foo/Foo.mkv",
	}}}
	movieSvc := services.NewMovieService(movies, fakeCleanupRepo{})
	h := NewAdminHandler(scan.srv.URL, "", "", "", "", nil, movieSvc, nil)

	r := gin.New()
	r.POST("/movies/:id/re-transcode", h.ReTranscodeMovie)

	w := doJSON(r, http.MethodPost, "/movies/"+movieID.String()+"/re-transcode", "")
	require.Equal(t, http.StatusAccepted, w.Code)

	assert.Equal(t, "/retranscode", scan.path)
	assert.Equal(t, "movie", scan.body["library_type"])
	assert.Equal(t, movieID.String(), scan.body["media_id"])
	assert.Equal(t, libID.String(), scan.body["library_id"])
	assert.Equal(t, "/media/movies/Foo/Foo.mkv", scan.body["file"])
}

func TestReTranscodeMovie_NoSourceIsConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	scan := newStubScan(t)

	movieID := uuid.New()
	movies := &fakeMovieRepo{movies: []*models.Movie{{
		Base: models.Base{ID: movieID}, LibraryID: uuid.New(), Title: "Foo", SourcePath: "",
	}}}
	h := NewAdminHandler(scan.srv.URL, "", "", "", "", nil, services.NewMovieService(movies, fakeCleanupRepo{}), nil)

	r := gin.New()
	r.POST("/movies/:id/re-transcode", h.ReTranscodeMovie)

	w := doJSON(r, http.MethodPost, "/movies/"+movieID.String()+"/re-transcode", "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Empty(t, scan.path, "scanner must not be called when there's no source file")
}

func TestReTranscodeMovie_NoScanURLIsUnavailable(t *testing.T) {
	gin.SetMode(gin.TestMode)
	movieID := uuid.New()
	movies := &fakeMovieRepo{movies: []*models.Movie{{Base: models.Base{ID: movieID}, SourcePath: "/x.mkv"}}}
	h := NewAdminHandler("", "", "", "", "", nil, services.NewMovieService(movies, fakeCleanupRepo{}), nil)

	r := gin.New()
	r.POST("/movies/:id/re-transcode", h.ReTranscodeMovie)

	w := doJSON(r, http.MethodPost, "/movies/"+movieID.String()+"/re-transcode", "")
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestReTranscodeEpisode_ProxiesForceEventWithSeasonName(t *testing.T) {
	gin.SetMode(gin.TestMode)
	scan := newStubScan(t)

	showID, seasonID, epID, libID := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	shows := &fakeShowRepo{shows: []*models.TVShow{{Base: models.Base{ID: showID}, LibraryID: libID, Title: "Show"}}}
	seasons := &fakeSeasonRepo{seasons: []*models.Season{{Base: models.Base{ID: seasonID}, TVShowID: showID, Number: 2}}}
	episodes := &fakeEpisodeRepo{episodes: []*models.Episode{{
		Base: models.Base{ID: epID}, TVShowID: showID, SeasonID: seasonID, Number: 3,
		SourcePath: "/media/shows/Show/Season 2/S02E03.mkv",
	}}}
	tvSvc := services.NewTVShowService(shows, seasons, episodes, fakeCleanupRepo{}, nil)
	h := NewAdminHandler(scan.srv.URL, "", "", "", "", nil, nil, tvSvc)

	r := gin.New()
	r.POST("/tvshows/:id/seasons/:seasonId/episodes/:episodeId/re-transcode", h.ReTranscodeEpisode)

	path := "/tvshows/" + showID.String() + "/seasons/" + seasonID.String() + "/episodes/" + epID.String() + "/re-transcode"
	w := doJSON(r, http.MethodPost, path, "")
	require.Equal(t, http.StatusAccepted, w.Code)

	assert.Equal(t, "/retranscode", scan.path)
	assert.Equal(t, "tvshow", scan.body["library_type"])
	assert.Equal(t, showID.String(), scan.body["media_id"])
	assert.Equal(t, seasonID.String(), scan.body["season_id"])
	assert.Equal(t, "Season 2", scan.body["season_name"])
	assert.Equal(t, libID.String(), scan.body["library_id"])
	assert.Equal(t, "/media/shows/Show/Season 2/S02E03.mkv", scan.body["file"])
}

func TestReTranscodeEpisode_NoSourceIsConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	scan := newStubScan(t)

	showID, seasonID, epID := uuid.New(), uuid.New(), uuid.New()
	shows := &fakeShowRepo{shows: []*models.TVShow{{Base: models.Base{ID: showID}, LibraryID: uuid.New()}}}
	seasons := &fakeSeasonRepo{seasons: []*models.Season{{Base: models.Base{ID: seasonID}, TVShowID: showID, Number: 1}}}
	episodes := &fakeEpisodeRepo{episodes: []*models.Episode{{
		Base: models.Base{ID: epID}, TVShowID: showID, SeasonID: seasonID, Number: 1, SourcePath: "",
	}}}
	tvSvc := services.NewTVShowService(shows, seasons, episodes, fakeCleanupRepo{}, nil)
	h := NewAdminHandler(scan.srv.URL, "", "", "", "", nil, nil, tvSvc)

	r := gin.New()
	r.POST("/tvshows/:id/seasons/:seasonId/episodes/:episodeId/re-transcode", h.ReTranscodeEpisode)

	path := "/tvshows/" + showID.String() + "/seasons/" + seasonID.String() + "/episodes/" + epID.String() + "/re-transcode"
	w := doJSON(r, http.MethodPost, path, "")
	assert.Equal(t, http.StatusConflict, w.Code)
	assert.Empty(t, scan.path, "scanner must not be called when there's no source file")
}
