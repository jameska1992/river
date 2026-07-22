package scanner

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"time"

	"github.com/google/uuid"

	"river-scan/internal/apiclient"
	"river-scan/internal/publisher"
)

// RequeueCounts is returned from RequeueUntranscoded so the caller can
// report what got published. Movies and Episodes are unit counts; Seasons
// is the number of season-level events emitted (one event groups all the
// untranscoded episodes in that season, matching how the periodic scan
// emits TV events).
type RequeueCounts struct {
	Movies          int `json:"movies"`
	Seasons         int `json:"seasons"`
	Episodes        int `json:"episodes"`
	SkippedLibraries int `json:"skipped_libraries"`
}

// RequeueUntranscoded finds media rows the transcoders never finished and
// republishes media_discovered events for each. Used as the remediation
// path when RabbitMQ messages have been drained without being processed.
//
// What counts as "untranscoded": source_path is set (the scanner saw the
// file) but file_path is empty (the transcoder never wrote an output).
//
// Idempotency: the transcoders already skip records whose FilePath is set
// — see processor.processMovie / processTVShow. Re-running this when the
// queue is empty is a no-op.
//
// Scope: movies and TV episodes only. Music tracks and audiobook chapters
// are created by the transcoder *after* transcoding, so a missed event
// leaves no row in the DB — there is nothing for this function to find.
// For those library types the remediation is to wipe the scanner state
// file (volume `scan_state`, `/state/scanner-state.json`) and let the
// next scan re-emit everything.
func (s *Scanner) RequeueUntranscoded(ctx context.Context) (*RequeueCounts, error) {
	if s.pub == nil {
		return nil, fmt.Errorf("requeue requires transcoding mode (publisher unavailable)")
	}
	if err := s.api.Login(); err != nil {
		return nil, fmt.Errorf("authenticate: %w", err)
	}

	libs, err := s.api.Libraries()
	if err != nil {
		return nil, fmt.Errorf("fetch libraries: %w", err)
	}

	counts := &RequeueCounts{}
	for _, lib := range libs {
		switch lib.Type {
		case "movie":
			if err := s.requeueMovies(ctx, lib, counts); err != nil {
				log.Printf("ERROR requeue movies in library %s: %v", lib.ID, err)
			}
		case "tvshow":
			if err := s.requeueEpisodes(ctx, lib, counts); err != nil {
				log.Printf("ERROR requeue episodes in library %s: %v", lib.ID, err)
			}
		default:
			log.Printf("INFO requeue: skipping library %s type=%s (music/audiobook need a state-wipe rescan, not a row-driven requeue)",
				lib.ID, lib.Type)
			counts.SkippedLibraries++
		}
	}
	msg := fmt.Sprintf("requeue done: movies=%d seasons=%d episodes=%d skipped_libs=%d",
		counts.Movies, counts.Seasons, counts.Episodes, counts.SkippedLibraries)
	log.Println("INFO " + msg)
	s.api.Log("info", msg)
	return counts, nil
}

func (s *Scanner) requeueMovies(ctx context.Context, lib apiclient.Library, counts *RequeueCounts) error {
	movies, err := s.api.ListMovies(lib.ID)
	if err != nil {
		return fmt.Errorf("list movies: %w", err)
	}
	for _, m := range movies {
		if m.SourcePath == "" || m.FilePath != "" {
			continue
		}
		event := publisher.MediaDiscoveredEvent{
			EventID:       uuid.New().String(),
			LibraryID:     lib.ID,
			LibraryType:   "movie",
			DirectoryName: filepath.Base(filepath.Dir(m.SourcePath)),
			DirectoryPath: filepath.Dir(m.SourcePath),
			MediaID:       m.ID,
			Files:         []string{m.SourcePath},
			DiscoveredAt:  time.Now().UTC(),
		}
		if err := s.pub.Publish(ctx, event); err != nil {
			log.Printf("ERROR publish movie %s: %v", m.ID, err)
			continue
		}
		counts.Movies++
	}
	return nil
}

func (s *Scanner) requeueEpisodes(ctx context.Context, lib apiclient.Library, counts *RequeueCounts) error {
	shows, err := s.api.ListTVShows(lib.ID)
	if err != nil {
		return fmt.Errorf("list tvshows: %w", err)
	}
	for _, show := range shows {
		seasons, err := s.api.ListSeasons(show.ID)
		if err != nil {
			log.Printf("ERROR list seasons for show %s: %v", show.ID, err)
			continue
		}
		for _, season := range seasons {
			eps, err := s.api.ListEpisodes(show.ID, season.ID)
			if err != nil {
				log.Printf("ERROR list episodes for show %s season %s: %v", show.ID, season.ID, err)
				continue
			}
			var files []string
			for _, ep := range eps {
				if ep.SourcePath == "" || ep.FilePath != "" {
					continue
				}
				files = append(files, ep.SourcePath)
			}
			if len(files) == 0 {
				continue
			}

			// Reconstruct the dir paths the periodic scan would have set.
			// The transcoder takes MediaID + SeasonID when present, but
			// processTVShow always calls parseSeasonNumber(event.SeasonName),
			// so SeasonName must parse back to the right number. "Season N"
			// is what the scanner emits and is what parseSeasonNumber expects.
			seasonPath := filepath.Dir(files[0])
			showPath := show.FolderPath
			if showPath == "" {
				showPath = filepath.Dir(seasonPath)
			}

			event := publisher.MediaDiscoveredEvent{
				EventID:       uuid.New().String(),
				LibraryID:     lib.ID,
				LibraryType:   "tvshow",
				DirectoryName: filepath.Base(showPath),
				DirectoryPath: showPath,
				SeasonName:    fmt.Sprintf("Season %d", season.Number),
				SeasonPath:    seasonPath,
				MediaID:       show.ID,
				SeasonID:      season.ID,
				Files:         files,
				DiscoveredAt:  time.Now().UTC(),
			}
			if err := s.pub.Publish(ctx, event); err != nil {
				log.Printf("ERROR publish season %s (show %s): %v", season.ID, show.ID, err)
				continue
			}
			counts.Seasons++
			counts.Episodes += len(files)
		}
	}
	return nil
}
