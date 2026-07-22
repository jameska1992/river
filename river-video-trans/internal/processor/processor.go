package processor

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"river-video-trans/internal/apiclient"
	"river-video-trans/internal/consumer"
	"river-video-trans/internal/transcoder"
)

// maxPostTranscodeConcurrency bounds parallel ffmpeg helpers (audio-variant
// creation, subtitle extraction) within a single event. Multiple events
// still run in parallel across WORKER_COUNT goroutines, so the total
// number of concurrent ffmpeg processes is WORKER_COUNT × this.
const maxPostTranscodeConcurrency = 3

// runConcurrent runs each job with at most max concurrent goroutines.
// Jobs are expected to handle their own errors (post-transcode work is
// best-effort — a failed subtitle extract logs a warning, doesn't fail
// the whole event).
func runConcurrent(jobs []func(), max int) {
	if max <= 0 {
		max = 1
	}
	sem := make(chan struct{}, max)
	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			job()
		}()
	}
	wg.Wait()
}

var subtitleExts = map[string]bool{
	".srt": true, ".vtt": true, ".ass": true, ".ssa": true,
}

var videoExts = map[string]bool{
	".mkv": true, ".mp4": true, ".avi": true, ".mov": true,
	".m4v": true, ".ts": true, ".wmv": true, ".webm": true, ".flv": true,
}

type Processor struct {
	api       *apiclient.Client
	outputDir string
}

func New(api *apiclient.Client, outputDir string) *Processor {
	return &Processor{api: api, outputDir: outputDir}
}

// tmpDir returns the scratch directory for in-flight transcodes. It lives
// under outputDir so the final move into place is a same-filesystem rename
// (no extra copy). Empty when outputDir is unset — the transcoder then
// falls back to a sibling of the final path, preserving the same guarantee.
func (p *Processor) tmpDir() string {
	if p.outputDir == "" {
		return ""
	}
	return filepath.Join(p.outputDir, "temp")
}

func (p *Processor) Handle(event consumer.MediaDiscoveredEvent) error {
	log.Printf("INFO processing event %s type=%s dir=%q files=%d",
		event.EventID, event.LibraryType, event.DirectoryName, len(event.Files))

	// Pre-transcoded libraries: the scanner has already registered the
	// media record with the source file as its stream path, and any
	// audio-variant / subtitle sidecars alongside it. Nothing for the
	// transcoder to do — ACK and move on. This runs BEFORE the type
	// switch so no per-type branch has to remember the check.
	if event.PreTranscoded {
		log.Printf("INFO skipping pre-transcoded event %s (%s / %q)",
			event.EventID, event.LibraryType, event.DirectoryName)
		return nil
	}

	switch event.LibraryType {
	case "movie":
		return p.processMovie(event)
	case "tvshow":
		return p.processTVShow(event)
	default:
		log.Printf("INFO skipping unsupported library type %q", event.LibraryType)
		return nil
	}
}

// processMovie picks the largest video file in the event, resolves the
// movie record so we know its canonical title, then transcodes if needed.
// The transcoded file is named "{movie.Title}.mp4" so it stays in sync
// with the admin "identify" flow (a renamed movie gets a renamed output
// on the next scan).
func (p *Processor) processMovie(event consumer.MediaDiscoveredEvent) error {
	videoFile := largestVideoFile(event.Files)
	if videoFile == "" {
		log.Printf("WARN no video files in event %s", event.EventID)
		return nil
	}

	existing, err := p.resolveMovie(event)
	if err != nil {
		return err
	}

	// Title/year for naming purposes: prefer the resolved record, else the
	// parsed dir name. The same values are used to create the record below
	// if no match was found.
	var title string
	var year int
	if existing != nil {
		title = existing.Title
		year = existing.Year
	} else {
		title, year = parseDirName(event.DirectoryName)
	}

	info, finalPath, err := p.processMovieFile(videoFile, title, year)
	if err != nil {
		return fmt.Errorf("process file %q: %w", videoFile, err)
	}

	movieID := ""
	if existing != nil {
		movieID = existing.ID
		if existing.FilePath != finalPath {
			log.Printf("INFO updating movie %q file_path → %s", existing.Title, finalPath)
			// Targeted PATCH — sending the full MovieRequest here would race
			// with river-meta-movie writing TMDB data during the transcode
			// window and clobber whatever metadata landed in the meantime.
			if err := p.api.UpdateMovieFilePath(existing.ID, finalPath); err != nil {
				return err
			}
		}
	} else {
		log.Printf("INFO creating movie %q year=%d", title, year)
		created, err := p.api.CreateMovie(apiclient.MovieRequest{
			LibraryID:  event.LibraryID,
			Title:      title,
			Year:       year,
			FilePath:   finalPath,
			SourcePath: videoFile,
		})
		if err != nil {
			return err
		}
		movieID = created.ID
	}

	p.registerAudioTracks("movie", movieID, finalPath, info)
	p.registerSubtitles("movie", movieID, videoFile, finalPath, info)
	return nil
}

// resolveMovie returns the movie record for the event. MediaID is the fast
// path (set by the post-scanner-rewrite scanner); the fallback list+match is
// only hit for legacy queue entries published before file-as-unit scanning.
// Returns (nil, nil) when no match — caller creates the record.
//
// Matching is title + year (case-insensitive title, exact year). Matching
// on title alone would collapse two different-year movies sharing a name
// onto the first record created, and a subsequent file_path update would
// silently overwrite the other movie's pointer to its file.
func (p *Processor) resolveMovie(event consumer.MediaDiscoveredEvent) (*apiclient.Movie, error) {
	if event.MediaID != "" {
		m, err := p.api.GetMovie(event.MediaID)
		if err != nil {
			return nil, fmt.Errorf("get movie %s: %w", event.MediaID, err)
		}
		return m, nil
	}
	title, year := parseDirName(event.DirectoryName)
	movies, err := p.api.ListMovies(event.LibraryID)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	var fallback *apiclient.Movie
	for i := range movies {
		if !strings.EqualFold(strings.TrimSpace(movies[i].Title), strings.TrimSpace(title)) {
			continue
		}
		if year > 0 && movies[i].Year == year {
			return &movies[i], nil
		}
		// Title-only fallback is only allowed when one side has no year
		// — otherwise a yeared event would alias onto a different-year
		// record and the transcoder would overwrite its file_path.
		if (year == 0 || movies[i].Year == 0) && fallback == nil {
			fallback = &movies[i]
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	return nil, nil
}

// processTVShow finds or creates the show + season, then creates an episode
// entry for each video file it hasn't seen before. The output directory
// uses the show's canonical title from river-api so admin renames flow
// through to disk on the next scan.
func (p *Processor) processTVShow(event consumer.MediaDiscoveredEvent) error {
	var showID, seasonID, showName string
	seasonNum := parseSeasonNumber(event.SeasonName)

	if event.MediaID != "" && event.SeasonID != "" {
		showID = event.MediaID
		seasonID = event.SeasonID
		show, err := p.api.GetTVShow(event.MediaID)
		if err != nil {
			return fmt.Errorf("get tvshow %s: %w", event.MediaID, err)
		}
		showName = show.Title
	} else {
		// Fallback for events without IDs (e.g. pre-existing queue entries).
		show, err := p.findOrCreateShow(event.LibraryID, event.DirectoryName)
		if err != nil {
			return fmt.Errorf("find/create show: %w", err)
		}
		season, err := p.findOrCreateSeason(show.ID, seasonNum)
		if err != nil {
			return fmt.Errorf("find/create season %d: %w", seasonNum, err)
		}
		showID = show.ID
		seasonID = season.ID
		showName = show.Title
	}

	existing, err := p.api.ListEpisodes(showID, seasonID)
	if err != nil {
		return fmt.Errorf("list episodes: %w", err)
	}
	episodeIDByNum := make(map[int]string, len(existing))
	// Specials are matched by source_path because we don't have a parseable
	// number from the filename — river-meta-tv stamps SourcePath at create
	// so this lookup works as long as meta-tv has run for the season.
	specialBySource := make(map[string]apiclient.Episode, len(existing))
	for _, ep := range existing {
		if ep.IsSpecial {
			if ep.SourcePath != "" {
				specialBySource[ep.SourcePath] = ep
			}
			continue
		}
		episodeIDByNum[ep.Number] = ep.ID
	}

	for _, file := range event.Files {
		if !isVideoFile(file) {
			continue
		}
		epNum := parseEpisodeNumber(file)
		if epNum == 0 {
			p.processSpecialFile(file, showID, seasonID, showName, seasonNum, specialBySource)
			continue
		}

		info, finalPath, err := p.processEpisodeFile(file, showName, seasonNum, epNum)
		if err != nil {
			log.Printf("ERROR processing file %q: %v", file, err)
			continue
		}

		var epID string
		if existing, exists := episodeIDByNum[epNum]; exists {
			log.Printf("INFO updating file_path for episode S%02dE%02d", seasonNum, epNum)
			// Also send SourcePath so rows created before this field existed
			// get backfilled on their next reprocessing. PATCH semantics on
			// the server skip empty fields, but file (the source) is always
			// set here.
			_, err = p.api.UpdateEpisode(showID, seasonID, existing, apiclient.EpisodeRequest{
				FilePath:   finalPath,
				SourcePath: file,
			})
			if err != nil {
				log.Printf("ERROR update episode S%02dE%02d: %v", seasonNum, epNum, err)
				continue
			}
			epID = existing
		} else {
			log.Printf("INFO creating episode S%02dE%02d", seasonNum, epNum)
			ep, err := p.api.CreateEpisode(showID, seasonID, apiclient.EpisodeRequest{
				Number:     epNum,
				FilePath:   finalPath,
				SourcePath: file,
			})
			if err != nil {
				log.Printf("ERROR create episode S%02dE%02d: %v", seasonNum, epNum, err)
				continue
			}
			epID = ep.ID
			episodeIDByNum[epNum] = ep.ID
		}
		p.registerAudioTracks("episode", epID, finalPath, info)
		p.registerSubtitles("episode", epID, file, finalPath, info)
	}
	return nil
}

// processSpecialFile transcodes a special episode and patches its file_path.
// The episode record itself is created by river-meta-tv (which owns the
// SPxx numbering); we only proceed when that record already exists. If
// meta-tv hasn't seen this special yet, we skip — the next scan retries.
func (p *Processor) processSpecialFile(
	file, showID, seasonID, showName string,
	seasonNum int,
	specialBySource map[string]apiclient.Episode,
) {
	ep, found := specialBySource[file]
	if !found {
		log.Printf("WARN special %q has no episode record yet (meta-tv hasn't run); will retry next scan", filepath.Base(file))
		return
	}
	outPath := specialOutputPath(file, showName, seasonNum, ep.Number, p.outputDir)
	info, finalPath, err := p.processEpisodeFileTo(file, outPath)
	if err != nil {
		log.Printf("ERROR processing special %q: %v", file, err)
		return
	}
	log.Printf("INFO updating file_path for special S%02dSP%02d", seasonNum, ep.Number)
	if _, err := p.api.UpdateEpisode(showID, seasonID, ep.ID, apiclient.EpisodeRequest{
		FilePath:   finalPath,
		SourcePath: file,
	}); err != nil {
		log.Printf("ERROR update special S%02dSP%02d: %v", seasonNum, ep.Number, err)
		return
	}
	p.registerAudioTracks("episode", ep.ID, finalPath, info)
	p.registerSubtitles("episode", ep.ID, file, finalPath, info)
}

// processMovieFile probes and transcodes the file if necessary, writing
// the output to {outputDir}/{Title} ({Year})/{Title}.mp4. Returns the
// probe info (for subtitle extraction) and the final path stored on the
// movie record.
func (p *Processor) processMovieFile(path, title string, year int) (*transcoder.FileInfo, string, error) {
	info, err := transcoder.Probe(path)
	if err != nil {
		return nil, "", fmt.Errorf("probe: %w", err)
	}

	outPath := movieOutputPath(path, title, year, p.outputDir)

	// Always check for an existing output first — even if the source happens to
	// be a compatible format and would otherwise short-circuit before this check.
	if _, err := os.Stat(outPath); err == nil {
		log.Printf("INFO transcoded output already exists: %q", outPath)
		p.api.Log("info", fmt.Sprintf("output already exists for movie %s", filepath.Base(outPath)))
		return info, outPath, nil
	}

	if !transcoder.NeedsTranscode(path, info) {
		if filepath.Clean(path) == filepath.Clean(outPath) {
			log.Printf("INFO %q is already at output path, no copy needed", filepath.Base(path))
			return info, outPath, nil
		}
		log.Printf("INFO %q is already h264/aac/mp4 ≤1080p, copying to %q", filepath.Base(path), outPath)
		p.api.Log("info", fmt.Sprintf("copying movie %s", filepath.Base(outPath)))
		if err := copyFile(path, outPath); err != nil {
			p.api.Log("error", fmt.Sprintf("copy failed for %s: %v", filepath.Base(outPath), err))
			return nil, "", fmt.Errorf("copy: %w", err)
		}
		log.Printf("INFO copy complete: %q", outPath)
		p.api.Log("info", fmt.Sprintf("copied movie %s", filepath.Base(outPath)))
		return info, outPath, nil
	}

	log.Printf("INFO transcoding %q → %q (codec=%s/%s %dx%d)",
		filepath.Base(path), filepath.Base(outPath),
		info.VideoCodec, info.AudioCodec, info.Width, info.Height)
	p.api.Log("info", fmt.Sprintf("transcoding movie %s", filepath.Base(outPath)))

	if err := transcoder.Transcode(path, outPath, p.tmpDir(), info, p.api); err != nil {
		p.api.Log("error", fmt.Sprintf("transcode failed for %s: %v", filepath.Base(outPath), err))
		return nil, "", fmt.Errorf("transcode: %w", err)
	}
	log.Printf("INFO transcode complete: %q", outPath)
	p.api.Log("info", fmt.Sprintf("transcoded movie %s", filepath.Base(outPath)))
	return info, outPath, nil
}

// processEpisodeFile probes and transcodes a TV episode if necessary,
// writing the output to {outputDir}/{ShowName}/Season N/S{ss:02}E{ee:02}.mp4.
// Same probe/early-return semantics as processMovieFile.
func (p *Processor) processEpisodeFile(path, showName string, seasonNum, episodeNum int) (*transcoder.FileInfo, string, error) {
	outPath := episodeOutputPath(path, showName, seasonNum, episodeNum, p.outputDir)
	return p.processEpisodeFileTo(path, outPath)
}

// processEpisodeFileTo does the probe / no-transcode-needed / copy / transcode
// dance against a caller-supplied output path. processEpisodeFile is a thin
// wrapper that resolves the standard SxxExx path; specials use specialOutputPath.
func (p *Processor) processEpisodeFileTo(path, outPath string) (*transcoder.FileInfo, string, error) {
	info, err := transcoder.Probe(path)
	if err != nil {
		return nil, "", fmt.Errorf("probe: %w", err)
	}

	if _, err := os.Stat(outPath); err == nil {
		log.Printf("INFO transcoded output already exists: %q", outPath)
		p.api.Log("info", fmt.Sprintf("output already exists for episode %s", filepath.Base(outPath)))
		return info, outPath, nil
	}

	if !transcoder.NeedsTranscode(path, info) {
		if filepath.Clean(path) == filepath.Clean(outPath) {
			log.Printf("INFO %q is already at output path, no copy needed", filepath.Base(path))
			return info, outPath, nil
		}
		log.Printf("INFO %q is already h264/aac/mp4 ≤1080p, copying to %q", filepath.Base(path), outPath)
		p.api.Log("info", fmt.Sprintf("copying episode %s", filepath.Base(outPath)))
		if err := copyFile(path, outPath); err != nil {
			p.api.Log("error", fmt.Sprintf("copy failed for %s: %v", filepath.Base(outPath), err))
			return nil, "", fmt.Errorf("copy: %w", err)
		}
		log.Printf("INFO copy complete: %q", outPath)
		p.api.Log("info", fmt.Sprintf("copied episode %s", filepath.Base(outPath)))
		return info, outPath, nil
	}

	log.Printf("INFO transcoding %q → %q (codec=%s/%s %dx%d)",
		filepath.Base(path), filepath.Base(outPath),
		info.VideoCodec, info.AudioCodec, info.Width, info.Height)
	p.api.Log("info", fmt.Sprintf("transcoding episode %s", filepath.Base(outPath)))

	if err := transcoder.Transcode(path, outPath, p.tmpDir(), info, p.api); err != nil {
		p.api.Log("error", fmt.Sprintf("transcode failed for %s: %v", filepath.Base(outPath), err))
		return nil, "", fmt.Errorf("transcode: %w", err)
	}
	log.Printf("INFO transcode complete: %q", outPath)
	p.api.Log("info", fmt.Sprintf("transcoded episode %s", filepath.Base(outPath)))
	return info, outPath, nil
}

// copyFile copies src to dst, creating dst's parent directory. The bytes are
// streamed to a sibling ".tmp" file and renamed into place atomically, so a
// process crash mid-copy leaves either the prior dst or no dst at all —
// never a half-written file at the canonical path.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("create dir: %w", err)
	}
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("copy: %w", err)
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("close dest: %w", err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
}


// registerAudioTracks creates per-language variant MP4 files (video + one audio
// track, streams copied) and registers AudioTrack records in river-api.
// Only runs when there are ≥2 audio streams. Non-fatal: errors are only logged.
//
// Variant naming: given finalPath = /out/movies/Foo/Foo.mp4, variants are
// /out/movies/Foo/Foo.audio_0.mp4, /out/movies/Foo/Foo.audio_1.mp4, …
func (p *Processor) registerAudioTracks(mediaType, mediaID, finalPath string, info *transcoder.FileInfo) {
	if len(info.AudioStreams) < 2 {
		return
	}
	stem := finalPath[:len(finalPath)-len(filepath.Ext(finalPath))]

	jobs := make([]func(), 0, len(info.AudioStreams))
	for _, a := range info.AudioStreams {
		a := a
		jobs = append(jobs, func() {
			lang := a.Language
			if lang == "" {
				lang = fmt.Sprintf("track%d", a.Index)
			}
			label := a.Title
			if label == "" {
				label = languageLabel(lang)
			}
			variantPath := fmt.Sprintf("%s.audio_%d.mp4", stem, a.Index)
			if _, err := os.Stat(variantPath); err != nil {
				log.Printf("INFO creating audio variant %d (%s) for %s", a.Index, lang, filepath.Base(finalPath))
				if err := transcoder.CreateVariant(finalPath, a.Index, variantPath); err != nil {
					log.Printf("WARN create audio variant %d: %v", a.Index, err)
					return
				}
			}
			if _, err := p.api.CreateAudioTrack(apiclient.AudioTrackRequest{
				MediaType:   mediaType,
				MediaID:     mediaID,
				Language:    lang,
				Label:       label,
				StreamIndex: a.Index,
				FilePath:    variantPath,
			}); err != nil {
				log.Printf("WARN register audio track %q for %s %s: %v", lang, mediaType, mediaID, err)
			}
		})
	}
	runConcurrent(jobs, maxPostTranscodeConcurrency)
}

// registerSubtitles extracts embedded subtitle streams from srcPath (original
// source file) and also detects sidecar subtitle files, then creates Subtitle
// records in river-api for each. Non-fatal: errors are only logged.
func (p *Processor) registerSubtitles(mediaType, mediaID, srcPath, finalPath string, info *transcoder.FileInfo) {
	subtitleDir := filepath.Join(filepath.Dir(finalPath), "subtitles")
	finalStem := filepath.Base(finalPath)
	finalStem = finalStem[:len(finalStem)-len(filepath.Ext(finalStem))]

	var jobs []func()

	// 1. Embedded subtitle streams from the source file.
	for i, sub := range info.Subtitles {
		i, sub := i, sub
		jobs = append(jobs, func() {
			lang := sub.Language
			if lang == "" {
				lang = fmt.Sprintf("sub%d", i)
			}
			label := sub.Title
			if label == "" {
				label = languageLabel(lang)
			}
			outPath := filepath.Join(subtitleDir, fmt.Sprintf("%s.%s.vtt", finalStem, lang))
			if _, err := os.Stat(outPath); err != nil {
				if err := transcoder.ExtractSubtitle(srcPath, sub.Index, outPath); err != nil {
					log.Printf("WARN extract subtitle stream %d from %q: %v", sub.Index, filepath.Base(srcPath), err)
					return
				}
			}
			if _, err := p.api.CreateSubtitle(apiclient.SubtitleRequest{
				MediaType: mediaType,
				MediaID:   mediaID,
				Language:  lang,
				Label:     label,
				FilePath:  outPath,
			}); err != nil {
				log.Printf("WARN register subtitle %q: %v", outPath, err)
			}
		})
	}

	// 2. Sidecar subtitle files co-located with the source file.
	dir := filepath.Dir(srcPath)
	stem := filepath.Base(srcPath)
	stem = stem[:len(stem)-len(filepath.Ext(stem))]
	entries, _ := os.ReadDir(dir)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !subtitleExts[ext] {
			continue
		}
		base := name[:len(name)-len(ext)]
		if !strings.EqualFold(base, stem) && !strings.HasPrefix(strings.ToLower(base), strings.ToLower(stem)+".") {
			continue
		}
		jobs = append(jobs, func() {
			lang := sidecarLang(base, stem)
			label := languageLabel(lang)

			sidecarPath := filepath.Join(dir, name)
			var vttPath string
			if ext == ".vtt" {
				vttPath = sidecarPath
			} else {
				// Convert srt/ass/ssa → vtt
				vttName := base + ".vtt"
				vttPath = filepath.Join(subtitleDir, vttName)
				if _, err := os.Stat(vttPath); err != nil {
					if err := transcoder.ExtractSubtitle(sidecarPath, 0, vttPath); err != nil {
						log.Printf("WARN convert sidecar %q → vtt: %v", name, err)
						return
					}
				}
			}
			if _, err := p.api.CreateSubtitle(apiclient.SubtitleRequest{
				MediaType: mediaType,
				MediaID:   mediaID,
				Language:  lang,
				Label:     label,
				FilePath:  vttPath,
			}); err != nil {
				log.Printf("WARN register sidecar subtitle %q: %v", vttPath, err)
			}
		})
	}

	runConcurrent(jobs, maxPostTranscodeConcurrency)
}

// sidecarLang extracts the language tag from a sidecar filename like "Movie.en.srt".
// Falls back to "und" (undetermined) if no language segment is found.
func sidecarLang(base, stem string) string {
	if strings.EqualFold(base, stem) {
		return "und"
	}
	suffix := base[len(stem):]
	if strings.HasPrefix(suffix, ".") {
		suffix = suffix[1:]
	}
	// Take only the first segment (e.g. "en" from "en.sdh")
	if idx := strings.Index(suffix, "."); idx != -1 {
		suffix = suffix[:idx]
	}
	if suffix == "" {
		return "und"
	}
	return strings.ToLower(suffix)
}

// languageLabel returns a human-readable label for common BCP-47 language codes.
func languageLabel(lang string) string {
	labels := map[string]string{
		"en": "English", "fr": "French", "de": "German", "es": "Spanish",
		"it": "Italian", "pt": "Portuguese", "ru": "Russian", "ja": "Japanese",
		"zh": "Chinese", "ko": "Korean", "ar": "Arabic", "nl": "Dutch",
		"pl": "Polish", "sv": "Swedish", "no": "Norwegian", "da": "Danish",
		"fi": "Finnish", "cs": "Czech", "tr": "Turkish", "hu": "Hungarian",
		"und": "Unknown",
	}
	if l, ok := labels[strings.ToLower(lang)]; ok {
		return l
	}
	return lang
}

// findOrCreateShow resolves the TV show record for a season event. Matching
// is title + year (case-insensitive title, exact year) so two shows
// sharing a name but produced in different years (e.g. Doctor Who 1963
// vs 2005, Battlestar Galactica 1978 vs 2004) stay as separate records.
// A title-only fallback is preserved when one side has no year, so a
// yearless folder or an unenriched record can still associate.
func (p *Processor) findOrCreateShow(libraryID, dirName string) (*apiclient.TVShow, error) {
	title, year := parseDirName(dirName)
	shows, err := p.api.ListTVShows(libraryID)
	if err != nil {
		return nil, err
	}
	var fallback *apiclient.TVShow
	for i := range shows {
		if !strings.EqualFold(strings.TrimSpace(shows[i].Title), strings.TrimSpace(title)) {
			continue
		}
		if year > 0 && shows[i].Year == year {
			return &shows[i], nil
		}
		if (year == 0 || shows[i].Year == 0) && fallback == nil {
			fallback = &shows[i]
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	log.Printf("INFO creating TV show %q year=%d", title, year)
	return p.api.CreateTVShow(apiclient.TVShowRequest{
		LibraryID: libraryID,
		Title:     title,
		Year:      year,
	})
}

func (p *Processor) findOrCreateSeason(showID string, number int) (*apiclient.Season, error) {
	seasons, err := p.api.ListSeasons(showID)
	if err != nil {
		return nil, err
	}
	for i := range seasons {
		if seasons[i].Number == number {
			return &seasons[i], nil
		}
	}
	log.Printf("INFO creating season %d for show %s", number, showID)
	return p.api.CreateSeason(showID, apiclient.SeasonRequest{Number: number})
}

// --- helpers ---

func isVideoFile(path string) bool {
	return videoExts[strings.ToLower(filepath.Ext(path))]
}

func largestVideoFile(files []string) string {
	var best string
	var bestSize int64
	for _, f := range files {
		if !isVideoFile(f) {
			continue
		}
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		if fi.Size() > bestSize {
			bestSize = fi.Size()
			best = f
		}
	}
	return best
}

var yearSuffix = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)\s*$`)

func parseDirName(name string) (title string, year int) {
	if m := yearSuffix.FindStringSubmatch(name); m != nil {
		y, _ := strconv.Atoi(m[2])
		return strings.TrimSpace(m[1]), y
	}
	return name, 0
}

var seasonNumRe = regexp.MustCompile(`(?i)(?:season\s*|s)(\d+)`)

func parseSeasonNumber(name string) int {
	if m := seasonNumRe.FindStringSubmatch(name); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 1
}

// episodePatterns are tried in order; the first match wins.
var episodePatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)[Ss]\d+[Ee](\d+)`),   // S01E03
	regexp.MustCompile(`(?i)\d+[xX](\d+)`),        // 1x03
	regexp.MustCompile(`(?i)episode\s*(\d+)`),      // Episode 03
	regexp.MustCompile(`(?:^|\D)0*(\d{1,3})(?:\D|$)`), // leading/isolated number
}

func parseEpisodeNumber(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	for _, re := range episodePatterns {
		if m := re.FindStringSubmatch(base); m != nil {
			n, _ := strconv.Atoi(m[1])
			if n > 0 {
				return n
			}
		}
	}
	return 0
}

