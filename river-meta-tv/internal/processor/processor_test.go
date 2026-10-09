package processor

import "testing"

// parseEpisodeNumber sees a wide range of release-naming conventions
// from the scanner. Lock the resolution order in so a regex tweak
// can't silently break one of them.
func TestParseEpisodeNumber(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		// SxxExx — most specific, must win over everything else.
		{"Show.S01E03.720p.x265.mkv", 3},
		{"Law and Order SVU S13E07 - Russian Brides.mkv", 7},
		{"Show.s9e12.mkv", 12},

		// NxNN — Plex / DVD-rip style. Has to skip "x265" / "H.264"
		// sequences that happen later in the filename.
		{"Law & Order Special Victims Unit - 7x01 - Demons.mp4", 1},
		{"Law & Order Special Victims Unit - 1x10 - Closure (1).mp4", 10},
		{"Law & Order Special Victims Unit - 23x22 - A Final Call.mp4", 22},

		// Spelled-out "Episode N" — must agree with river-video-trans, or the
		// file is imported twice (episode + special).
		{"The Five Star Weekend (2026) Season 1 Episode 1- Friday- Arrivals - PrimeWire.mp4", 1},
		{"Show - Episode 05.mkv", 5},
		{"Show.Episode.7.mkv", 7},
		{"Show_episode_12.mkv", 12},

		// E-only fallback — only if none of the above hit.
		{"Doctor Who - E07.mkv", 7},

		// False-positive guards.
		{"Show.S01E03.1080p.x265-Group.mkv", 3},     // SxxExx wins, x265 ignored
		{"Show.H.264.NTb.mkv", 0},                   // bare "264" doesn't look like NxNN
		{"Show.1080p.WEB-DL.mkv", 0},                // no episode marker at all

		// Edge cases.
		{"Show.10x100.mkv", 100},                    // 3-digit episode allowed
		{"Show.1x07.x265.mkv", 7},                   // NxNN, then x265 must not steal
	}
	for _, c := range cases {
		if got := parseEpisodeNumber(c.name); got != c.want {
			t.Errorf("parseEpisodeNumber(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}
