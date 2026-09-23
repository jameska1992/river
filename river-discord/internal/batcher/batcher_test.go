package batcher

import (
	"sync"
	"testing"
	"time"

	"river-discord/internal/events"
)

type collector struct {
	mu      sync.Mutex
	flushes map[string][]events.LifecycleEvent
	done    chan string
}

func newCollector() *collector {
	return &collector{flushes: map[string][]events.LifecycleEvent{}, done: make(chan string, 16)}
}

func (c *collector) flush(key string, evs []events.LifecycleEvent) {
	c.mu.Lock()
	c.flushes[key] = evs
	c.mu.Unlock()
	c.done <- key
}

func ev(id string) events.LifecycleEvent {
	return events.LifecycleEvent{Kind: events.KindReady, Type: "tvshow", ParentID: "show1", MediaID: id, Title: id}
}

func TestBatcher_CoalescesWithinWindow(t *testing.T) {
	c := newCollector()
	b := New(30*time.Millisecond, c.flush)

	b.Add("show1", ev("e1"))
	b.Add("show1", ev("e2"))
	b.Add("show1", ev("e3"))

	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("flush never fired")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if got := c.flushes["show1"]; len(got) != 3 {
		t.Fatalf("expected 3 coalesced events, got %d", len(got))
	}
}

func TestBatcher_SeparateKeysFlushIndependently(t *testing.T) {
	c := newCollector()
	b := New(20*time.Millisecond, c.flush)
	b.Add("a", ev("1"))
	b.Add("b", ev("2"))

	got := map[string]bool{}
	for range 2 {
		select {
		case k := <-c.done:
			got[k] = true
		case <-time.After(time.Second):
			t.Fatal("expected two flushes")
		}
	}
	if !got["a"] || !got["b"] {
		t.Fatalf("both keys should flush: %v", got)
	}
}

func TestBatcher_MaxBatchFlushesEarly(t *testing.T) {
	c := newCollector()
	// Long window so only the size cap can trigger the flush.
	b := New(10*time.Second, c.flush)
	for i := 0; i < maxBatch; i++ {
		b.Add("show1", ev(string(rune('a'+i%26))))
	}
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("size cap did not flush early")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if got := len(c.flushes["show1"]); got != maxBatch {
		t.Fatalf("expected %d events, got %d", maxBatch, got)
	}
}

func TestBatcher_CloseFlushesPending(t *testing.T) {
	c := newCollector()
	b := New(10*time.Second, c.flush)
	b.Add("show1", ev("e1"))
	b.Close()
	select {
	case <-c.done:
	case <-time.After(time.Second):
		t.Fatal("Close did not flush pending group")
	}
}
