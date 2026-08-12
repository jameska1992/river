package openlib

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newTestClient points the client at an httptest server so the work/author
// JSON parsing can be exercised without hitting the real Open Library.
func newTestClient(baseURL string) *Client {
	return &Client{http: &http.Client{}, baseURL: baseURL}
}

func TestFetchByWorkKey_ResolvesWorkAndAuthor(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/works/OL45804W.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{
			"title": "Dune",
			"description": {"type": "/type/text", "value": "  Desert planet.  "},
			"subjects": ["Science fiction", "Adventure"],
			"covers": [8451080],
			"authors": [{"author": {"key": "/authors/OL79034A"}, "type": {"key": "/type/author_role"}}]
		}`))
	})
	mux.HandleFunc("/authors/OL79034A.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"name": "Frank Herbert"}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	meta, err := newTestClient(srv.URL).FetchByWorkKey("/works/OL45804W")
	if err != nil {
		t.Fatalf("FetchByWorkKey: %v", err)
	}
	if meta.Title != "Dune" {
		t.Errorf("title = %q, want Dune", meta.Title)
	}
	if meta.Author != "Frank Herbert" {
		t.Errorf("author = %q, want Frank Herbert", meta.Author)
	}
	if meta.Description != "Desert planet." {
		t.Errorf("description = %q (should be trimmed value)", meta.Description)
	}
	if meta.Genre != "Science fiction" {
		t.Errorf("genre = %q, want first subject", meta.Genre)
	}
	if meta.CoverURL != "https://covers.openlibrary.org/b/id/8451080-L.jpg" {
		t.Errorf("cover = %q", meta.CoverURL)
	}
	if meta.WorkKey != "/works/OL45804W" {
		t.Errorf("workKey = %q", meta.WorkKey)
	}
	// Year and ISBNs aren't on the work endpoint — must be zero so the caller
	// preserves the existing record values rather than blanking them.
	if meta.Year != 0 {
		t.Errorf("year = %d, want 0 (not available by-key)", meta.Year)
	}
	if len(meta.ISBNs) != 0 {
		t.Errorf("isbns = %v, want empty (not available by-key)", meta.ISBNs)
	}
}

func TestFetchByWorkKey_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := newTestClient(srv.URL).FetchByWorkKey("/works/OLmissingW")
	if err == nil {
		t.Fatal("expected error for a missing work")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestFetchByWorkKey_MissingAuthorIsBlank(t *testing.T) {
	// A work with no resolvable author yields an empty author (caller keeps the
	// existing value); it must not fail the whole fetch.
	mux := http.NewServeMux()
	mux.HandleFunc("/works/OL1W.json", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"title": "Untitled", "subjects": [], "covers": [], "authors": []}`))
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	meta, err := newTestClient(srv.URL).FetchByWorkKey("/works/OL1W")
	if err != nil {
		t.Fatalf("FetchByWorkKey: %v", err)
	}
	if meta.Author != "" || meta.CoverURL != "" || meta.Genre != "" {
		t.Errorf("expected empty author/cover/genre, got %+v", meta)
	}
}
