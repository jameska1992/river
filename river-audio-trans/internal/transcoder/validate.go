package transcoder

import (
	"errors"
	"fmt"
	"math"
	"os"
)

// Output-validation tuning. Not admin-configurable — implementation details
// of "is this file obviously broken"; the admin-facing knobs (on/off +
// duration tolerance) live in the shared transcoding settings.
const (
	// minOutputBytes rejects empty / header-only output. Any real transcode
	// is far larger.
	minOutputBytes = 1024
	// minDurationToleranceSec is an absolute floor so normal container
	// rounding (sub-second) never trips the duration check.
	minDurationToleranceSec = 2.0
)

// durationTolerance is the allowed absolute drift (seconds): pct% of the
// source, but never below the sub-second floor transcoding introduces.
func durationTolerance(srcDur float64, pct int) float64 {
	t := srcDur * float64(pct) / 100
	if t < minDurationToleranceSec {
		t = minDurationToleranceSec
	}
	return t
}

// validateAudioOutput is the pure decision: given the probed output, its
// size, and the source duration, decide whether the transcode is
// acceptable. No I/O, so it's fully unit-tested.
func validateAudioOutput(out *AudioFileInfo, size int64, srcDuration, tolPct int) error {
	if out == nil {
		return errors.New("could not probe output")
	}
	if out.Codec != "aac" {
		return fmt.Errorf("output codec is %q, expected aac", out.Codec)
	}
	if size < minOutputBytes {
		return fmt.Errorf("output is only %d bytes (looks empty/truncated)", size)
	}
	if srcDuration > 0 && out.Duration > 0 {
		tol := durationTolerance(float64(srcDuration), tolPct)
		if diff := math.Abs(float64(out.Duration - srcDuration)); diff > tol {
			return fmt.Errorf("output duration %ds differs from source %ds by %.0fs (> %.1fs tolerance)",
				out.Duration, srcDuration, diff, tol)
		}
	}
	return nil
}

// validateAttempt probes the output and stats its size, then runs
// validateAudioOutput.
func validateAttempt(outputPath string, srcDuration, tolPct int) error {
	out, err := Probe(outputPath)
	if err != nil {
		return fmt.Errorf("probe output: %w", err)
	}
	var size int64
	if fi, err := os.Stat(outputPath); err == nil {
		size = fi.Size()
	}
	return validateAudioOutput(out, size, srcDuration, tolPct)
}
