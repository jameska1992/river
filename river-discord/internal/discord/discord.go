// Package discord posts messages to a Discord incoming-webhook URL, honouring
// Discord's rate limits (429 + Retry-After) and retrying transient failures.
package discord

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"river-discord/internal/embed"
)

type Client struct {
	http       *http.Client
	maxRetries int
	// sleep is time.Sleep in production; overridden in tests to avoid real waits.
	sleep func(time.Duration)
}

func New() *Client {
	return &Client{
		http:       &http.Client{Timeout: 15 * time.Second},
		maxRetries: 3,
		sleep:      time.Sleep,
	}
}

// Send posts msg to a Discord webhook URL. It retries on 429 (waiting the
// server-specified Retry-After) and on 5xx/transport errors with a short
// backoff. A non-2xx 4xx (other than 429) is a permanent error and isn't
// retried. Returns nil on any 2xx (Discord replies 204).
func (c *Client) Send(url string, msg embed.Message) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 0; attempt <= c.maxRetries; attempt++ {
		code, retryAfter, err := c.post(url, body)
		switch {
		case err == nil && code >= 200 && code < 300:
			return nil
		case code == http.StatusTooManyRequests:
			lastErr = fmt.Errorf("rate limited (429)")
			c.sleep(retryAfter)
		case err == nil && code >= 400 && code < 500:
			// Permanent client error (bad URL/payload) — don't retry.
			return fmt.Errorf("discord webhook rejected: status %d", code)
		default:
			// 5xx or transport error — retry with a short backoff.
			if err != nil {
				lastErr = err
			} else {
				lastErr = fmt.Errorf("discord webhook error: status %d", code)
			}
			c.sleep(backoff(attempt))
		}
	}
	return fmt.Errorf("giving up after %d attempts: %w", c.maxRetries+1, lastErr)
}

// post performs one POST. On 429 it returns the Retry-After duration.
func (c *Client) post(url string, body []byte) (code int, retryAfter time.Duration, err error) {
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return resp.StatusCode, parseRetryAfter(resp), nil
	}
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode, 0, nil
}

// parseRetryAfter reads Discord's rate-limit hint: a JSON body {"retry_after":
// <seconds>} (float), falling back to the Retry-After header, then a default.
func parseRetryAfter(resp *http.Response) time.Duration {
	var payload struct {
		RetryAfter float64 `json:"retry_after"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err := json.Unmarshal(body, &payload); err == nil && payload.RetryAfter > 0 {
		return time.Duration(payload.RetryAfter * float64(time.Second))
	}
	if h := resp.Header.Get("Retry-After"); h != "" {
		if secs, err := strconv.ParseFloat(h, 64); err == nil && secs > 0 {
			return time.Duration(secs * float64(time.Second))
		}
	}
	return time.Second
}

func backoff(attempt int) time.Duration {
	return time.Duration(attempt+1) * 250 * time.Millisecond
}
