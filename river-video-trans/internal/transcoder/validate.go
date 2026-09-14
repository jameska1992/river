package transcoder

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// Output-validation tuning. These are deliberately not admin-configurable —
// they're implementation details of "is this file obviously broken", where
// the admin-facing knobs (on/off + duration tolerance) live in Config.
const (
	// minOutputBytes rejects empty / header-only output. Any real transcode
	// is far larger; this only catches truncated or zero-content files.
	minOutputBytes = 1024
	// minDurationToleranceSec is an absolute floor on the duration check so a
	// legitimate container/frame-boundary rounding difference (sub-second)
	// never trips it, even when DurationTolerancePct resolves to ~0.
	minDurationToleranceSec = 2.0
	// contentSampleFrames is how many frames we sample for the solid-colour
	// check. We reject only if ALL sampled frames are uniform, so a few
	// samples spread across the file avoid false-positives on a single
	// fade-to-black / title card while still catching the green-frame bug
	// (which makes every frame uniform).
	contentSampleFrames = 3
)

// frameStat is the per-frame luma/chroma spread from ffmpeg's signalstats
// filter. A frame is "uniform" (a single flat colour) when every channel's
// min equals its max — the signature of the green-frame / decode-failure
// class this check exists to catch.
type frameStat struct {
	yMin, yMax int
	uMin, uMax int
	vMin, vMax int
}

func (f frameStat) uniform() bool {
	return f.yMin == f.yMax && f.uMin == f.uMax && f.vMin == f.vMax
}

// durationTolerance is the allowed absolute drift (seconds) between source
// and output duration: pct% of the source, but never below the sub-second
// floor that normal transcoding introduces.
func durationTolerance(srcDur float64, pct int) float64 {
	t := srcDur * float64(pct) / 100
	if t < minDurationToleranceSec {
		t = minDurationToleranceSec
	}
	return t
}

// validateOutput is the pure decision: given the source info, the probed
// output info, the output size, and any sampled frame stats, decide whether
// the transcode is acceptable. It performs no I/O so it's fully unit-tested;
// the ffprobe/ffmpeg gathering lives in validateAttempt.
func validateOutput(src, out *FileInfo, size int64, frames []frameStat, cfg Config) error {
	if out == nil {
		return errors.New("could not probe output")
	}
	if out.VideoCodec == "" {
		return errors.New("output has no video stream")
	}
	// We map every source audio stream, so the output must carry at least as
	// many. Fewer means a stream was dropped despite ffmpeg exiting 0.
	if len(out.AudioStreams) < len(src.AudioStreams) {
		return fmt.Errorf("output has %d audio stream(s), source had %d", len(out.AudioStreams), len(src.AudioStreams))
	}
	if size < minOutputBytes {
		return fmt.Errorf("output is only %d bytes (looks empty/truncated)", size)
	}
	// Duration check only when both are known; some containers don't report a
	// duration and we don't want to fail on missing metadata.
	if src.Duration > 0 && out.Duration > 0 {
		tol := durationTolerance(src.Duration, cfg.DurationTolerancePct)
		if diff := math.Abs(out.Duration - src.Duration); diff > tol {
			return fmt.Errorf("output duration %.1fs differs from source %.1fs by %.1fs (> %.1fs tolerance)",
				out.Duration, src.Duration, diff, tol)
		}
	}
	// Content: reject only if EVERY sampled frame is a uniform solid colour.
	if len(frames) > 0 {
		for _, f := range frames {
			if !f.uniform() {
				return nil
			}
		}
		return errors.New("every sampled frame is a uniform solid colour (likely green-frame / decode failure)")
	}
	return nil
}

// validateAttempt gathers the evidence (ffprobe the output, stat its size,
// optionally sample frames) and runs validateOutput. Returns nil — accept —
// when validation is disabled. Any error means "reject this attempt and fall
// through to the next encoder".
func validateAttempt(src *FileInfo, outputPath string, cfg Config) error {
	if !cfg.ValidateOutput {
		return nil
	}
	out, err := Probe(outputPath)
	if err != nil {
		return fmt.Errorf("probe output: %w", err)
	}
	var size int64
	if fi, err := os.Stat(outputPath); err == nil {
		size = fi.Size()
	}
	var frames []frameStat
	if cfg.ValidateContent && out.Duration > 0 {
		frames = sampleFrameStats(outputPath, out.Duration, contentSampleFrames)
	}
	return validateOutput(src, out, size, frames, cfg)
}

// sampleFrameStats samples n frames spread across the file and returns their
// signalstats. A frame that can't be read/parsed is skipped rather than
// treated as uniform, so a probe hiccup can't cause a false rejection (the
// caller only rejects when *all* returned frames are uniform).
func sampleFrameStats(path string, dur float64, n int) []frameStat {
	var stats []frameStat
	for i := 1; i <= n; i++ {
		ts := dur * float64(i) / float64(n+1) // spread e.g. 25/50/75% for n=3
		if s, ok := sampleFrameAt(path, ts); ok {
			stats = append(stats, s)
		}
	}
	return stats
}

// sampleFrameAt seeks to ts seconds, reads one frame, and parses its
// signalstats. metadata=print writes to a temp file (not stdout/stderr) so
// ffmpeg's own logging can't corrupt the parse.
func sampleFrameAt(path string, ts float64) (frameStat, bool) {
	meta, err := os.CreateTemp("", "signalstats-*.txt")
	if err != nil {
		return frameStat{}, false
	}
	metaPath := meta.Name()
	meta.Close()
	defer os.Remove(metaPath)

	cmd := exec.Command("ffmpeg",
		"-v", "error",
		"-ss", strconv.FormatFloat(ts, 'f', 3, 64),
		"-i", path,
		"-frames:v", "1",
		"-vf", "signalstats,metadata=print:file="+metaPath,
		"-an",
		"-f", "null", "-",
	)
	if err := cmd.Run(); err != nil {
		return frameStat{}, false
	}
	out, err := os.ReadFile(metaPath)
	if err != nil {
		return frameStat{}, false
	}
	return parseSignalstats(string(out))
}

// parseSignalstats extracts the min/max luma/chroma values from one frame's
// `metadata=print` output. Lines look like `lavfi.signalstats.YMIN=16`.
// Returns ok=false unless all six min/max fields were present.
func parseSignalstats(s string) (frameStat, bool) {
	vals := map[string]int{}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimPrefix(strings.TrimSpace(key), "lavfi.signalstats.")
		// Values can be floats (e.g. YAVG); the min/max we care about are
		// integers, but parse leniently and truncate.
		if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
			vals[key] = int(f)
		}
	}
	need := []string{"YMIN", "YMAX", "UMIN", "UMAX", "VMIN", "VMAX"}
	for _, k := range need {
		if _, ok := vals[k]; !ok {
			return frameStat{}, false
		}
	}
	return frameStat{
		yMin: vals["YMIN"], yMax: vals["YMAX"],
		uMin: vals["UMIN"], uMax: vals["UMAX"],
		vMin: vals["VMIN"], vMax: vals["VMAX"],
	}, true
}
