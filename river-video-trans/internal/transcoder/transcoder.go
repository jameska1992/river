package transcoder

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type FileInfo struct {
	VideoCodec   string
	// VideoProfile is the codec profile reported by ffprobe (e.g. "High",
	// "Main 10", "Rext"). Used in fallback diagnostics so we can tell
	// *why* NVDEC bailed out — Turing's NVDEC handles HEVC Main 10 fine
	// but not Rext / 4:2:2 / 4:4:4 variants, and that's invisible from
	// VideoCodec alone.
	VideoProfile string
	AudioCodec   string // first audio stream codec (used by NeedsTranscode)
	PixFmt       string // e.g. "yuv420p", "yuv420p10le"
	Width        int
	Height       int
	AudioStreams  []AudioStream
	Subtitles    []SubtitleStream
}

type AudioStream struct {
	Index     int    // 0-based index among audio streams
	Language  string // BCP-47 tag e.g. "en"; empty if unknown
	Title     string // stream title tag if present
	CodecName string
}

type SubtitleStream struct {
	Index    int    // absolute stream index (for ffmpeg -map 0:N)
	Language string // BCP-47 tag e.g. "en"; empty if unknown
	Title    string // stream title tag if present
}

type ffprobeOutput struct {
	Streams []struct {
		Index     int    `json:"index"`
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Profile   string `json:"profile"`
		PixFmt    string `json:"pix_fmt"`
		Width     int    `json:"width"`
		Height    int    `json:"height"`
		Tags      struct {
			Language string `json:"language"`
			Title    string `json:"title"`
		} `json:"tags"`
	} `json:"streams"`
}

func Probe(path string) (*FileInfo, error) {
	out, err := exec.Command(
		"ffprobe", "-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		path,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %q: %w", path, err)
	}
	var result ffprobeOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	info := &FileInfo{}
	audioIdx := 0
	for _, s := range result.Streams {
		switch s.CodecType {
		case "video":
			if info.VideoCodec == "" {
				info.VideoCodec = s.CodecName
				info.VideoProfile = s.Profile
				info.PixFmt = s.PixFmt
				info.Width = s.Width
				info.Height = s.Height
			}
		case "audio":
			info.AudioStreams = append(info.AudioStreams, AudioStream{
				Index:     audioIdx,
				Language:  s.Tags.Language,
				Title:     s.Tags.Title,
				CodecName: s.CodecName,
			})
			if audioIdx == 0 {
				info.AudioCodec = s.CodecName
			}
			audioIdx++
		case "subtitle":
			info.Subtitles = append(info.Subtitles, SubtitleStream{
				Index:    s.Index,
				Language: s.Tags.Language,
				Title:    s.Tags.Title,
			})
		}
	}
	return info, nil
}

// CreateVariant remuxes the input file retaining the first video stream and
// exactly one audio stream (by relative audio index). Both streams are copied
// (no re-encoding), so the operation is fast and lossless.
// The output is a regular faststart MP4 that supports HTTP Range requests.
func CreateVariant(inputPath string, audioIdx int, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create variant dir: %w", err)
	}
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", fmt.Sprintf("0:a:%d", audioIdx),
		"-c:v", "copy",
		"-c:a", "copy",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create audio variant %d: %w", audioIdx, err)
	}
	return nil
}

// ExtractSubtitle extracts one subtitle stream to a WebVTT file.
// outputPath must end in .vtt.
func ExtractSubtitle(inputPath string, streamIndex int, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create subtitle dir: %w", err)
	}
	cmd := exec.Command("ffmpeg",
		"-i", inputPath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		"-y",
		outputPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("extract subtitle stream %d: %w", streamIndex, err)
	}
	return nil
}

func NeedsTranscode(path string, info *FileInfo) bool {
	ext := strings.ToLower(filepath.Ext(path))
	is10bit := strings.Contains(info.PixFmt, "10")
	if ext != ".mp4" || info.VideoCodec != "h264" || is10bit || info.Width > 1920 || info.Height > 1080 {
		return true
	}
	for _, a := range info.AudioStreams {
		if a.CodecName != "aac" {
			return true
		}
	}
	return false
}


// Transcode produces an h.264 + AAC mp4 at most 1080p, attempting (in order):
//
//  1. NVENC encode + CUDA decode — full GPU pipeline; biggest win for 4K
//     sources because the downscale runs on the GPU and frames never copy
//     back to system memory. Requires NVDEC support for the input codec.
//  2. NVENC encode + CPU decode — falls back when NVDEC can't handle the
//     input codec (e.g. AV1 on pre-Ampere, some 10-bit HEVC variants).
//  3. libx264 — full CPU path; final fallback for hosts without NVENC.
//
// Transcoding writes through a scratch file under tmpDir; only on success is
// the file renamed into outputPath. A crash mid-transcode therefore leaves
// the canonical path untouched and a stray scratch file behind for the next
// run. tmpDir must live on the same filesystem as outputPath so the final
// move is an atomic rename; an empty tmpDir falls back to the output's own
// directory.
// Logger is the surface the transcoder needs to forward path-selection
// notes to river-api so they show up in the admin log feed, not just in
// docker logs. Satisfied by *apiclient.Client (its Log method matches);
// nil is fine — the transcoder falls back to local log.Printf only.
type Logger interface {
	Log(level, message string)
}

func Transcode(inputPath, outputPath, tmpDir string, info *FileInfo, logger Logger) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}
	if tmpDir == "" {
		tmpDir = filepath.Dir(outputPath)
	}
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("create tmp dir: %w", err)
	}
	tmpFile, err := os.CreateTemp(tmpDir, "transcode-*"+filepath.Ext(outputPath))
	if err != nil {
		return fmt.Errorf("create tmp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close() // ffmpeg reopens via -y; we just needed a unique name
	defer os.Remove(tmpPath)

	attempts := []struct {
		enc       videoEncoder
		name      string
		// cpuDecode tagging is the cheap way to log a one-line warning the
		// moment we drop off the GPU decode path — useful when triaging why
		// per-stream throughput is below expected (CPU decode roughly halves
		// it). The encoder name alone is ambiguous since "h264_nvenc"
		// appears in both GPU and CPU-decode paths.
		cpuDecode bool
	}{
		{nvencHWEncoder, "h264_nvenc + cuda decode", false},
		{nvencEncoder, "h264_nvenc (cpu decode)", true},
		{x264Encoder, "libx264 (cpu decode + cpu encode)", true},
	}
	// sourceDesc summarizes the input in a "codec/profile pixfmt WxH" shape
	// for diagnostic logs. Profile + pixfmt are the most common reasons
	// NVDEC bails out (e.g. hevc/Rext, yuv420p10le on a non-Ampere card),
	// so they're worth surfacing every time we drop off the GPU path.
	sourceDesc := info.VideoCodec
	if info.VideoProfile != "" {
		sourceDesc += "/" + info.VideoProfile
	}
	if info.PixFmt != "" {
		sourceDesc += " " + info.PixFmt
	}
	if info.Width > 0 && info.Height > 0 {
		sourceDesc += fmt.Sprintf(" %dx%d", info.Width, info.Height)
	}

	// emit logs both to docker stdout (always) and to river-api (when a
	// logger was supplied) so the admin log feed surfaces CPU-decode
	// falls without anyone having to shell into the container.
	emit := func(level, msg string) {
		log.Printf("%s %s", strings.ToUpper(level), msg)
		if logger != nil {
			logger.Log(level, msg)
		}
	}

	var lastErr error
	var lastStderr string
	for i, a := range attempts {
		stderr, err := runFFmpeg(buildArgs(inputPath, tmpPath, info, a.enc))
		if err == nil {
			if a.cpuDecode {
				// One-line, grep-friendly. The earlier WARN logged WHY the
				// GPU path failed; this confirms which fallback path won.
				emit("warn", fmt.Sprintf("%s transcoded with CPU decode path: %s (source: %s)",
					filepath.Base(inputPath), a.name, sourceDesc))
			} else {
				emit("info", fmt.Sprintf("%s transcoded with %s (source: %s)",
					filepath.Base(inputPath), a.name, sourceDesc))
			}
			if err := os.Rename(tmpPath, outputPath); err != nil {
				return fmt.Errorf("move tmp into output: %w", err)
			}
			return nil
		}
		lastErr = err
		lastStderr = stderr
		if i == len(attempts)-1 {
			break
		}
		nextPath := "CPU decode"
		if !attempts[i+1].cpuDecode {
			nextPath = "GPU decode"
		}
		// API log gets the concise reason; the full ffmpeg stderr is only
		// dumped to docker logs since it'd otherwise drown the admin feed.
		emit("warn", fmt.Sprintf("%s failed (%v), falling back to %s (%s); source: %s",
			a.name, err, nextPath, attempts[i+1].name, sourceDesc))
		log.Printf("ffmpeg stderr: %s", stderr)
		// No need to remove tmpPath; the next attempt's ffmpeg -y overwrites it.
	}
	if lastStderr != "" {
		return fmt.Errorf("%w: %s", lastErr, lastStderr)
	}
	return lastErr
}

type videoEncoder struct {
	codec   string
	preset  string
	quality []string // encoder-specific quality flags (appended after preset)
	hwAccel bool     // when true, decode frames into CUDA memory and use scale_cuda
}

var (
	// nvencHWEncoder: NVENC encode with full-GPU pipeline (CUDA decode +
	// scale_cuda). Fastest path; requires NVDEC support for the input codec.
	nvencHWEncoder = videoEncoder{
		codec:   "h264_nvenc",
		preset:  "p3",
		quality: []string{"-rc", "vbr", "-cq", "23", "-b:v", "0"},
		hwAccel: true,
	}
	// nvencEncoder: NVENC encode with CPU decode — used when NVDEC can't
	// handle the input. Same VBR constant-quality knobs as the HW path.
	nvencEncoder = videoEncoder{
		codec:   "h264_nvenc",
		preset:  "p3",
		quality: []string{"-rc", "vbr", "-cq", "23", "-b:v", "0"},
	}
	// x264Encoder: software CRF 23, medium preset. Final fallback.
	x264Encoder = videoEncoder{
		codec:   "libx264",
		preset:  "medium",
		quality: []string{"-crf", "23"},
	}
)

func buildArgs(inputPath, outputPath string, info *FileInfo, enc videoEncoder) []string {
	is10bit := strings.Contains(info.PixFmt, "10")
	needsResize := info.Width > 1920 || info.Height > 1080
	needsVideoEncode := info.VideoCodec != "h264" || is10bit || needsResize

	var args []string
	// Hardware-decoded frames land in GPU memory and stay there through
	// the scale filter and into the encoder — no CPU↔GPU copies. Only
	// useful when we're actually re-encoding; the stream-copy path needs
	// frames on CPU for the muxer.
	if enc.hwAccel && needsVideoEncode {
		args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
	}
	args = append(args, "-i", inputPath, "-y")

	// Explicit stream mapping: first video track + all audio tracks.
	// This preserves multi-language audio while ignoring cover art / data streams.
	args = append(args, "-map", "0:v:0")
	for i := range info.AudioStreams {
		args = append(args, "-map", fmt.Sprintf("0:a:%d", i))
	}

	if needsVideoEncode {
		args = append(args, "-c:v", enc.codec, "-preset", enc.preset)
		args = append(args, enc.quality...)

		// Filter chain. Two jobs:
		//   1. Resize anything over 1080p down to 1080p.
		//   2. Convert 10-bit pixel formats to 8-bit yuv420p.
		// Job 2 matters because h264_nvenc on consumer NVENC (Turing/Ampere
		// for h264) only accepts 8-bit input — feeding it yuv420p10le yields
		// "10 bit encode not supported / Provided device doesn't support
		// required NVENC features", and the whole NVENC fallback chain
		// fails to libx264 (a ~30× CPU penalty). The conversion has to
		// happen in the filter graph (before encoder init) because
		// -pix_fmt alone gets negotiated too late.
		if enc.hwAccel {
			// Frames are in CUDA memory; only scale_cuda can operate on
			// them without a download/upload round-trip.
			var opts []string
			if needsResize {
				opts = append(opts,
					"w='min(iw,1920)'",
					"h='min(ih,1080)'",
					"force_original_aspect_ratio=decrease",
				)
			}
			if is10bit {
				opts = append(opts, "format=yuv420p")
			}
			if len(opts) > 0 {
				args = append(args, "-vf", "scale_cuda="+strings.Join(opts, ":"))
			}
		} else {
			// CPU decode → CPU filters → encoder. Always end with
			// format=yuv420p — both h264_nvenc and libx264 want 8-bit
			// here, and explicit beats encoder-side auto-negotiation.
			var filters []string
			if needsResize {
				filters = append(filters, "scale=w='min(iw,1920)':h='min(ih,1080)':force_original_aspect_ratio=decrease")
			}
			filters = append(filters, "format=yuv420p")
			args = append(args, "-vf", strings.Join(filters, ","))
		}
	} else {
		args = append(args, "-c:v", "copy")
	}

	// Per-stream audio: copy AAC streams, encode everything else.
	for i, a := range info.AudioStreams {
		if a.CodecName == "aac" {
			args = append(args, fmt.Sprintf("-c:a:%d", i), "copy")
		} else {
			args = append(args,
				fmt.Sprintf("-c:a:%d", i), "aac",
				fmt.Sprintf("-b:a:%d", i), "192k",
			)
		}
	}

	return append(args, "-movflags", "+faststart", outputPath)
}

// runFFmpeg invokes ffmpeg and tees stderr to both the container's stderr
// (real-time visibility) and an in-memory buffer (so callers can include the
// failure reason in user-facing logs). On success returns ("", nil); on
// failure returns a trimmed stderr tail and the underlying exec error.
func runFFmpeg(args []string) (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("ffmpeg", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = io.MultiWriter(os.Stderr, &buf)
	err := cmd.Run()
	if err == nil {
		return "", nil
	}
	return ffmpegErrorTail(buf.String()), err
}

// ffmpegErrorTail extracts the last few meaningful lines of ffmpeg stderr,
// stripping the progress chatter that dominates the buffer. ffmpeg writes
// progress with \r so it overwrites on a TTY but accumulates when captured;
// we split on both newlines and carriage returns to flatten that.
func ffmpegErrorTail(s string) string {
	parts := strings.FieldsFunc(s, func(r rune) bool {
		return r == '\n' || r == '\r'
	})
	var kept []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "frame=") || strings.HasPrefix(p, "size=") {
			continue
		}
		kept = append(kept, p)
	}
	const maxLen = 500
	out := ""
	for i := len(kept) - 1; i >= 0; i-- {
		var candidate string
		if out == "" {
			candidate = kept[i]
		} else {
			candidate = kept[i] + " | " + out
		}
		if len(candidate) > maxLen {
			break
		}
		out = candidate
	}
	return out
}
