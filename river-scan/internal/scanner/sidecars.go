package scanner

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"river-scan/internal/apiclient"
)

// registerMovieSidecars finds sibling audio-variant MP4s and subtitle
// files that belong to a movie video and creates AudioTrack/Subtitle
// records against the given movieID. Called after the movie record
// itself is resolved (find-or-create), regardless of transcode mode:
// the sidecars exist on disk either way, and the transcoder path only
// handles subtitles co-located with the source video (not files under
// a `subtitles/` subdir) and doesn't understand loose `.audio_N.mp4`
// variants at all.
//
// Idempotent — each side's ListByMedia is queried up front and any
// sidecar whose file_path is already registered is skipped. Errors on
// individual sidecars are logged, not returned; a single bad file
// shouldn't fail the whole scan.
func (s *Scanner) registerMovieSidecars(movieID, videoPath string) {
	tracks, err := s.api.ListMovieAudioTracks(movieID)
	if err != nil {
		log.Printf("WARN list movie audio tracks for sidecar dedupe: %v", err)
	}
	subs, err := s.api.ListMovieSubtitles(movieID)
	if err != nil {
		log.Printf("WARN list movie subtitles for sidecar dedupe: %v", err)
	}
	s.registerSidecars("movie", movieID, videoPath, tracks, subs)
}

// registerEpisodeSidecars is the episode-scoped twin of the movie
// variant above. It takes the show/season/episode IDs so it can hit
// the nested list endpoints for de-duplication.
func (s *Scanner) registerEpisodeSidecars(showID, seasonID, episodeID, videoPath string) {
	tracks, err := s.api.ListEpisodeAudioTracks(showID, seasonID, episodeID)
	if err != nil {
		log.Printf("WARN list episode audio tracks for sidecar dedupe: %v", err)
	}
	subs, err := s.api.ListEpisodeSubtitles(showID, seasonID, episodeID)
	if err != nil {
		log.Printf("WARN list episode subtitles for sidecar dedupe: %v", err)
	}
	s.registerSidecars("episode", episodeID, videoPath, tracks, subs)
}

// registerSidecars is the shared implementation.
//
// It looks for two artifacts co-located with videoPath:
//
//  1. `<dir>/<stem>.audio_<N>.<vidext>` — per-audio-track variant MP4s
//     produced by river-video-trans. Registered as AudioTrack records
//     with StreamIndex=N; Language is left empty because the filename
//     encodes the stream index, not a language tag. The consuming UI
//     surfaces these as "Track 1", "Track 2", … when Language is blank.
//
//  2. `<dir>/{subtitles,subs,sub}/<stem>.<lang>.vtt` (case-insensitive
//     directory name) — subtitle sidecars produced by the transcoder.
//     Language is parsed from the filename segment between the video
//     stem and `.vtt`; multi-segment tags (e.g. `en.forced`) collapse
//     to the first segment.
func (s *Scanner) registerSidecars(
	mediaType, mediaID, videoPath string,
	existingTracks []apiclient.AudioTrack,
	existingSubs []apiclient.Subtitle,
) {
	dir := filepath.Dir(videoPath)
	stem := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// -------- Audio variants (siblings) --------
	tracksByPath := make(map[string]bool, len(existingTracks))
	for _, t := range existingTracks {
		tracksByPath[t.FilePath] = true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("WARN read %s for audio sidecars: %v", dir, err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !videoExtensions[ext] {
			continue
		}
		idx, ok := audioVariantIndex(name, stem)
		if !ok {
			continue
		}
		full := filepath.Join(dir, name)
		if tracksByPath[full] {
			continue
		}
		label := fmt.Sprintf("Track %d", idx+1)
		// river-api requires a non-empty language on POST /audio-tracks
		// (validation tag), so fall back to "und" (ISO 639-2 for
		// "undetermined") when the filename encodes only the stream
		// index and not the language. UI code that formats language
		// codes maps "und" to "Unknown".
		if _, err := s.api.CreateAudioTrack(apiclient.AudioTrackRequest{
			MediaType:   mediaType,
			MediaID:     mediaID,
			Language:    "und",
			Label:       label,
			StreamIndex: idx,
			FilePath:    full,
		}); err != nil {
			log.Printf("WARN register audio variant %s: %v", full, err)
			continue
		}
		log.Printf("INFO registered audio variant %d for %s %s → %s", idx, mediaType, mediaID, filepath.Base(full))
	}

	// -------- Subtitle sidecars (subdirectory) --------
	subDir := findSubtitleSubdir(dir)
	if subDir == "" {
		return
	}
	subsByPath := make(map[string]bool, len(existingSubs))
	for _, sub := range existingSubs {
		subsByPath[sub.FilePath] = true
	}
	subEntries, err := os.ReadDir(subDir)
	if err != nil {
		log.Printf("WARN read %s for subtitle sidecars: %v", subDir, err)
		return
	}
	for _, e := range subEntries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.ToLower(filepath.Ext(name)) != ".vtt" {
			continue
		}
		lang, ok := subtitleLangForStem(name, stem)
		if !ok {
			continue
		}
		full := filepath.Join(subDir, name)
		if subsByPath[full] {
			continue
		}
		if _, err := s.api.CreateSubtitle(apiclient.SubtitleRequest{
			MediaType: mediaType,
			MediaID:   mediaID,
			Language:  lang,
			Label:     languageLabel(lang),
			FilePath:  full,
		}); err != nil {
			log.Printf("WARN register subtitle %s: %v", full, err)
			continue
		}
		log.Printf("INFO registered subtitle %q for %s %s → %s", lang, mediaType, mediaID, filepath.Base(full))
	}
}

// audioVariantIndex parses the stream index out of a filename like
// "<videoStem>.audio_<N>.<vidext>". Returns (N, true) on match, else
// (0, false). Comparison is exact on the stem (case-sensitive) because
// the transcoder uses the movie's canonical title verbatim.
func audioVariantIndex(name, videoStem string) (int, bool) {
	ext := filepath.Ext(name)
	if !videoExtensions[strings.ToLower(ext)] {
		return 0, false
	}
	base := name[:len(name)-len(ext)]
	prefix := videoStem + ".audio_"
	if !strings.HasPrefix(base, prefix) {
		return 0, false
	}
	idx, err := strconv.Atoi(base[len(prefix):])
	if err != nil {
		return 0, false
	}
	return idx, true
}

// subtitleLangForStem returns the language segment of a filename like
// "<videoStem>.<lang>.vtt". `<lang>` may itself contain further dot-
// separated qualifiers (e.g. "en.forced") — only the first segment is
// returned. A file named exactly "<videoStem>.vtt" is treated as
// undetermined ("und").
//
// The comparison is case-insensitive so a filesystem that lowercased
// the parent folder still lines up.
func subtitleLangForStem(name, videoStem string) (string, bool) {
	if !strings.EqualFold(filepath.Ext(name), ".vtt") {
		return "", false
	}
	base := name[:len(name)-len(filepath.Ext(name))]
	if strings.EqualFold(base, videoStem) {
		return "und", true
	}
	prefix := strings.ToLower(videoStem) + "."
	if !strings.HasPrefix(strings.ToLower(base), prefix) {
		return "", false
	}
	suffix := base[len(prefix):]
	if idx := strings.Index(suffix, "."); idx != -1 {
		suffix = suffix[:idx]
	}
	if suffix == "" {
		return "und", true
	}
	return suffix, true
}

// findSubtitleSubdir locates a subtitle sidecar directory alongside the
// media file. river-video-trans writes to lowercase "subtitles"; we
// also accept the common alternatives "subs" and "sub" case-insen-
// sitively so libraries curated by hand line up too.
func findSubtitleSubdir(parentDir string) string {
	entries, err := os.ReadDir(parentDir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		switch strings.ToLower(e.Name()) {
		case "subtitles", "subs", "sub":
			return filepath.Join(parentDir, e.Name())
		}
	}
	return ""
}

// languageLabel maps common BCP-47 (and legacy ISO 639-2/B) language
// codes to display names. Unmapped codes are returned as-is so 3-letter
// tags like "eng" or "ita" still surface something rather than empty.
// Mirrors the same table in river-video-trans; we deliberately keep a
// copy here to avoid a cross-service import for a 20-line lookup.
func languageLabel(lang string) string {
	labels := map[string]string{
		"en": "English", "eng": "English",
		"fr": "French", "fre": "French", "fra": "French",
		"de": "German", "ger": "German", "deu": "German",
		"es": "Spanish", "spa": "Spanish",
		"it": "Italian", "ita": "Italian",
		"pt": "Portuguese", "por": "Portuguese",
		"ru": "Russian", "rus": "Russian",
		"ja": "Japanese", "jpn": "Japanese",
		"zh": "Chinese", "chi": "Chinese", "zho": "Chinese",
		"ko": "Korean", "kor": "Korean",
		"ar": "Arabic", "ara": "Arabic",
		"nl": "Dutch", "dut": "Dutch", "nld": "Dutch",
		"pl": "Polish", "pol": "Polish",
		"sv": "Swedish", "swe": "Swedish",
		"no": "Norwegian", "nor": "Norwegian",
		"da": "Danish", "dan": "Danish",
		"fi": "Finnish", "fin": "Finnish",
		"cs": "Czech", "cze": "Czech", "ces": "Czech",
		"tr": "Turkish", "tur": "Turkish",
		"hu": "Hungarian", "hun": "Hungarian",
		"und": "Unknown",
	}
	if l, ok := labels[strings.ToLower(lang)]; ok {
		return l
	}
	return lang
}
