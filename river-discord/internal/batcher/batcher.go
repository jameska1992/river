// Package batcher coalesces a burst of events sharing a key into a single flush
// once the burst goes quiet (debounce), with a size cap so a never-idle stream
// (a large import) still flushes promptly.
package batcher

import (
	"sync"
	"time"

	"river-discord/internal/events"
)

// FlushFunc receives the events accumulated for a key. It runs off the caller's
// goroutine and must be safe for concurrent use.
type FlushFunc func(key string, evs []events.LifecycleEvent)

// maxBatch flushes a group early once it reaches this many events, so a
// continuous stream can't defer the flush indefinitely.
const maxBatch = 25

type Batcher struct {
	window time.Duration
	flush  FlushFunc

	mu     sync.Mutex
	groups map[string]*group
}

type group struct {
	evs   []events.LifecycleEvent
	timer *time.Timer
}

func New(window time.Duration, flush FlushFunc) *Batcher {
	return &Batcher{window: window, flush: flush, groups: map[string]*group{}}
}

// Add appends an event to its key's group and (re)arms the quiet-period timer.
// If the group hits maxBatch it flushes immediately instead of waiting.
func (b *Batcher) Add(key string, e events.LifecycleEvent) {
	b.mu.Lock()
	g := b.groups[key]
	if g == nil {
		g = &group{}
		b.groups[key] = g
	}
	g.evs = append(g.evs, e)

	if len(g.evs) >= maxBatch {
		b.mu.Unlock()
		b.fire(key)
		return
	}
	if g.timer != nil {
		g.timer.Stop()
	}
	g.timer = time.AfterFunc(b.window, func() { b.fire(key) })
	b.mu.Unlock()
}

// fire removes a key's group and flushes it (once — a concurrent timer + size
// trip can't double-flush because the group is deleted under the lock).
func (b *Batcher) fire(key string) {
	b.mu.Lock()
	g := b.groups[key]
	if g == nil {
		b.mu.Unlock()
		return
	}
	if g.timer != nil {
		g.timer.Stop()
	}
	delete(b.groups, key)
	b.mu.Unlock()

	if len(g.evs) > 0 {
		b.flush(key, g.evs)
	}
}

// Close flushes all pending groups immediately (for graceful shutdown).
func (b *Batcher) Close() {
	b.mu.Lock()
	keys := make([]string, 0, len(b.groups))
	for k := range b.groups {
		keys = append(keys, k)
	}
	b.mu.Unlock()
	for _, k := range keys {
		b.fire(k)
	}
}
