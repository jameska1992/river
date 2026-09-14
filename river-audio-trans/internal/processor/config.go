package processor

import (
	"sync"
	"time"

	"river-audio-trans/internal/apiclient"
	"river-audio-trans/internal/transcoder"
)

// settingsTTL is how long a fetched transcoding config is reused before
// the next job re-fetches. Short enough that an admin's change lands on
// the next job (no service restart), long enough that a burst of jobs
// doesn't hammer river-api.
const settingsTTL = 30 * time.Second

// settingsSource is the slice of apiclient the cache needs; an interface
// so tests can substitute a stub without a live API.
type settingsSource interface {
	GetTranscodingSettings() (*apiclient.TranscodingSettings, error)
}

// audioSettings is the subset of the shared transcoding config this service
// uses: the AAC bitrate plus the output-validation knobs.
type audioSettings struct {
	musicBitrate         int
	validateOutput       bool
	durationTolerancePct int
}

// defaultAudioSettings mirrors the river-api defaults, used when no config
// has ever been fetched — validation on so broken transcodes are rejected
// out of the box.
func defaultAudioSettings() audioSettings {
	return audioSettings{
		musicBitrate:         transcoder.DefaultMusicBitrate,
		validateOutput:       true,
		durationTolerancePct: 2,
	}
}

// settingsCache serves the audio transcoding config with a short TTL. A
// fetch failure never breaks a job: it serves the last good value if it has
// one, otherwise the compiled-in defaults — so a transient river-api outage
// leaves transcoding running on the previous (or default) config.
type settingsCache struct {
	src settingsSource
	ttl time.Duration

	mu        sync.Mutex
	cached    audioSettings
	have      bool
	fetchedAt time.Time
}

func newSettingsCache(src settingsSource, ttl time.Duration) *settingsCache {
	return &settingsCache{src: src, ttl: ttl}
}

// get returns the effective config, refreshing from river-api when the
// cached value is missing or older than the TTL.
func (s *settingsCache) get() audioSettings {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.have && time.Since(s.fetchedAt) < s.ttl {
		return s.cached
	}

	ts, err := s.src.GetTranscodingSettings()
	if err != nil {
		if s.have {
			return s.cached // serve the last good value on a transient failure
		}
		return defaultAudioSettings()
	}

	s.cached = audioSettings{
		musicBitrate:         ts.MusicBitrate,
		validateOutput:       ts.ValidateOutput,
		durationTolerancePct: ts.DurationTolerancePct,
	}
	s.have = true
	s.fetchedAt = time.Now()
	return s.cached
}

// musicBitrate returns the effective AAC bitrate (kbps).
func (s *settingsCache) musicBitrate() int {
	return s.get().musicBitrate
}
