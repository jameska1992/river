package handlers

import (
	"encoding/json"
	"errors"
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

// peopleSearchSetup wires the handler against an httptest stand-in for
// river-meta-movie. meta == nil leaves RIVER_META_MOVIE_URL unset.
func peopleSearchSetup(t *testing.T, meta http.HandlerFunc, repo *fakeCreditsRepo) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	metaURL := ""
	if meta != nil {
		srv := httptest.NewServer(meta)
		t.Cleanup(srv.Close)
		metaURL = srv.URL
	}
	h := NewPeopleSearchHandler(metaURL, services.NewCreditsService(repo))
	r := gin.New()
	r.GET("/people/tmdb/search", h.SearchTMDB)
	r.GET("/people/tmdb/:tmdbId", h.GetTMDBPerson)
	return r
}

func TestSearchTMDB_ProxiesAndMarksLocalPeople(t *testing.T) {
	var gotPath, gotQuery string
	meta := func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.Query().Get("q")
		w.Write([]byte(`[{"tmdb_id":287,"name":"Brad Pitt","known_for":["Fight Club"]},{"tmdb_id":500,"name":"Tom Cruise","known_for":[]}]`))
	}
	pid := uuid.New()
	tmdbID := 287
	repo := &fakeCreditsRepo{byTmdb: []models.Person{{Base: models.Base{ID: pid}, TmdbID: &tmdbID}}}
	r := peopleSearchSetup(t, meta, repo)

	w := doJSON(r, http.MethodGet, "/people/tmdb/search?q=brad+pitt", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/people/search", gotPath)
	assert.Equal(t, "brad pitt", gotQuery)

	var got []ExternalPersonResult
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	require.Len(t, got, 2)
	require.NotNil(t, got[0].PersonID, "existing person is marked")
	assert.Equal(t, pid.String(), *got[0].PersonID)
	assert.Nil(t, got[1].PersonID)
	assert.Equal(t, []string{"Fight Club"}, got[0].KnownFor)
}

func TestSearchTMDB_LocalLookupFailureStillReturnsResults(t *testing.T) {
	meta := func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[{"tmdb_id":287,"name":"Brad Pitt"}]`)) }
	r := peopleSearchSetup(t, meta, &fakeCreditsRepo{byTmdbErr: errors.New("db down")})

	w := doJSON(r, http.MethodGet, "/people/tmdb/search?q=brad", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), `"person_id":null`)
}

func TestSearchTMDB_EmptyResultsIsArray(t *testing.T) {
	meta := func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`[]`)) }
	r := peopleSearchSetup(t, meta, &fakeCreditsRepo{})

	w := doJSON(r, http.MethodGet, "/people/tmdb/search?q=zzz", "")
	require.Equal(t, http.StatusOK, w.Code)
	assert.JSONEq(t, `[]`, w.Body.String())
}

func TestSearchTMDB_Errors(t *testing.T) {
	status := func(code int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { http.Error(w, "upstream says no", code) }
	}
	cases := []struct {
		name   string
		meta   http.HandlerFunc
		target string
		want   int
	}{
		{"missing q", status(http.StatusOK), "/people/tmdb/search?q=%20", http.StatusBadRequest},
		{"meta not configured", nil, "/people/tmdb/search?q=x", http.StatusServiceUnavailable},
		{"no tmdb key passes through", status(http.StatusServiceUnavailable), "/people/tmdb/search?q=x", http.StatusServiceUnavailable},
		{"tmdb down is 502", status(http.StatusBadGateway), "/people/tmdb/search?q=x", http.StatusBadGateway},
		{"unexpected status is 502", status(http.StatusInternalServerError), "/people/tmdb/search?q=x", http.StatusBadGateway},
		{"garbage body is 502", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(`not json`)) }, "/people/tmdb/search?q=x", http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := peopleSearchSetup(t, tc.meta, &fakeCreditsRepo{})
			w := doJSON(r, http.MethodGet, tc.target, "")
			assert.Equal(t, tc.want, w.Code, w.Body.String())
		})
	}
}

func TestSearchTMDB_UnreachableMetaIs502(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close() // nothing listening any more
	gin.SetMode(gin.TestMode)
	h := NewPeopleSearchHandler(srv.URL, services.NewCreditsService(&fakeCreditsRepo{}))
	r := gin.New()
	r.GET("/people/tmdb/search", h.SearchTMDB)

	w := doJSON(r, http.MethodGet, "/people/tmdb/search?q=x", "")
	assert.Equal(t, http.StatusBadGateway, w.Code)
}

func TestGetTMDBPerson(t *testing.T) {
	var gotPath string
	meta := func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		if r.URL.Path != "/people/287" {
			http.Error(w, "person not found", http.StatusNotFound)
			return
		}
		w.Write([]byte(`{"tmdb_id":287,"name":"Brad Pitt","biography":"Bio.","imdb_id":"nm0000093"}`))
	}
	pid := uuid.New()
	tmdbID := 287
	r := peopleSearchSetup(t, meta, &fakeCreditsRepo{byTmdb: []models.Person{{Base: models.Base{ID: pid}, TmdbID: &tmdbID}}})

	w := doJSON(r, http.MethodGet, "/people/tmdb/287", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "/people/287", gotPath)
	var got ExternalPersonDetail
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Bio.", got.Biography)
	require.NotNil(t, got.PersonID)
	assert.Equal(t, pid.String(), *got.PersonID)

	assert.Equal(t, http.StatusNotFound, doJSON(r, http.MethodGet, "/people/tmdb/1", "").Code)
	assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, "/people/tmdb/abc", "").Code)
	assert.Equal(t, http.StatusBadRequest, doJSON(r, http.MethodGet, "/people/tmdb/-5", "").Code)
}
