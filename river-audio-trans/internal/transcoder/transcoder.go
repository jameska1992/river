package transcoder

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type AudioFileInfo struct {
	Codec    string
	Duration int // seconds
}

type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		CodecName string `json:"codec_name"`
		Duration  string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

func Probe(path string) (*AudioFileInfo, error) {
	out, err := exec.Command(
		"ffprobe", "-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		path,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe %q: %w", path, err)
	}
	var result ffprobeOutput
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}
	info := &AudioFileInfo{}
	for _, s := range result.Streams {
		if s.CodecType == "audio" && info.Codec == "" {
			info.Codec = s.CodecName
			if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
				info.Duration = int(d)
			}
		}
	}
	// Fall back to container duration if the stream didn't report one.
	if info.Duration == 0 {
		if d, err := strconv.ParseFloat(result.Format.Duration, 64); err == nil {
			info.Duration = int(d)
		}
	}
	return info, nil
}

func NeedsTranscode(path string, info *AudioFileInfo) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext != ".m4a" || info.Codec != "aac"
}

// OutputPath returns where the transcoded .m4a should be written. When
// outputDir is set, output lands at
//
//	{outputDir}/{libraryType}/{rel under libraryPath}/{file}.m4a
//
// libraryType is the top-level grouping ("music" / "audiobook") rather than
// the basename of the source path — so a library configured with two source
// paths (e.g. /mnt/lib_a and /mnt/lib_b) still collapses into one output
// root instead of one per source directory.
//
//	libraryType=music, libraryPath=/mnt/truenas/music_classical,
//	outputDir=/mnt/tank/media,
//	input=/mnt/truenas/music_classical/Artist/Album/01.flac
//	  → /mnt/tank/media/music/Artist/Album/01.m4a
//
// Falls back to {outputDir}/{libraryType}/{file} when libraryPath is unset
// or the input isn't under it; falls back to flat {outputDir}/{file} when
// libraryType is also unset. With outputDir empty, the file lands next to
// the source as before.
func OutputPath(inputPath, libraryType, libraryPath, outputDir string) string {
	ext := filepath.Ext(inputPath)
	stem := inputPath[:len(inputPath)-len(ext)]
	name := filepath.Base(stem) + ".m4a"
	if strings.EqualFold(ext, ".m4a") {
		name = filepath.Base(stem) + "_transcoded.m4a"
	}
	if outputDir == "" {
		return filepath.Join(filepath.Dir(inputPath), name)
	}
	if libraryType == "" {
		// Defensive fallback: keep output under outputDir but flat. Shouldn't
		// happen in practice — every event carries a library type.
		return filepath.Join(outputDir, name)
	}
	if libraryPath == "" {
		return filepath.Join(outputDir, libraryType, name)
	}
	rel, err := filepath.Rel(filepath.Clean(libraryPath), filepath.Dir(inputPath))
	if err != nil || strings.HasPrefix(rel, "..") {
		return filepath.Join(outputDir, libraryType, name)
	}
	return filepath.Join(outputDir, libraryType, rel, name)
}

// Transcode converts inputPath to AAC in an M4A container at outputPath.
func Transcode(inputPath, outputPath string) error {
	if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	cmd := exec.Command(
		"ffmpeg",
		"-i", inputPath,
		"-vn",
		"-c:a", "aac",
		"-b:a", "256k",
		"-movflags", "+faststart",
		"-y",
		outputPath,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
