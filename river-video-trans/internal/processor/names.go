package processor

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// unsafeFileChars catches characters that are illegal in Windows filenames
// (and a few NAS systems). Linux/macOS are more permissive but the conservative
// set keeps cross-mount compatibility intact.
var unsafeFileChars = regexp.MustCompile(`[\\/:*?"<>|]`)

// sanitizeFilename replaces filesystem-unsafe characters with hyphens,
// collapses runs of hyphens and whitespace, and trims leading/trailing
// dots, spaces, and hyphens. Returns "untitled" for empty input so the
// resulting path is never just an extension.
func sanitizeFilename(name string) string {
	s := unsafeFileChars.ReplaceAllString(name, "-")
	s = strings.Join(strings.Fields(s), " ")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	// Leading/trailing dots and spaces would create hidden files on Unix
	// or be silently trimmed on Windows; leading/trailing hyphens are just ugly.
	s = strings.Trim(s, ". -")
	if s == "" {
		return "untitled"
	}
	return s
}

// movieOutputPath returns the canonical location for a transcoded movie:
//
//	{outputDir}/movies/{Title} ({Year})/{Title}.mp4
//
// Year is omitted (and the parens with it) when 0 — better an unadorned
// folder name than "Title ()" for movies we haven't enriched yet. When
// outputDir is empty the file lands beside the source, since we have no
// canonical root to use.
func movieOutputPath(inputPath, title string, year int, outputDir string) string {
	clean := sanitizeFilename(title)
	folder := clean
	if year > 0 {
		folder = fmt.Sprintf("%s (%d)", clean, year)
	}
	file := clean + ".mp4"
	if outputDir == "" {
		return filepath.Join(filepath.Dir(inputPath), file)
	}
	return filepath.Join(outputDir, "movies", folder, file)
}

// episodeOutputPath returns the canonical location for a transcoded episode:
//
//	{outputDir}/shows/{ShowName}/Season {N}/S{ss:02}E{ee:02}.mp4
//
// Season number in the directory name is left unpadded ("Season 1"); the
// filename is zero-padded to two digits ("S01E03.mp4"). When outputDir is
// empty the file lands beside the source.
func episodeOutputPath(inputPath, showName string, seasonNum, episodeNum int, outputDir string) string {
	file := fmt.Sprintf("S%02dE%02d.mp4", seasonNum, episodeNum)
	if outputDir == "" {
		return filepath.Join(filepath.Dir(inputPath), file)
	}
	return filepath.Join(outputDir, "shows", sanitizeFilename(showName), fmt.Sprintf("Season %d", seasonNum), file)
}

// specialOutputPath is the analogue of episodeOutputPath for files that
// didn't yield a SxxExx number. The special's record-side sequence number
// (assigned by river-meta-tv) is suffixed with SP so the output name stays
// stable across re-runs:
//
//	{outputDir}/shows/{ShowName}/Season {N}/S{ss:02}SP{nn:02}.mp4
func specialOutputPath(inputPath, showName string, seasonNum, specialNum int, outputDir string) string {
	file := fmt.Sprintf("S%02dSP%02d.mp4", seasonNum, specialNum)
	if outputDir == "" {
		return filepath.Join(filepath.Dir(inputPath), file)
	}
	return filepath.Join(outputDir, "shows", sanitizeFilename(showName), fmt.Sprintf("Season %d", seasonNum), file)
}
