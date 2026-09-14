package transcoder

import "testing"

func TestValidateAudioOutput(t *testing.T) {
	cases := []struct {
		name        string
		out         *AudioFileInfo
		size        int64
		srcDuration int
		tolPct      int
		wantErr     bool
	}{
		{"accepts good aac output", &AudioFileInfo{Codec: "aac", Duration: 300}, 4_000_000, 300, 2, false},
		{"nil output", nil, 4_000_000, 300, 2, true},
		{"wrong codec", &AudioFileInfo{Codec: "mp3", Duration: 300}, 4_000_000, 300, 2, true},
		{"empty/truncated size", &AudioFileInfo{Codec: "aac", Duration: 300}, 512, 300, 2, true},
		{"duration far off", &AudioFileInfo{Codec: "aac", Duration: 120}, 4_000_000, 300, 2, true},
		{"duration within tolerance", &AudioFileInfo{Codec: "aac", Duration: 301}, 4_000_000, 300, 2, false},
		{"unknown source duration skips check", &AudioFileInfo{Codec: "aac", Duration: 5}, 4_000_000, 0, 2, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateAudioOutput(c.out, c.size, c.srcDuration, c.tolPct)
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestDurationTolerance(t *testing.T) {
	if got := durationTolerance(1000, 2); got != 20 {
		t.Errorf("durationTolerance(1000, 2) = %v, want 20", got)
	}
	if got := durationTolerance(30, 2); got != minDurationToleranceSec {
		t.Errorf("durationTolerance(30, 2) = %v, want %v (floor)", got, minDurationToleranceSec)
	}
}
