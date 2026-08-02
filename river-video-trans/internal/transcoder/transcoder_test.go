package transcoder

import (
	"strings"
	"testing"
)

func contains(hay []string, needle string) bool {
	for _, h := range hay {
		if h == needle {
			return true
		}
	}
	return false
}

// containsSeq reports whether seq appears as a contiguous run within hay.
func containsSeq(hay []string, seq ...string) bool {
	for i := 0; i+len(seq) <= len(hay); i++ {
		match := true
		for j, s := range seq {
			if hay[i+j] != s {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// flagValue returns the argument following flag.
func flagValue(args []string, flag string) (string, bool) {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func TestNeedsTranscode(t *testing.T) {
	compliant := func() *FileInfo {
		return &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 1920, Height: 1080,
			AudioStreams: []AudioStream{{CodecName: "aac"}}}
	}
	cases := []struct {
		name string
		path string
		info *FileInfo
		want bool
	}{
		{"already compliant", "v.mp4", compliant(), false},
		{"two aac streams still compliant", "v.mp4", &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 1280, Height: 720, AudioStreams: []AudioStream{{CodecName: "aac"}, {CodecName: "aac"}}}, false},
		{"non-mp4 container", "v.mkv", compliant(), true},
		{"non-h264 codec", "v.mp4", &FileInfo{VideoCodec: "hevc", PixFmt: "yuv420p", Width: 1920, Height: 1080, AudioStreams: []AudioStream{{CodecName: "aac"}}}, true},
		{"10-bit pixel format", "v.mp4", &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p10le", Width: 1920, Height: 1080, AudioStreams: []AudioStream{{CodecName: "aac"}}}, true},
		{"over 1080p width", "v.mp4", &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 3840, Height: 2160, AudioStreams: []AudioStream{{CodecName: "aac"}}}, true},
		{"non-aac audio", "v.mp4", &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 1920, Height: 1080, AudioStreams: []AudioStream{{CodecName: "ac3"}}}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := NeedsTranscode(c.path, c.info); got != c.want {
				t.Errorf("NeedsTranscode = %v, want %v", got, c.want)
			}
		})
	}
}

func TestBuildArgs_StreamCopy(t *testing.T) {
	info := &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 1920, Height: 1080,
		AudioStreams: []AudioStream{{CodecName: "aac"}}}
	// Even with a hwAccel encoder, a compliant input is stream-copied — no
	// re-encode, so no -hwaccel decode setup and no filter chain.
	args := buildArgs("in.mp4", "out.mp4", info, nvencHWEncoder)

	if !containsSeq(args, "-c:v", "copy") {
		t.Errorf("expected stream copy, got %v", args)
	}
	if contains(args, "-hwaccel") {
		t.Error("compliant input must not set up hardware decode")
	}
	if _, ok := flagValue(args, "-vf"); ok {
		t.Error("stream copy should have no filter chain")
	}
	if !containsSeq(args, "-c:a:0", "copy") {
		t.Error("aac audio should be copied")
	}
	if !containsSeq(args, "-map", "0:v:0") || !containsSeq(args, "-map", "0:a:0") {
		t.Errorf("expected explicit v+a stream mapping, got %v", args)
	}
	if args[len(args)-1] != "out.mp4" || !containsSeq(args, "-movflags", "+faststart") {
		t.Errorf("expected faststart + output path last, got %v", args)
	}
}

func TestBuildArgs_SoftwareResizeAndAudioEncode(t *testing.T) {
	info := &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 3840, Height: 2160,
		AudioStreams: []AudioStream{{CodecName: "ac3"}}}
	args := buildArgs("in.mkv", "out.mp4", info, x264Encoder)

	if !containsSeq(args, "-c:v", "libx264") || !containsSeq(args, "-preset", "medium") || !containsSeq(args, "-crf", "23") {
		t.Errorf("expected libx264 CRF settings, got %v", args)
	}
	if contains(args, "-hwaccel") {
		t.Error("software encoder must not request hardware decode")
	}
	vf, ok := flagValue(args, "-vf")
	if !ok || !strings.Contains(vf, "scale=") || !strings.Contains(vf, "format=yuv420p") {
		t.Errorf("expected CPU scale+format filter, got -vf %q", vf)
	}
	// ac3 is not aac → transcode to aac 192k.
	if !containsSeq(args, "-c:a:0", "aac") || !containsSeq(args, "-b:a:0", "192k") {
		t.Errorf("expected ac3 → aac 192k, got %v", args)
	}
}

func TestBuildArgs_HardwarePipeline10Bit(t *testing.T) {
	// 10-bit HEVC at 1080p: no resize, only a pixel-format downconvert.
	info := &FileInfo{VideoCodec: "hevc", PixFmt: "yuv420p10le", Width: 1920, Height: 1080,
		AudioStreams: []AudioStream{{CodecName: "aac"}}}
	args := buildArgs("in.mkv", "out.mp4", info, nvencHWEncoder)

	if !containsSeq(args, "-hwaccel", "cuda") || !containsSeq(args, "-hwaccel_output_format", "cuda") {
		t.Errorf("expected CUDA decode pipeline, got %v", args)
	}
	if !containsSeq(args, "-c:v", "h264_nvenc") {
		t.Errorf("expected nvenc encoder, got %v", args)
	}
	vf, ok := flagValue(args, "-vf")
	if !ok {
		t.Fatal("expected a filter chain for 10-bit conversion")
	}
	// The 10-bit→8-bit conversion must run on the CPU (hwdownload), NOT via
	// scale_cuda's format= option, which emits solid-green frames on driver
	// >= 610 + ffmpeg 8.x. At 1080p there's no resize, so no scale_cuda at all.
	if !strings.Contains(vf, "hwdownload") || !strings.Contains(vf, "format=yuv420p") {
		t.Errorf("expected hwdownload + CPU format=yuv420p, got -vf %q", vf)
	}
	if strings.Contains(vf, "scale_cuda") {
		t.Errorf("1080p needs no resize; expected no scale_cuda, got -vf %q", vf)
	}
}

func TestBuildArgs_HardwarePipeline10BitDownscale(t *testing.T) {
	// 10-bit HEVC at 4K: resize on-GPU (scale_cuda) + CPU pixel-format convert.
	info := &FileInfo{VideoCodec: "hevc", PixFmt: "yuv420p10le", Width: 3840, Height: 2160,
		AudioStreams: []AudioStream{{CodecName: "aac"}}}
	args := buildArgs("in.mkv", "out.mp4", info, nvencHWEncoder)

	vf, ok := flagValue(args, "-vf")
	if !ok {
		t.Fatal("expected a filter chain")
	}
	if !strings.Contains(vf, "scale_cuda=") {
		t.Errorf("4K should resize on-GPU via scale_cuda, got -vf %q", vf)
	}
	if !strings.Contains(vf, "hwdownload") || !strings.Contains(vf, "format=yuv420p") {
		t.Errorf("expected CPU pixel-format conversion via hwdownload, got -vf %q", vf)
	}
	// Regression guard: scale_cuda (first in the chain) must carry no format=
	// conversion — that combination is the green-frame bug.
	scaleSeg := strings.SplitN(vf, ",", 2)[0]
	if strings.Contains(scaleSeg, "format=") {
		t.Errorf("scale_cuda must not carry format= (green-frame bug), got %q", scaleSeg)
	}
}

func TestBuildArgs_MultipleAudioMapping(t *testing.T) {
	info := &FileInfo{VideoCodec: "h264", PixFmt: "yuv420p", Width: 1920, Height: 1080,
		AudioStreams: []AudioStream{{CodecName: "aac"}, {CodecName: "dts"}}}
	args := buildArgs("in.mkv", "out.mp4", info, x264Encoder)

	if !containsSeq(args, "-map", "0:a:0") || !containsSeq(args, "-map", "0:a:1") {
		t.Errorf("both audio streams should be mapped, got %v", args)
	}
	if !containsSeq(args, "-c:a:0", "copy") { // aac copied
		t.Error("stream 0 (aac) should be copied")
	}
	if !containsSeq(args, "-c:a:1", "aac") { // dts encoded
		t.Error("stream 1 (dts) should be encoded to aac")
	}
}

func TestFfmpegErrorTail(t *testing.T) {
	in := "frame=  10 fps=5\r" +
		"size=    2048kB\r" +
		"[hevc @ 0x1] Error while decoding\n" +
		"Conversion failed!\n"
	got := ffmpegErrorTail(in)
	want := "[hevc @ 0x1] Error while decoding | Conversion failed!"
	if got != want {
		t.Errorf("ffmpegErrorTail = %q, want %q", got, want)
	}

	if ffmpegErrorTail("frame=1\rsize=2\r") != "" {
		t.Error("output of only progress chatter should be empty")
	}
}
