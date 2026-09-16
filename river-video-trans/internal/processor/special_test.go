package processor

import (
	"testing"

	"river-video-trans/internal/apiclient"
)

// When meta-tv hasn't created the special's episode record yet, the special
// has no entry in specialBySource. Previously this was silently skipped
// (returning without error), so the special was never transcoded. It must now
// return an error so the message is retried/dead-lettered instead (issue #78).
func TestProcessSpecialFile_NotReadyReturnsError(t *testing.T) {
	p := &Processor{} // not-ready path returns before touching api/transcoder
	err := p.processSpecialFile(
		"/tv/Doctor Who/Specials/S02E00 The Christmas Invasion.mkv",
		"show-id", "season-id", "Doctor Who", 0,
		map[string]apiclient.Episode{}, // empty → record not created yet
		false,
	)
	if err == nil {
		t.Fatal("expected a retryable error when the special's record isn't ready, got nil")
	}
}
