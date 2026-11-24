package tmdb

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := New(func() string { return "k" }, "https://img/t/p/original")
	c.apiBase = srv.URL
	return c
}

func TestSearchPeople(t *testing.T) {
	var gotPath, gotQuery, gotKey string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query().Get("query")
		gotKey = r.URL.Query().Get("api_key")
		w.Write([]byte(`{"results":[
			{"id":287,"name":"Brad Pitt","profile_path":"/bp.jpg","known_for_department":"Acting",
			 "known_for":[{"title":"Fight Club"},{"name":"Friends"},{"title":"Se7en"},{"title":"Troy"}]},
			{"id":9,"name":"No Photo","profile_path":null,"known_for":[]}
		]}`))
	})

	got, err := c.SearchPeople("brad pitt")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/search/person" || gotQuery != "brad pitt" || gotKey != "k" {
		t.Fatalf("request = %s q=%q key=%q", gotPath, gotQuery, gotKey)
	}
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
	bp := got[0]
	if bp.TmdbID != 287 || bp.Name != "Brad Pitt" || bp.KnownForDepartment != "Acting" {
		t.Errorf("hit = %+v", bp)
	}
	if bp.ProfilePath != "https://img/t/p/original/bp.jpg" {
		t.Errorf("profile = %q", bp.ProfilePath)
	}
	// Movie titles and TV names both count; capped at maxKnownFor.
	if want := []string{"Fight Club", "Friends", "Se7en"}; len(bp.KnownFor) != 3 || bp.KnownFor[1] != want[1] || bp.KnownFor[2] != want[2] {
		t.Errorf("known_for = %v, want %v", bp.KnownFor, want)
	}
	if got[1].ProfilePath != "" || got[1].KnownFor == nil {
		t.Errorf("empty hit = %+v (want blank profile, non-nil known_for)", got[1])
	}
}

func TestSearchPeople_UpstreamError(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	})
	if _, err := c.SearchPeople("x"); err == nil {
		t.Fatal("want error on 401")
	}
}

func TestGetPerson(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/person/287" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"id":287,"name":"Brad Pitt","profile_path":"/bp.jpg","biography":"Bio.",
			"known_for_department":"Acting","birthday":"1963-12-18","place_of_birth":"Shawnee","imdb_id":"nm0000093"}`))
	})

	p, err := c.GetPerson(287)
	if err != nil {
		t.Fatal(err)
	}
	if p.Biography != "Bio." || p.IMDBID != "nm0000093" || p.ProfilePath != "https://img/t/p/original/bp.jpg" {
		t.Errorf("person = %+v", p)
	}

	if _, err := c.GetPerson(1); !errors.Is(err, ErrNotFound) {
		t.Errorf("missing person err = %v, want ErrNotFound", err)
	}
}

func TestPeopleSearchCache(t *testing.T) {
	calls := 0
	fail := false
	cache := NewPeopleSearchCache(func(q string) ([]PersonResult, error) {
		calls++
		if fail {
			return nil, errors.New("boom")
		}
		return []PersonResult{{Name: q}}, nil
	}, time.Minute, 2)
	now := time.Unix(0, 0)
	cache.now = func() time.Time { return now }

	cache.Search("Brad Pitt")
	cache.Search("  brad   PITT ")
	if calls != 1 {
		t.Fatalf("calls = %d, want 1 (normalised key hit)", calls)
	}

	now = now.Add(2 * time.Minute)
	cache.Search("brad pitt")
	if calls != 2 {
		t.Fatalf("calls = %d, want 2 after expiry", calls)
	}

	fail = true
	if _, err := cache.Search("new"); err == nil {
		t.Fatal("want error passed through")
	}
	fail = false
	cache.Search("new")
	if calls != 4 {
		t.Fatalf("calls = %d, want 4 (errors not cached)", calls)
	}

	// Full (2 live entries): a third distinct query still caches without growing past max.
	cache.Search("third")
	if len(cache.entries) > 2 {
		t.Errorf("entries = %d, want <= 2", len(cache.entries))
	}
}
