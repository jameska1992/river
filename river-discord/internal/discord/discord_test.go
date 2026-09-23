package discord

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"river-discord/internal/embed"
)

func testClient() (*Client, *int32) {
	var slept int32
	c := New()
	c.sleep = func(d time.Duration) { atomic.AddInt32(&slept, 1) } // no real waits
	return c, &slept
}

func TestSend_Success(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ = readAll(r)
		w.WriteHeader(http.StatusNoContent) // Discord's success reply
	}))
	defer srv.Close()

	c, _ := testClient()
	if err := c.Send(srv.URL, embed.Message{Content: "hi"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(body) == 0 {
		t.Fatal("expected a JSON body to be posted")
	}
}

func TestSend_RetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"retry_after":0.01}`))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c, slept := testClient()
	if err := c.Send(srv.URL, embed.Message{Content: "x"}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if atomic.LoadInt32(&calls) != 2 {
		t.Fatalf("expected 2 calls, got %d", calls)
	}
	if atomic.LoadInt32(slept) == 0 {
		t.Fatal("expected a rate-limit wait")
	}
}

func TestSend_PermanentClientError(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	c, _ := testClient()
	if err := c.Send(srv.URL, embed.Message{}); err == nil {
		t.Fatal("expected error for 400")
	}
	if atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("400 must not be retried, got %d calls", calls)
	}
}

func TestSend_RetriesOn5xxThenGivesUp(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := testClient()
	if err := c.Send(srv.URL, embed.Message{}); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != int32(c.maxRetries+1) {
		t.Fatalf("expected %d attempts, got %d", c.maxRetries+1, got)
	}
}

func readAll(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	buf := make([]byte, r.ContentLength)
	_, err := r.Body.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return buf, err
	}
	return buf, nil
}
