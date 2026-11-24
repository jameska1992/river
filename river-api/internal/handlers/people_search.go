package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"river-api/internal/services"

	"github.com/gin-gonic/gin"
)

// PeopleSearchHandler looks people up in TMDB for the credits editor's picker
// (#254). river-api holds no TMDB client of its own: like identify/refresh it
// proxies to river-meta-movie, whose TMDB person endpoints serve both the
// movie and TV editors (TMDB's person namespace is shared).
type PeopleSearchHandler struct {
	metaMovieURL string
	http         *http.Client
	credits      *services.CreditsService
}

func NewPeopleSearchHandler(metaMovieURL string, credits *services.CreditsService) *PeopleSearchHandler {
	return &PeopleSearchHandler{
		metaMovieURL: metaMovieURL,
		http:         &http.Client{Timeout: 10 * time.Second},
		credits:      credits,
	}
}

// ExternalPersonResult is one TMDB person search hit. PersonID is set when
// the person is already in the library (matched on tmdb_id).
type ExternalPersonResult struct {
	TmdbID             int      `json:"tmdb_id"`
	Name               string   `json:"name"`
	ProfilePath        string   `json:"profile_path"`
	KnownForDepartment string   `json:"known_for_department"`
	KnownFor           []string `json:"known_for"`
	PersonID           *string  `json:"person_id"`
}

// ExternalPersonDetail is a single TMDB person.
type ExternalPersonDetail struct {
	TmdbID             int     `json:"tmdb_id"`
	Name               string  `json:"name"`
	ProfilePath        string  `json:"profile_path"`
	Biography          string  `json:"biography"`
	KnownForDepartment string  `json:"known_for_department"`
	Birthday           string  `json:"birthday"`
	PlaceOfBirth       string  `json:"place_of_birth"`
	IMDBID             string  `json:"imdb_id"`
	PersonID           *string `json:"person_id"`
}

// SearchTMDB searches TMDB for people by name.
//
// @Summary      Search TMDB people
// @Description  Searches TMDB (via river-meta-movie) for cast/crew by name. Hits already in the library carry their person_id.
// @Tags         credits
// @Produce      json
// @Param        q    query  string  true  "Name to search for"
// @Success      200  {array}   ExternalPersonResult
// @Failure      400  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /people/tmdb/search [get]
func (h *PeopleSearchHandler) SearchTMDB(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q is required"})
		return
	}
	var results []ExternalPersonResult
	if !h.proxy(c, "/people/search?q="+url.QueryEscape(q), &results) {
		return
	}

	ids := make([]int, len(results))
	for i, r := range results {
		ids[i] = r.TmdbID
	}
	// Best-effort: a failed lookup only loses the "in library" marker.
	if local, err := h.credits.LocalPersonIDsByTmdbID(ids); err != nil {
		log.Printf("WARN people search: local match: %v", err)
	} else {
		for i := range results {
			if pid, ok := local[results[i].TmdbID]; ok {
				results[i].PersonID = &pid
			}
		}
	}
	if results == nil {
		results = []ExternalPersonResult{}
	}
	c.JSON(http.StatusOK, results)
}

// GetTMDBPerson fetches one TMDB person (biography, profile, …).
//
// @Summary      Get TMDB person
// @Tags         credits
// @Produce      json
// @Param        tmdbId  path  int  true  "TMDB person ID"
// @Success      200  {object}  ExternalPersonDetail
// @Failure      400  {object}  map[string]string
// @Failure      404  {object}  map[string]string
// @Failure      502  {object}  map[string]string
// @Failure      503  {object}  map[string]string
// @Security     BearerAuth
// @Router       /people/tmdb/{tmdbId} [get]
func (h *PeopleSearchHandler) GetTMDBPerson(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("tmdbId"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid tmdb id"})
		return
	}
	var p ExternalPersonDetail
	if !h.proxy(c, fmt.Sprintf("/people/%d", id), &p) {
		return
	}
	if local, err := h.credits.LocalPersonIDsByTmdbID([]int{id}); err != nil {
		log.Printf("WARN tmdb person %d: local match: %v", id, err)
	} else if pid, ok := local[id]; ok {
		p.PersonID = &pid
	}
	c.JSON(http.StatusOK, p)
}

// proxy GETs path from river-meta-movie and decodes a 200 into out. On any
// other outcome it writes the error response and returns false: 400/404/503
// from the meta service pass through (bad input, unknown person, no TMDB
// key); anything else is a 502.
func (h *PeopleSearchHandler) proxy(c *gin.Context, path string, out any) bool {
	if h.metaMovieURL == "" {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "RIVER_META_MOVIE_URL is not configured"})
		return false
	}
	resp, err := h.http.Get(h.metaMovieURL + path)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "could not reach metadata service: " + err.Error()})
		return false
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			c.JSON(http.StatusBadGateway, gin.H{"error": "invalid response from metadata service"})
			return false
		}
		return true
	case http.StatusBadRequest, http.StatusNotFound, http.StatusServiceUnavailable:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		c.JSON(resp.StatusCode, gin.H{"error": strings.TrimSpace(string(msg))})
	default:
		c.JSON(http.StatusBadGateway, gin.H{"error": fmt.Sprintf("metadata service returned %d", resp.StatusCode)})
	}
	return false
}
