package processor

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"river-audio-trans/internal/apiclient"
	"river-audio-trans/internal/consumer"
	"river-audio-trans/internal/transcoder"
)

type Processor struct {
	api         *apiclient.Client
	outputDir   string
	concurrency int // max parallel transcode operations within one event
}

func New(api *apiclient.Client, outputDir string, concurrency int) *Processor {
	if concurrency < 1 {
		concurrency = 1
	}
	return &Processor{api: api, outputDir: outputDir, concurrency: concurrency}
}

func (p *Processor) Handle(event consumer.MediaDiscoveredEvent) error {
	log.Printf("INFO processing event %s type=%s dir=%q files=%d",
		event.EventID, event.LibraryType, event.DirectoryName, len(event.Files))
	// Pre-transcoded libraries: scanner already registered records +
	// sidecars; nothing to encode. See river-video-trans processor for
	// the shape of this contract.
	if event.PreTranscoded {
		log.Printf("INFO skipping pre-transcoded event %s (%s / %q)",
			event.EventID, event.LibraryType, event.DirectoryName)
		return nil
	}
	switch event.LibraryType {
	case "music":
		return p.processMusic(event)
	case "audiobook":
		return p.processAudiobook(event)
	default:
		log.Printf("INFO skipping unsupported library type %q", event.LibraryType)
		return nil
	}
}

// processMusic treats the top-level directory as the artist, and each
// immediate subdirectory as an album. Files directly in the top-level
// directory are grouped under an album named after the artist.
func (p *Processor) processMusic(event consumer.MediaDiscoveredEvent) error {
	var artist *apiclient.Artist
	if event.MediaID != "" {
		artist = &apiclient.Artist{ID: event.MediaID, LibraryID: event.LibraryID, Name: event.DirectoryName}
	} else {
		var err error
		artist, err = p.findOrCreateArtist(event.LibraryID, event.DirectoryName)
		if err != nil {
			return fmt.Errorf("find/create artist: %w", err)
		}
	}

	for albumName, files := range groupByAlbum(event.DirectoryPath, event.Files) {
		albumTitle, albumYear := parseDirName(albumName)
		album, err := p.findOrCreateAlbum(event.LibraryID, artist.ID, albumTitle, albumYear)
		if err != nil {
			log.Printf("ERROR find/create album %q: %v", albumTitle, err)
			continue
		}

		existing, err := p.api.ListAlbumTracks(album.ID)
		if err != nil {
			log.Printf("ERROR list tracks for album %s: %v", album.ID, err)
			continue
		}
		registered := make(map[int]bool, len(existing))
		for _, t := range existing {
			registered[t.Number] = true
		}

		type trackJob struct {
			file string
			num  int
		}
		var trackJobs []trackJob
		for _, file := range files {
			num := parseTrackNumber(file)
			if num > 0 && registered[num] {
				log.Printf("INFO track %d already registered, skipping %q", num, filepath.Base(file))
				continue
			}
			trackJobs = append(trackJobs, trackJob{file, num})
		}

		albumID := album.ID
		sem := make(chan struct{}, p.concurrency)
		var wg sync.WaitGroup
		for _, job := range trackJobs {
			job := job
			wg.Add(1)
			sem <- struct{}{}
			go func() {
				defer wg.Done()
				defer func() { <-sem }()
				finalPath, duration, err := p.processFile(job.file, event.LibraryType, event.LibraryPath)
				if err != nil {
					log.Printf("ERROR processing %q: %v", job.file, err)
					return
				}
				title := parseAudioTitle(job.file)
				log.Printf("INFO creating track %q (album %s, num=%d)", title, albumID, job.num)
				if _, err := p.api.CreateTrack(apiclient.TrackRequest{
					LibraryID: event.LibraryID,
					AlbumID:   albumID,
					ArtistID:  artist.ID,
					Title:     title,
					Number:    job.num,
					Duration:  duration,
					FilePath:  finalPath,
				}); err != nil {
					log.Printf("ERROR create track %q: %v", title, err)
				}
			}()
		}
		wg.Wait()
	}
	return nil
}

// processAudiobook treats all files in the event as chapters of a single
// audiobook named after the directory.
func (p *Processor) processAudiobook(event consumer.MediaDiscoveredEvent) error {
	var book *apiclient.Audiobook
	if event.MediaID != "" {
		title, _ := parseDirName(event.DirectoryName)
		book = &apiclient.Audiobook{ID: event.MediaID, LibraryID: event.LibraryID, Title: title}
	} else {
		title, year := parseDirName(event.DirectoryName)
		var err error
		book, err = p.findOrCreateAudiobook(event.LibraryID, title, year)
		if err != nil {
			return fmt.Errorf("find/create audiobook: %w", err)
		}
	}

	existing, err := p.api.ListChapters(book.ID)
	if err != nil {
		return fmt.Errorf("list chapters: %w", err)
	}
	registered := make(map[int]bool, len(existing))
	for _, ch := range existing {
		registered[ch.Number] = true
	}

	type chapterJob struct {
		file string
		num  int
	}
	files := sortedFiles(event.Files)
	var jobs []chapterJob
	for i, file := range files {
		num := parseChapterNumber(file)
		if num == 0 {
			num = i + 1
		}
		if registered[num] {
			log.Printf("INFO chapter %d already registered, skipping %q", num, filepath.Base(file))
			continue
		}
		jobs = append(jobs, chapterJob{file, num})
	}

	sem := make(chan struct{}, p.concurrency)
	var wg sync.WaitGroup
	for _, job := range jobs {
		job := job
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			finalPath, duration, err := p.processFile(job.file, event.LibraryType, event.LibraryPath)
			if err != nil {
				log.Printf("ERROR processing %q: %v", job.file, err)
				return
			}
			chTitle := parseAudioTitle(job.file)
			log.Printf("INFO creating chapter %d %q (audiobook %s)", job.num, chTitle, book.ID)
			if _, err := p.api.CreateChapter(book.ID, apiclient.ChapterRequest{
				Number:   job.num,
				Title:    chTitle,
				Duration: duration,
				FilePath: finalPath,
			}); err != nil {
				log.Printf("ERROR create chapter %d: %v", job.num, err)
			}
		}()
	}
	wg.Wait()
	return nil
}

// processFile probes the file, transcodes if necessary, and returns the
// final path and duration in seconds.
func (p *Processor) processFile(path, libraryType, libraryPath string) (finalPath string, duration int, err error) {
	info, err := transcoder.Probe(path)
	if err != nil {
		return "", 0, fmt.Errorf("probe: %w", err)
	}
	if !transcoder.NeedsTranscode(path, info) {
		log.Printf("INFO %q is already aac/m4a, no transcode needed", filepath.Base(path))
		return path, info.Duration, nil
	}
	outPath := transcoder.OutputPath(path, libraryType, libraryPath, p.outputDir)
	if _, err := os.Stat(outPath); err == nil {
		log.Printf("INFO transcoded output already exists: %q", outPath)
		return outPath, info.Duration, nil
	}
	log.Printf("INFO transcoding %q → %q (codec=%s)", filepath.Base(path), filepath.Base(outPath), info.Codec)
	p.api.Log("info", fmt.Sprintf("transcoding audio %s", filepath.Base(outPath)))
	if err := transcoder.Transcode(path, outPath); err != nil {
		p.api.Log("error", fmt.Sprintf("transcode failed for %s: %v", filepath.Base(outPath), err))
		return "", 0, fmt.Errorf("transcode: %w", err)
	}
	log.Printf("INFO transcode complete: %q", outPath)
	p.api.Log("info", fmt.Sprintf("transcoded audio %s", filepath.Base(outPath)))
	return outPath, info.Duration, nil
}

func (p *Processor) findOrCreateArtist(libraryID, name string) (*apiclient.Artist, error) {
	artists, err := p.api.ListArtists(libraryID)
	if err != nil {
		return nil, err
	}
	for i := range artists {
		if strings.EqualFold(artists[i].Name, name) {
			return &artists[i], nil
		}
	}
	log.Printf("INFO creating artist %q", name)
	return p.api.CreateArtist(apiclient.ArtistRequest{
		LibraryID: libraryID,
		Name:      name,
	})
}

func (p *Processor) findOrCreateAlbum(libraryID, artistID, title string, year int) (*apiclient.Album, error) {
	albums, err := p.api.ListAlbums(libraryID)
	if err != nil {
		return nil, err
	}
	for i := range albums {
		if strings.EqualFold(albums[i].Title, title) && albums[i].ArtistID == artistID {
			return &albums[i], nil
		}
	}
	log.Printf("INFO creating album %q (year=%d)", title, year)
	return p.api.CreateAlbum(apiclient.AlbumRequest{
		LibraryID: libraryID,
		ArtistID:  artistID,
		Title:     title,
		Year:      year,
	})
}

func (p *Processor) findOrCreateAudiobook(libraryID, title string, year int) (*apiclient.Audiobook, error) {
	books, err := p.api.ListAudiobooks(libraryID)
	if err != nil {
		return nil, err
	}
	for i := range books {
		if strings.EqualFold(books[i].Title, title) {
			return &books[i], nil
		}
	}
	log.Printf("INFO creating audiobook %q (year=%d)", title, year)
	return p.api.CreateAudiobook(apiclient.AudiobookRequest{
		LibraryID: libraryID,
		Title:     title,
		Year:      year,
	})
}

// groupByAlbum groups files by their immediate subdirectory under dirPath.
// Files directly in dirPath are grouped under the dirPath base name.
func groupByAlbum(dirPath string, files []string) map[string][]string {
	groups := make(map[string][]string)
	for _, f := range files {
		rel, err := filepath.Rel(dirPath, filepath.Dir(f))
		if err != nil || rel == "." {
			groups[filepath.Base(dirPath)] = append(groups[filepath.Base(dirPath)], f)
			continue
		}
		// Use only the first path component so nested dirs collapse to one album.
		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		groups[parts[0]] = append(groups[parts[0]], f)
	}
	return groups
}

// sortedFiles returns a copy of files sorted by base name.
func sortedFiles(files []string) []string {
	sorted := make([]string, len(files))
	copy(sorted, files)
	sort.Slice(sorted, func(i, j int) bool {
		return filepath.Base(sorted[i]) < filepath.Base(sorted[j])
	})
	return sorted
}

var yearSuffix = regexp.MustCompile(`^(.+?)\s*\((\d{4})\)\s*$`)

// parseDirName extracts "Title (YYYY)" → (title, year).
func parseDirName(name string) (string, int) {
	if m := yearSuffix.FindStringSubmatch(name); m != nil {
		y, _ := strconv.Atoi(m[2])
		return strings.TrimSpace(m[1]), y
	}
	return name, 0
}

// trackNumRe matches common track number prefixes:
// "01 - Title", "01. Title", "01 Title", "Track 01"
var trackNumRe = regexp.MustCompile(`^(?:track\s*)?0*(\d{1,3})[\s.\-_]+`)

func parseTrackNumber(path string) int {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if m := trackNumRe.FindStringSubmatch(base); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

// chapterNumRe matches "01", "Chapter 01", "Part 01", "01 -" prefixes.
var chapterNumRe = regexp.MustCompile(`^(?:chapter\s*|part\s*)?0*(\d{1,3})[\s.\-_]`)

func parseChapterNumber(path string) int {
	base := strings.ToLower(strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)))
	if m := chapterNumRe.FindStringSubmatch(base); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0
}

var numPrefixRe = regexp.MustCompile(`^(?:(?:track|chapter|part)\s*)?0*\d+[\s.\-_]+`)
var dotSpaceRe = regexp.MustCompile(`[._]+`)
var qualityTagRe = regexp.MustCompile(`(?i)[._\s](128k|192k|256k|320k|flac|lossless|hd|remaster(ed)?|deluxe).*$`)

// parseAudioTitle strips numeric prefixes and junk tags to produce a readable title.
func parseAudioTitle(path string) string {
	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	base = numPrefixRe.ReplaceAllString(base, "")
	base = qualityTagRe.ReplaceAllString(base, "")
	base = dotSpaceRe.ReplaceAllString(base, " ")
	return strings.TrimSpace(base)
}
