package processor

import (
	"testing"

	"river-meta-movie/internal/consumer"
)

func TestParentDirHint(t *testing.T) {
	cases := []struct {
		name  string
		event consumer.MediaDiscoveredEvent
		want  string
	}{
		{
			name: "curated subfolder yields its parsed title",
			event: consumer.MediaDiscoveredEvent{
				LibraryPath:   "/lib/Movies",
				DirectoryPath: "/lib/Movies/Inception (2010)",
				DirectoryName: "Inception (2010)",
			},
			want: "Inception",
		},
		{
			name: "containing dir IS library root → no hint",
			event: consumer.MediaDiscoveredEvent{
				LibraryPath:   "/lib/Movies",
				DirectoryPath: "/lib/Movies",
				DirectoryName: "Movies",
			},
			want: "",
		},
		{
			name: "trailing slash on library path doesn't matter",
			event: consumer.MediaDiscoveredEvent{
				LibraryPath:   "/lib/Movies/",
				DirectoryPath: "/lib/Movies",
				DirectoryName: "Movies",
			},
			want: "",
		},
		{
			name: "release-named subfolder gets normalized",
			event: consumer.MediaDiscoveredEvent{
				LibraryPath:   "/lib/Movies",
				DirectoryPath: "/lib/Movies/The.Matrix.1999.1080p.BluRay",
				DirectoryName: "The.Matrix.1999.1080p.BluRay",
			},
			want: "The Matrix",
		},
		{
			name: "missing LibraryPath → no hint (can't tell)",
			event: consumer.MediaDiscoveredEvent{
				LibraryPath:   "",
				DirectoryPath: "/lib/Movies/Inception (2010)",
				DirectoryName: "Inception (2010)",
			},
			want: "",
		},
		{
			name: "nested category folder above curated folder",
			event: consumer.MediaDiscoveredEvent{
				LibraryPath:   "/lib/Movies",
				DirectoryPath: "/lib/Movies/Action/Inception (2010)",
				DirectoryName: "Inception (2010)",
			},
			want: "Inception",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parentDirHint(tc.event); got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
