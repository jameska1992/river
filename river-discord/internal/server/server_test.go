package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"river-discord/internal/events"
)

const testSecret = "whsec_test"

// signed builds a POST request to path with a valid signature for body.
func signed(path, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set(signatureHeader, "sha256="+sign(testSecret, []byte(body)))
	return req
}

func newTestServer(handle Handler) *Server {
	return New(":0", "/hooks/river", testSecret, handle)
}

func TestReceive_ValidEventDispatched(t *testing.T) {
	var (
		mu  sync.Mutex
		got []events.LifecycleEvent
	)
	s := newTestServer(func(e events.LifecycleEvent) {
		mu.Lock()
		got = append(got, e)
		mu.Unlock()
	})

	body := `{"schema_version":1,"kind":"media.ready","type":"movie","media_id":"m1","title":"Inception"}`
	w := httptest.NewRecorder()
	s.receive(w, signed("/hooks/river", body))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || got[0].MediaID != "m1" || got[0].Kind != events.KindReady {
		t.Fatalf("handler got %+v", got)
	}
}

func TestReceive_BadSignatureRejected(t *testing.T) {
	called := false
	s := newTestServer(func(events.LifecycleEvent) { called = true })

	body := `{"kind":"media.ready","type":"movie","media_id":"m1"}`
	req := httptest.NewRequest(http.MethodPost, "/hooks/river", strings.NewReader(body))
	req.Header.Set(signatureHeader, "sha256=deadbeef")
	w := httptest.NewRecorder()
	s.receive(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", w.Code)
	}
	if called {
		t.Fatal("handler must not run for a bad signature")
	}
}

func TestReceive_MalformedJSON(t *testing.T) {
	s := newTestServer(func(events.LifecycleEvent) { t.Fatal("handler must not run") })
	w := httptest.NewRecorder()
	s.receive(w, signed("/hooks/river", `{not json`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
}

func TestReceive_UnknownKindAckedButIgnored(t *testing.T) {
	called := false
	s := newTestServer(func(events.LifecycleEvent) { called = true })

	// A discovered/unknown kind is acked (202) so the sender doesn't retry, but
	// the notifier takes no action on it.
	body := `{"kind":"media.discovered","type":"movie","media_id":"m1"}`
	w := httptest.NewRecorder()
	s.receive(w, signed("/hooks/river", body))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	if called {
		t.Fatal("handler must not run for an unknown kind")
	}
}

func TestReceive_RoutesAndHealthz(t *testing.T) {
	s := newTestServer(nil)

	// Healthz.
	hw := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(hw, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if hw.Code != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", hw.Code)
	}

	// Valid POST routed through the mux (nil handler is tolerated).
	body := `{"kind":"media.ready","type":"movie","media_id":"m1"}`
	pw := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(pw, signed("/hooks/river", body))
	if pw.Code != http.StatusAccepted {
		t.Fatalf("post status = %d, want 202", pw.Code)
	}
}
