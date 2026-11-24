package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"

	"river-meta-movie/internal/tmdb"
)

// peopleSource is the slice of the TMDB client the people endpoints use.
type peopleSource interface {
	Search(query string) ([]tmdb.PersonResult, error)
	GetPerson(id int) (*tmdb.PersonDetail, error)
}

type cachedPeople struct {
	*tmdb.PeopleSearchCache
	client *tmdb.Client
}

func (c cachedPeople) GetPerson(id int) (*tmdb.PersonDetail, error) { return c.client.GetPerson(id) }

// registerPeopleRoutes adds the TMDB person lookups river-api proxies for the
// credits editor's picker (#254). TMDB's person namespace is shared by movies
// and TV, so this one service serves both editors. keyFn reports whether a
// TMDB key is configured, so a missing key is a clear 503 rather than an
// opaque upstream 401.
func registerPeopleRoutes(mux *http.ServeMux, src peopleSource, keyFn func() string) {
	mux.HandleFunc("GET /people/search", func(w http.ResponseWriter, r *http.Request) {
		q := strings.TrimSpace(r.URL.Query().Get("q"))
		if q == "" {
			http.Error(w, "q is required", http.StatusBadRequest)
			return
		}
		if keyFn() == "" {
			http.Error(w, "TMDB API key is not configured", http.StatusServiceUnavailable)
			return
		}
		results, err := src.Search(q)
		if err != nil {
			log.Printf("ERROR people search %q: %v", q, err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, results)
	})

	mux.HandleFunc("GET /people/{tmdbId}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.Atoi(r.PathValue("tmdbId"))
		if err != nil || id <= 0 {
			http.Error(w, "invalid tmdb id", http.StatusBadRequest)
			return
		}
		if keyFn() == "" {
			http.Error(w, "TMDB API key is not configured", http.StatusServiceUnavailable)
			return
		}
		p, err := src.GetPerson(id)
		if errors.Is(err, tmdb.ErrNotFound) {
			http.Error(w, "person not found", http.StatusNotFound)
			return
		}
		if err != nil {
			log.Printf("ERROR get person %d: %v", id, err)
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		writeJSON(w, p)
	})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("ERROR encode response: %v", err)
	}
}
