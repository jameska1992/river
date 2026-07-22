package scanner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// defaultMaxDepth bounds recursive walking so a misconfigured library root
// or a symlink loop can't run away. Plex/Jellyfin recommend ≤3 levels of
// nesting for movies (e.g. Movies/Genre/Title/file); 6 is comfortably above
// that and still cheap to walk over NFS.
const defaultMaxDepth = 6

// systemDirs are directories that should never be descended into. NAS
// platforms (Synology @eaDir, macOS .AppleDouble) and various filesystem
// scaffolding live here. We also skip Plex-convention "extras" subdirs —
// the file inside is not the feature.
var systemDirs = map[string]bool{
	"@eadir":                    true, // Synology NAS thumb cache
	".appledouble":              true,
	"lost+found":                true,
	"$recycle.bin":              true,
	"system volume information": true,
	"extras":                    true,
	"behind the scenes":         true,
	"behindthescenes":           true,
	"featurettes":               true,
	"trailers":                  true,
	"interviews":                true,
	"deleted scenes":            true,
	"deletedscenes":             true,
	"bloopers":                  true,
	"shorts":                    true,
	"scenes":                    true,
	"other":                     true,
	// Sidecar subtitle directories — river-video-trans writes .vtt files
	// under a "subtitles" subdir of the movie/episode's home folder. We
	// don't want the walker crawling into it looking for movies, and any
	// video file that ever landed there is by definition not a feature.
	"subtitles":                 true,
	"subs":                      true,
	"sub":                       true,
}

// skipDir reports whether walkMovieFiles should refuse to descend into name.
// Hidden directories (any leading dot) and the systemDirs denylist are
// rejected. Comparison is case-insensitive — NAS exports often have mixed
// casing for "EXTRAS" / "Featurettes".
func skipDir(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	return systemDirs[strings.ToLower(name)]
}

// extraSuffixes are Plex/Jellyfin filename markers for non-feature content
// that lives alongside the movie file. A video whose basename ends in one
// of these (case-insensitive) is treated as an extra and skipped, so we
// don't accidentally register "Inception-trailer.mkv" as its own movie.
var extraSuffixes = []string{
	"-trailer",
	"-behindthescenes",
	"-deleted",
	"-deletedscene",
	"-featurette",
	"-interview",
	"-scene",
	"-short",
	"-other",
	"-extra",
	"-bloopers",
	"-clip",
	"-sample",
}

// audioVariantSuffixRe matches the ".audio_N" suffix river-video-trans
// appends to per-audio-track variant MP4 files (see the processor's
// registerAudioTracks). When the scanner is later pointed at an output
// tree — e.g. a pre-transcoded library — we don't want those variants
// to be treated as separate feature films.
var audioVariantSuffixRe = regexp.MustCompile(`\.audio_\d+$`)

// audioVariantSpaceSuffixRe matches the same variants after nameinfo-
// style separator normalisation ("._-" → space), for defence against
// a legacy failure mode where the OLD (unfiltered) scanner registered
// each variant as its own "Movie audio N" movie, and the transcoder
// then wrote a copy to a directory whose folder name and inner file-
// name both include " audio N". Those files look like
// `Air Force One audio 0.mp4` on disk; matching this pattern keeps
// them out even though no dot-underscore is left.
var audioVariantSpaceSuffixRe = regexp.MustCompile(`(?i)\baudio\s+\d+$`)

// isAudioVariantFile reports whether path looks like a river-video-trans
// audio variant (either the canonical `.audio_N.<ext>` form the trans-
// coder writes today, or the space-normalised `<Title> audio N.<ext>`
// form left behind by the legacy duplication bug). Only video-extension
// files can be variants.
func isAudioVariantFile(path string) bool {
	if !videoExtensions[strings.ToLower(filepath.Ext(path))] {
		return false
	}
	stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	return audioVariantSuffixRe.MatchString(stem) ||
		audioVariantSpaceSuffixRe.MatchString(stem)
}

// isExtraFile reports whether path is a Plex-convention "extra" rather than
// a feature film. Extension is stripped before suffix matching so the rule
// works on .mkv / .mp4 / etc. alike.
func isExtraFile(path string) bool {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	for _, s := range extraSuffixes {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	// "Sample.mkv" as a sibling to the feature is also common in scene releases.
	if base == "sample" {
		return true
	}
	return false
}

// walkMovieFiles walks root recursively up to maxDepth, calling fn for each
// video file encountered. System/extras directories are skipped; extras
// files are skipped. Symlinks are NOT followed (avoids loops on NAS mounts
// that cross-link libraries). Read errors on subdirs are logged via fn's
// caller — walkMovieFiles itself never returns mid-walk on a single error,
// since a partial scan beats no scan at all when one subtree is unreadable.
//
// fn receives the absolute file path. Returning an error from fn aborts the
// walk and is propagated to the caller.
func walkMovieFiles(root string, maxDepth int, fn func(path string) error) error {
	if maxDepth <= 0 {
		maxDepth = defaultMaxDepth
	}
	return walkDepth(root, 0, maxDepth, fn)
}

func walkDepth(dir string, depth, maxDepth int, fn func(path string) error) error {
	if depth > maxDepth {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Unreadable subtree: stop here, but don't fail the whole scan.
		return nil
	}
	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(dir, name)
		if e.IsDir() {
			if skipDir(name) {
				continue
			}
			if err := walkDepth(full, depth+1, maxDepth, fn); err != nil {
				return err
			}
			continue
		}
		// Files: only video extensions, skip extras.
		if !videoExtensions[strings.ToLower(filepath.Ext(name))] {
			continue
		}
		if isExtraFile(full) {
			continue
		}
		if isAudioVariantFile(full) {
			continue
		}
		if err := fn(full); err != nil {
			return err
		}
	}
	return nil
}
