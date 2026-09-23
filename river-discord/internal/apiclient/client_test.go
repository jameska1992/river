package apiclient

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetMovie_AuthAndDecode(t *testing.T) {
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"m1","title":"Inception","year":2010,"description":"A thief.","genres":"[\"Action\",\"Sci-Fi\"]","poster_path":"https://img/x.jpg"}`))
	}))
	defer srv.Close()

	c := New(srv.URL, "rvat_test")
	m, err := c.GetMovie("m1")
	if err != nil {
		t.Fatalf("GetMovie: %v", err)
	}
	if gotAuth != "Bearer rvat_test" {
		t.Fatalf("auth header = %q", gotAuth)
	}
	if gotPath != "/movies/m1" {
		t.Fatalf("path = %q", gotPath)
	}
	if m.Title != "Inception" || m.Year != 2010 {
		t.Fatalf("decoded = %+v", m)
	}
	if genres := ParseGenres(m.Genres); len(genres) != 2 || genres[0] != "Action" {
		t.Fatalf("genres = %v", genres)
	}
}

func TestGet_Non200IsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	c := New(srv.URL, "t")
	if _, err := c.GetTVShow("missing"); err == nil {
		t.Fatal("expected error for 404")
	}
}

func TestOtherGetters_Paths(t *testing.T) {
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		_, _ = w.Write([]byte(`{"id":"x"}`))
	}))
	defer srv.Close()
	c := New(srv.URL, "t")

	_, _ = c.GetAlbum("a1")
	if path != "/albums/a1" {
		t.Fatalf("album path = %q", path)
	}
	_, _ = c.GetArtist("ar1")
	if path != "/artists/ar1" {
		t.Fatalf("artist path = %q", path)
	}
	_, _ = c.GetAudiobook("b1")
	if path != "/audiobooks/b1" {
		t.Fatalf("audiobook path = %q", path)
	}
}

func TestParseGenres(t *testing.T) {
	if g := ParseGenres(""); g != nil {
		t.Fatalf("empty = %v", g)
	}
	if g := ParseGenres("not json"); g != nil {
		t.Fatalf("malformed should be nil, got %v", g)
	}
	if g := ParseGenres(`["Drama"]`); len(g) != 1 || g[0] != "Drama" {
		t.Fatalf("got %v", g)
	}
}
