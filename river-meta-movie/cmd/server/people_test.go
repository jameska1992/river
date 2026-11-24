package main

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"river-meta-movie/internal/tmdb"
)

type fakePeople struct {
	query   string
	results []tmdb.PersonResult
	person  *tmdb.PersonDetail
	err     error
}

func (f *fakePeople) Search(q string) ([]tmdb.PersonResult, error) {
	f.query = q
	return f.results, f.err
}

func (f *fakePeople) GetPerson(id int) (*tmdb.PersonDetail, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.person == nil || f.person.TmdbID != id {
		return nil, fmt.Errorf("%w: person %d", tmdb.ErrNotFound, id)
	}
	return f.person, nil
}

func serve(src peopleSource, key, target string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	registerPeopleRoutes(mux, src, func() string { return key })
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func TestPeopleSearchRoute(t *testing.T) {
	src := &fakePeople{results: []tmdb.PersonResult{{TmdbID: 287, Name: "Brad Pitt", KnownFor: []string{"Fight Club"}}}}
	w := serve(src, "k", "/people/search?q=%20brad%20")
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", w.Code, w.Body)
	}
	if src.query != "brad" {
		t.Errorf("query = %q, want trimmed", src.query)
	}
	if !strings.Contains(w.Body.String(), `"tmdb_id":287`) || w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("body = %s", w.Body)
	}
}

func TestPeopleSearchRoute_Errors(t *testing.T) {
	cases := []struct {
		name, key, target string
		err               error
		want              int
	}{
		{"missing q", "k", "/people/search?q=", nil, http.StatusBadRequest},
		{"no key", "", "/people/search?q=x", nil, http.StatusServiceUnavailable},
		{"upstream", "k", "/people/search?q=x", errors.New("tmdb down"), http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if w := serve(&fakePeople{err: tc.err}, tc.key, tc.target); w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}

func TestPersonRoute(t *testing.T) {
	src := &fakePeople{person: &tmdb.PersonDetail{TmdbID: 287, Name: "Brad Pitt", Biography: "Bio."}}
	w := serve(src, "k", "/people/287")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"biography":"Bio."`) {
		t.Fatalf("status = %d body = %s", w.Code, w.Body)
	}

	cases := []struct {
		name, key, target string
		err               error
		want              int
	}{
		{"bad id", "k", "/people/abc", nil, http.StatusBadRequest},
		{"zero id", "k", "/people/0", nil, http.StatusBadRequest},
		{"no key", "", "/people/287", nil, http.StatusServiceUnavailable},
		{"not found", "k", "/people/1", nil, http.StatusNotFound},
		{"upstream", "k", "/people/287", errors.New("tmdb down"), http.StatusBadGateway},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := &fakePeople{person: src.person, err: tc.err}
			if w := serve(s, tc.key, tc.target); w.Code != tc.want {
				t.Errorf("status = %d, want %d", w.Code, tc.want)
			}
		})
	}
}
