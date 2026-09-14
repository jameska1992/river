package transcoder

import "testing"

// validCfg enables both validation stages with the default tolerance.
func validCfg() Config {
	c := DefaultConfig()
	c.ValidateOutput = true
	c.ValidateContent = true
	return c
}

func src() *FileInfo {
	return &FileInfo{VideoCodec: "h264", Duration: 100, AudioStreams: []AudioStream{{CodecName: "aac"}}}
}

func goodOut() *FileInfo {
	return &FileInfo{VideoCodec: "h264", Duration: 100, AudioStreams: []AudioStream{{CodecName: "aac"}}}
}

// a non-uniform frame (real content)
func varied() frameStat { return frameStat{yMin: 16, yMax: 235, uMin: 16, uMax: 240, vMin: 16, vMax: 240} }

// a uniform (solid-colour) frame
func solid() frameStat { return frameStat{yMin: 128, yMax: 128, uMin: 128, uMax: 128, vMin: 128, vMax: 128} }

func TestValidateOutput(t *testing.T) {
	cases := []struct {
		name    string
		out     *FileInfo
		size    int64
		frames  []frameStat
		wantErr bool
	}{
		{"accepts good output", goodOut(), 5_000_000, []frameStat{varied(), varied(), varied()}, false},
		{"nil output", nil, 5_000_000, nil, true},
		{"no video stream", &FileInfo{Duration: 100, AudioStreams: []AudioStream{{CodecName: "aac"}}}, 5_000_000, nil, true},
		{"dropped audio stream", &FileInfo{VideoCodec: "h264", Duration: 100}, 5_000_000, nil, true},
		{"empty/truncated size", goodOut(), 512, nil, true},
		{"duration far off", &FileInfo{VideoCodec: "h264", Duration: 40, AudioStreams: []AudioStream{{CodecName: "aac"}}}, 5_000_000, nil, true},
		{"duration within tolerance", &FileInfo{VideoCodec: "h264", Duration: 101, AudioStreams: []AudioStream{{CodecName: "aac"}}}, 5_000_000, []frameStat{varied()}, false},
		{"all frames solid colour", goodOut(), 5_000_000, []frameStat{solid(), solid(), solid()}, true},
		{"one varied frame saves it", goodOut(), 5_000_000, []frameStat{solid(), varied(), solid()}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := validateOutput(src(), c.out, c.size, c.frames, validCfg())
			if c.wantErr && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !c.wantErr && err != nil {
				t.Errorf("expected nil, got %v", err)
			}
		})
	}
}

func TestValidateOutput_MissingDurationsSkipCheck(t *testing.T) {
	// Source with unknown duration must not trip the duration check.
	s := &FileInfo{VideoCodec: "h264", Duration: 0, AudioStreams: []AudioStream{{CodecName: "aac"}}}
	out := &FileInfo{VideoCodec: "h264", Duration: 12, AudioStreams: []AudioStream{{CodecName: "aac"}}}
	if err := validateOutput(s, out, 5_000_000, []frameStat{varied()}, validCfg()); err != nil {
		t.Errorf("unknown source duration should skip the duration check, got %v", err)
	}
}

func TestDurationTolerance(t *testing.T) {
	// pct dominates for long sources: 2% of 1000s = 20s.
	if got := durationTolerance(1000, 2); got != 20 {
		t.Errorf("durationTolerance(1000, 2) = %v, want 20", got)
	}
	// floor dominates for short sources: 2% of 30s = 0.6s < 2s floor.
	if got := durationTolerance(30, 2); got != minDurationToleranceSec {
		t.Errorf("durationTolerance(30, 2) = %v, want %v (floor)", got, minDurationToleranceSec)
	}
	// pct 0 still honours the absolute floor.
	if got := durationTolerance(1000, 0); got != minDurationToleranceSec {
		t.Errorf("durationTolerance(1000, 0) = %v, want %v (floor)", got, minDurationToleranceSec)
	}
}

func TestParseSignalstats(t *testing.T) {
	// A uniform (solid green) frame: every channel min == max.
	uniform := `frame:0    pts:0       pts_time:0
lavfi.signalstats.YMIN=128
lavfi.signalstats.YLOW=128
lavfi.signalstats.YAVG=128.0
lavfi.signalstats.YHIGH=128
lavfi.signalstats.YMAX=128
lavfi.signalstats.UMIN=64
lavfi.signalstats.UMAX=64
lavfi.signalstats.VMIN=200
lavfi.signalstats.VMAX=200
`
	fs, ok := parseSignalstats(uniform)
	if !ok {
		t.Fatal("expected parse ok for complete metadata")
	}
	if !fs.uniform() {
		t.Errorf("expected uniform frame, got %+v", fs)
	}

	varied := `lavfi.signalstats.YMIN=16
lavfi.signalstats.YMAX=235
lavfi.signalstats.UMIN=16
lavfi.signalstats.UMAX=240
lavfi.signalstats.VMIN=16
lavfi.signalstats.VMAX=240
`
	fs, ok = parseSignalstats(varied)
	if !ok {
		t.Fatal("expected parse ok")
	}
	if fs.uniform() {
		t.Errorf("expected non-uniform frame, got %+v", fs)
	}

	// Missing fields → not ok (caller skips the frame rather than assuming).
	if _, ok := parseSignalstats("lavfi.signalstats.YMIN=1\nlavfi.signalstats.YMAX=2\n"); ok {
		t.Error("expected ok=false when chroma fields are missing")
	}
}
