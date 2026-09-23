// Package server is river-discord's HTTP receiver for River's outbound webhooks
// (#195). It verifies the HMAC signature, parses the lifecycle event, and hands
// relevant events to a Handler. It holds no Discord/river-api logic itself —
// later phases plug that in via the Handler.
package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"river-discord/internal/events"
)

// Handler processes a verified, relevant lifecycle event. It runs off the
// request path (the receiver has already acked), so it must not block the
// caller and should be safe for concurrent use.
type Handler func(events.LifecycleEvent)

// maxBody caps the request body — lifecycle envelopes are tiny.
const maxBody = 1 << 20 // 1 MiB

type Server struct {
	secret string
	path   string
	handle Handler
	srv    *http.Server
}

// New builds the receiver. handle is invoked for each verified event whose kind
// is a known lifecycle kind; pass a no-op to only validate.
func New(addr, path, secret string, handle Handler) *Server {
	s := &Server{secret: secret, path: path, handle: handle}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST "+path, s.receive)
	s.srv = &http.Server{Addr: addr, Handler: mux}
	return s
}

// ListenAndServe blocks serving requests.
func (s *Server) ListenAndServe() error { return s.srv.ListenAndServe() }

// Shutdown gracefully stops the server.
func (s *Server) Shutdown() error { return s.srv.Close() }

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = io.WriteString(w, "ok")
}

// receive verifies the signature, parses the event, and dispatches it. It acks
// with 202 as soon as the event is accepted so the delivery worker isn't held
// on downstream work (and won't retry a slow-but-successful receive).
func (s *Server) receive(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBody))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	if !validSignature(r.Header.Get(signatureHeader), s.secret, body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var e events.LifecycleEvent
	if err := json.Unmarshal(body, &e); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	// Ignore unknown kinds so new event types don't error the sender; still ack.
	if !events.IsLifecycleKind(e.Kind) {
		w.WriteHeader(http.StatusAccepted)
		return
	}

	if s.handle != nil {
		s.handle(e)
	}
	log.Printf("INFO received %s media_id=%s %q", e.RoutingKey(), e.MediaID, e.Title)
	w.WriteHeader(http.StatusAccepted)
}
