package processor

import (
	"errors"
	"testing"
	"time"

	"river-video-trans/internal/apiclient"
	"river-video-trans/internal/transcoder"
)

type stubSource struct {
	ts    *apiclient.TranscodingSettings
	err   error
	calls int
}

func (s *stubSource) GetTranscodingSettings() (*apiclient.TranscodingSettings, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return s.ts, nil
}

func TestSettingsCache_MapsAndCachesWithinTTL(t *testing.T) {
	src := &stubSource{ts: &apiclient.TranscodingSettings{
		MaxHeight: 2160, Quality: 20, NVENCPreset: "p5", X264Preset: "slow",
		ForceCPU: true, AudioBitrate: 256, MusicBitrate: 320,
	}}
	c := newSettingsCache(src, time.Minute)

	got := c.get()
	want := transcoder.Config{
		MaxHeight: 2160, Quality: 20, NVENCPreset: "p5", X264Preset: "slow",
		ForceCPU: true, AudioBitrate: 256,
	}
	if got != want {
		t.Errorf("mapped config = %+v, want %+v", got, want)
	}
	// MusicBitrate is not part of the video transcoder config.
	c.get()
	if src.calls != 1 {
		t.Errorf("expected a single fetch within the TTL, got %d", src.calls)
	}
}

func TestSettingsCache_RefetchesAfterTTL(t *testing.T) {
	src := &stubSource{ts: &apiclient.TranscodingSettings{Quality: 23, AudioBitrate: 192}}
	c := newSettingsCache(src, time.Minute)

	c.get()
	// Force expiry by backdating the last fetch.
	c.mu.Lock()
	c.fetchedAt = time.Now().Add(-2 * time.Minute)
	c.mu.Unlock()
	c.get()

	if src.calls != 2 {
		t.Errorf("expected a re-fetch after the TTL, got %d calls", src.calls)
	}
}

func TestSettingsCache_FallsBackToDefaultOnFirstError(t *testing.T) {
	src := &stubSource{err: errors.New("api down")}
	c := newSettingsCache(src, time.Minute)

	if got := c.get(); got != transcoder.DefaultConfig() {
		t.Errorf("first-fetch failure should yield DefaultConfig, got %+v", got)
	}
}

func TestSettingsCache_ServesStaleOnLaterError(t *testing.T) {
	src := &stubSource{ts: &apiclient.TranscodingSettings{
		MaxHeight: 1080, Quality: 18, NVENCPreset: "p4", X264Preset: "fast", AudioBitrate: 256,
	}}
	c := newSettingsCache(src, time.Minute)
	good := c.get()

	// Expire the cache, then make the next fetch fail — the last good config
	// must be served rather than snapping back to defaults mid-operation.
	c.mu.Lock()
	c.fetchedAt = time.Now().Add(-2 * time.Minute)
	c.mu.Unlock()
	src.err = errors.New("api blip")

	if got := c.get(); got != good {
		t.Errorf("stale-on-error should serve last good config %+v, got %+v", good, got)
	}
}
