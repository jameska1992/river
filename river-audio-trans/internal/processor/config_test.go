package processor

import (
	"errors"
	"testing"
	"time"

	"river-audio-trans/internal/apiclient"
	"river-audio-trans/internal/transcoder"
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

func TestSettingsCache_UsesMusicBitrateAndCaches(t *testing.T) {
	src := &stubSource{ts: &apiclient.TranscodingSettings{MusicBitrate: 320, AudioBitrate: 192}}
	c := newSettingsCache(src, time.Minute)

	// The audio service reads MusicBitrate, not the video AudioBitrate.
	if got := c.musicBitrate(); got != 320 {
		t.Errorf("musicBitrate = %d, want 320", got)
	}
	c.musicBitrate()
	if src.calls != 1 {
		t.Errorf("expected a single fetch within the TTL, got %d", src.calls)
	}
}

func TestSettingsCache_RefetchesAfterTTL(t *testing.T) {
	src := &stubSource{ts: &apiclient.TranscodingSettings{MusicBitrate: 256}}
	c := newSettingsCache(src, time.Minute)

	c.musicBitrate()
	c.mu.Lock()
	c.fetchedAt = time.Now().Add(-2 * time.Minute)
	c.mu.Unlock()
	c.musicBitrate()

	if src.calls != 2 {
		t.Errorf("expected a re-fetch after the TTL, got %d calls", src.calls)
	}
}

func TestSettingsCache_FallsBackToDefaultOnFirstError(t *testing.T) {
	src := &stubSource{err: errors.New("api down")}
	c := newSettingsCache(src, time.Minute)

	if got := c.musicBitrate(); got != transcoder.DefaultMusicBitrate {
		t.Errorf("first-fetch failure should yield the default %d, got %d",
			transcoder.DefaultMusicBitrate, got)
	}
}

func TestSettingsCache_ServesStaleOnLaterError(t *testing.T) {
	src := &stubSource{ts: &apiclient.TranscodingSettings{MusicBitrate: 192}}
	c := newSettingsCache(src, time.Minute)
	c.musicBitrate()

	c.mu.Lock()
	c.fetchedAt = time.Now().Add(-2 * time.Minute)
	c.mu.Unlock()
	src.err = errors.New("api blip")

	if got := c.musicBitrate(); got != 192 {
		t.Errorf("stale-on-error should serve last good bitrate 192, got %d", got)
	}
}
