package transcoder

import (
	"path/filepath"
	"testing"
)

func TestNeedsTranscode(t *testing.T) {
	cases := []struct {
		path string
		info *AudioFileInfo
		want bool
	}{
		{"song.m4a", &AudioFileInfo{Codec: "aac"}, false},  // already target
		{"song.flac", &AudioFileInfo{Codec: "aac"}, true},  // wrong container
		{"song.m4a", &AudioFileInfo{Codec: "alac"}, true},  // wrong codec
		{"song.mp3", &AudioFileInfo{Codec: "mp3"}, true},   // both wrong
		{"song.M4A", &AudioFileInfo{Codec: "aac"}, false},  // extension case-insensitive
	}
	for _, c := range cases {
		if got := NeedsTranscode(c.path, c.info); got != c.want {
			t.Errorf("NeedsTranscode(%q, %+v) = %v, want %v", c.path, c.info, got, c.want)
		}
	}
}

func TestOutputPath(t *testing.T) {
	p := filepath.FromSlash
	cases := []struct {
		name                              string
		input, libType, libPath, outDir   string
		want                              string
	}{
		{
			name:  "empty outputDir writes beside the source",
			input: "/mnt/music/Artist/Album/01.flac", libType: "music", libPath: "/mnt/music", outDir: "",
			want: "/mnt/music/Artist/Album/01.m4a",
		},
		{
			name:  "relative path preserved under outputDir/libType",
			input: "/mnt/music/Artist/Album/01.flac", libType: "music", libPath: "/mnt/music", outDir: "/out",
			want: "/out/music/Artist/Album/01.m4a",
		},
		{
			name:  "m4a input gets a _transcoded suffix",
			input: "/mnt/music/Artist/song.m4a", libType: "music", libPath: "/mnt/music", outDir: "/out",
			want: "/out/music/Artist/song_transcoded.m4a",
		},
		{
			name:  "empty libraryType falls back to flat",
			input: "/x/01.flac", libType: "", libPath: "", outDir: "/out",
			want: "/out/01.m4a",
		},
		{
			name:  "empty libraryPath keeps just the file under libType",
			input: "/x/01.flac", libType: "music", libPath: "", outDir: "/out",
			want: "/out/music/01.m4a",
		},
		{
			name:  "input outside libraryPath falls back to libType root",
			input: "/other/01.flac", libType: "music", libPath: "/mnt/music", outDir: "/out",
			want: "/out/music/01.m4a",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := OutputPath(p(c.input), c.libType, p(c.libPath), p(c.outDir))
			if got != p(c.want) {
				t.Errorf("OutputPath = %q, want %q", got, p(c.want))
			}
		})
	}
}
