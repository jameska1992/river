package apiclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// mergeAPIServer stands in for river-api. It records whether a show was created
// and answers the resolve endpoint from a fixed alias -> survivor mapping.
type mergeAPIServer struct {
	shows        []TVShow          // returned by the list endpoint
	aliases      map[string]string // folder_path -> surviving show id
	createCalled bool
}

func (m *mergeAPIServer) start(t *testing.T) *Client {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/tvshows", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(m.shows)
	})
	mux.HandleFunc("POST /api/tvshows", func(w http.ResponseWriter, r *http.Request) {
		m.createCalled = true
		var req TVShowRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		_ = json.NewEncoder(w).Encode(TVShow{
			ID: "new-id", LibraryID: req.LibraryID, Title: req.Title, FolderPath: req.FolderPath,
		})
	})
	mux.HandleFunc("GET /api/tvshows/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		for _, s := range m.shows {
			if s.ID == id {
				_ = json.NewEncoder(w).Encode(s)
				return
			}
		}
		http.Error(w, "not found", http.StatusNotFound)
	})
	mux.HandleFunc("GET /api/admin/tvshows/resolve", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"show_id": m.aliases[r.URL.Query().Get("folder_path")],
		})
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Client{baseURL: srv.URL, token: "t", http: srv.Client()}
}

func TestFindOrCreateTVShow_ResolvesMergedAlias(t *testing.T) {
	// Survivor lives under /media; the absorbed root /truenas was merged into it.
	survivor := TVShow{ID: "survivor-1", LibraryID: "lib", Title: "MyShow", FolderPath: "/media/shows/MyShow"}
	m := &mergeAPIServer{
		shows:   []TVShow{survivor},
		aliases: map[string]string{"/truenas/tv/MyShow": "survivor-1"},
	}
	c := m.start(t)

	// Scanning the absorbed root: Pass 1/2 miss (different folder_path), so the
	// alias resolve must route it to the survivor instead of creating a dup.
	got, err := c.FindOrCreateTVShow("lib", "MyShow", "/truenas/tv/MyShow")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ID != "survivor-1" {
		t.Errorf("expected survivor-1, got %q", got.ID)
	}
	if m.createCalled {
		t.Error("must not create a duplicate when the path resolves to a survivor")
	}
}

func TestFindOrCreateTVShow_CreatesWhenNoAlias(t *testing.T) {
	m := &mergeAPIServer{shows: nil, aliases: map[string]string{}}
	c := m.start(t)

	got, err := c.FindOrCreateTVShow("lib", "Brand New", "/media/shows/Brand New")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !m.createCalled {
		t.Error("a genuinely new folder should be created")
	}
	if got.ID != "new-id" {
		t.Errorf("expected the created show, got %q", got.ID)
	}
}
