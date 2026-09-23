package webhooks

import (
	"sync"
	"time"

	"river-events/internal/apiclient"
)

// Cache holds the active-webhook list fetched from river-api, refreshed at most
// once per TTL so the hot path (every lifecycle event) doesn't hit river-api
// each time. On a refresh error it serves the last good snapshot.
type Cache struct {
	fetch func() ([]apiclient.ActiveWebhook, error)
	ttl   time.Duration

	mu     sync.Mutex
	cached []apiclient.ActiveWebhook
	at     time.Time
	loaded bool
}

func NewCache(fetch func() ([]apiclient.ActiveWebhook, error), ttl time.Duration) *Cache {
	return &Cache{fetch: fetch, ttl: ttl}
}

// ActiveWebhooks returns the cached list, refreshing if stale. A refresh failure
// returns the previous snapshot (and no error) so a transient river-api blip
// doesn't stall delivery — a truly empty first load returns the error.
func (c *Cache) ActiveWebhooks() ([]apiclient.ActiveWebhook, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded && time.Since(c.at) < c.ttl {
		return c.cached, nil
	}
	hooks, err := c.fetch()
	if err != nil {
		if c.loaded {
			return c.cached, nil
		}
		return nil, err
	}
	c.cached = hooks
	c.at = time.Now()
	c.loaded = true
	return hooks, nil
}
