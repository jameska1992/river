package services

import (
	"fmt"
	"sort"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"
)

const completedThreshold = 0.9

type ProgressService struct {
	repo           repository.ProgressRepository
	movies         repository.MovieRepository
	episodes       repository.EpisodeRepository
	seasons        repository.SeasonRepository
	shows          repository.TVShowRepository
	users          repository.UserRepository
	audiobooks     repository.AudiobookRepository
	chapters       repository.ChapterRepository
	dismissedNext  repository.DismissedNextUpRepository
}

func NewProgressService(
	repo repository.ProgressRepository,
	movies repository.MovieRepository,
	episodes repository.EpisodeRepository,
	seasons repository.SeasonRepository,
	shows repository.TVShowRepository,
	users repository.UserRepository,
	audiobooks repository.AudiobookRepository,
	chapters repository.ChapterRepository,
	dismissedNext repository.DismissedNextUpRepository,
) *ProgressService {
	return &ProgressService{
		repo: repo, movies: movies, episodes: episodes, seasons: seasons,
		shows: shows, users: users, audiobooks: audiobooks, chapters: chapters,
		dismissedNext: dismissedNext,
	}
}

type ProgressInput struct {
	UserID    string
	MediaType string
	MediaID   string
	Position  float64
	Duration  float64
}

type ContinueWatchingItem struct {
	models.WatchProgress
	Title string `json:"title"`
	// PosterPath is the portrait artwork (2:3). BackdropPath is the wide
	// landscape "cover" (16:9). The home-screen rail prefers BackdropPath
	// for movies/episodes since the cards are landscape; PosterPath is the
	// fallback when no backdrop exists (e.g. audiobooks have only a
	// square cover, surfaced via PosterPath).
	PosterPath    string `json:"poster_path"`
	BackdropPath  string `json:"backdrop_path,omitempty"`
	ShowTitle     string `json:"show_title,omitempty"`
	ShowID        string `json:"show_id,omitempty"`
	SeasonID      string `json:"season_id,omitempty"`
	SeasonNumber  int    `json:"season_number,omitempty"`
	EpisodeNumber int    `json:"episode_number,omitempty"`
	// Audiobook-only fields. Title here holds the audiobook title (parent),
	// matching the show/episode pattern where the top-level title is the
	// parent and chapter/episode info lives in the dedicated fields below.
	AudiobookID   string `json:"audiobook_id,omitempty"`
	ChapterNumber int    `json:"chapter_number,omitempty"`
	ChapterTitle  string `json:"chapter_title,omitempty"`
}

func (s *ProgressService) Report(input ProgressInput) (*models.WatchProgress, error) {
	completed := input.Duration > 0 && input.Position/input.Duration >= completedThreshold
	p := &models.WatchProgress{
		UserID:    input.UserID,
		MediaType: input.MediaType,
		MediaID:   input.MediaID,
		Position:  input.Position,
		Duration:  input.Duration,
		Completed: completed,
	}
	return p, s.repo.Upsert(p)
}

func (s *ProgressService) Get(userID, mediaType, mediaID string) (*models.WatchProgress, error) {
	return s.repo.Find(userID, mediaType, mediaID)
}

func (s *ProgressService) ContinueWatching(userID string) ([]ContinueWatchingItem, error) {
	records, err := s.repo.FindInProgress(userID, 20)
	if err != nil {
		return nil, err
	}
	items := make([]ContinueWatchingItem, 0, len(records))
	// seenAudiobook tracks which audiobook IDs we've already emitted a row
	// for in this response, so a single book in progress doesn't dominate
	// the carousel by surfacing every chapter the user touched. Records
	// arrive ordered by recency, so the first match per book is kept.
	seenAudiobook := make(map[string]bool)
	for _, p := range records {
		switch p.MediaType {
		case "movie":
			m, err := s.movies.FindByID(p.MediaID)
			if err != nil {
				continue // stale reference
			}
			items = append(items, ContinueWatchingItem{
				WatchProgress: p,
				Title:         m.Title,
				PosterPath:    m.PosterPath,
				BackdropPath:  m.BackdropPath,
			})
		case "episode":
			ep, err := s.episodes.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			season, err := s.seasons.FindByID(ep.SeasonID.String())
			if err != nil {
				continue
			}
			show, err := s.shows.FindByID(ep.TVShowID.String())
			if err != nil {
				continue
			}
			title := ep.Title
			if title == "" {
				title = fmt.Sprintf("Episode %d", ep.Number)
			}
			items = append(items, ContinueWatchingItem{
				WatchProgress: p,
				Title:         title,
				PosterPath:    show.PosterPath,
				BackdropPath:  show.BackdropPath,
				ShowTitle:     show.Title,
				ShowID:        show.ID.String(),
				SeasonID:      season.ID.String(),
				SeasonNumber:  season.Number,
				EpisodeNumber: ep.Number,
			})
		case "chapter":
			ch, err := s.chapters.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			bookID := ch.AudiobookID.String()
			if seenAudiobook[bookID] {
				continue
			}
			seenAudiobook[bookID] = true
			book, err := s.audiobooks.FindByID(bookID)
			if err != nil {
				continue
			}
			items = append(items, ContinueWatchingItem{
				WatchProgress: p,
				// Top-level Title = audiobook title so the card shows the
				// book at a glance; chapter info lives in the dedicated
				// chapter fields below. Mirrors the show/episode shape.
				Title:         book.Title,
				PosterPath:    book.CoverPath,
				AudiobookID:   bookID,
				ChapterNumber: ch.Number,
				ChapterTitle:  ch.Title,
			})
		}
	}
	return items, nil
}

func (s *ProgressService) GetAllByType(userID, mediaType string) ([]models.WatchProgress, error) {
	return s.repo.FindAllByType(userID, mediaType)
}

func (s *ProgressService) Delete(userID, mediaType, mediaID string) error {
	return s.repo.Delete(userID, mediaType, mediaID)
}

// ShowWatchState is the per-user, per-show summary used by the library
// UI to render a single watched indicator for a whole show. A show is
// considered "watched" when Completed == Total && Total > 0.
type ShowWatchState struct {
	ShowID    string `json:"show_id"`
	Total     int    `json:"total"`
	Completed int    `json:"completed"`
}

// ShowWatchStates returns total + completed-by-user episode counts for
// every show in libraryID (or all libraries when libraryID is empty).
// One bulk call so a library page with N shows doesn't fan out into N
// per-show progress lookups from the client.
func (s *ProgressService) ShowWatchStates(userID, libraryID string) ([]ShowWatchState, error) {
	// The shows list is intentionally unbounded here — a library page
	// already loads its shows in pages, but the watched-states map needs
	// every show at once so the user can see the indicator without
	// pagination tied to it. A high cap keeps a runaway library from
	// blowing memory.
	shows, err := s.shows.FindAllUnpaged(libraryID)
	if err != nil {
		return nil, err
	}
	progress, err := s.repo.FindAllByType(userID, "episode")
	if err != nil {
		return nil, err
	}
	completedEpisodes := make(map[string]bool, len(progress))
	for _, p := range progress {
		if p.Completed {
			completedEpisodes[p.MediaID] = true
		}
	}
	out := make([]ShowWatchState, 0, len(shows))
	for _, show := range shows {
		episodes, err := s.episodes.FindByShowID(show.ID.String())
		if err != nil || len(episodes) == 0 {
			continue
		}
		completed := 0
		for _, ep := range episodes {
			if completedEpisodes[ep.ID.String()] {
				completed++
			}
		}
		out = append(out, ShowWatchState{
			ShowID:    show.ID.String(),
			Total:     len(episodes),
			Completed: completed,
		})
	}
	return out, nil
}

// GetShowWatchState returns the watched-state summary for a single show.
// Used by the detail page so its toggle button can render the right
// "Mark watched" / "Mark unwatched" label without computing from
// individual episode rows on the client.
func (s *ProgressService) GetShowWatchState(userID, showID string) (*ShowWatchState, error) {
	episodes, err := s.episodes.FindByShowID(showID)
	if err != nil {
		return nil, err
	}
	state := &ShowWatchState{ShowID: showID, Total: len(episodes)}
	for _, ep := range episodes {
		prog, err := s.repo.Find(userID, "episode", ep.ID.String())
		if err == nil && prog.Completed {
			state.Completed++
		}
	}
	return state, nil
}

// SetShowCompleted cascades a watched/unwatched toggle to every episode
// of a show. Marking watched flips the Completed flag on each episode's
// progress row (creating it if absent); marking unwatched deletes each
// row, returning the episodes to a clean "never seen" state. Errors on
// individual episodes are logged and skipped rather than aborting — a
// partial cascade is preferable to leaving the show in an inconsistent
// half-watched mix.
func (s *ProgressService) SetShowCompleted(userID, showID string, completed bool) error {
	episodes, err := s.episodes.FindByShowID(showID)
	if err != nil {
		return err
	}
	for _, ep := range episodes {
		if err := s.SetCompleted(userID, "episode", ep.ID.String(), completed); err != nil {
			// Best-effort cascade. Continue so a single broken row
			// doesn't block the rest of the show.
			continue
		}
	}
	return nil
}

// SetCompleted is an explicit toggle for the watched/completed flag that
// bypasses Report's position/duration → completed inference. Marking
// uncompleted deletes the progress row so the item returns to a clean
// "never seen" state (no progress bar, no badge). Marking completed
// preserves any existing position/duration and just flips the flag, so a
// row partway through being watched and then "mark watched" still records
// the real position the user reached.
func (s *ProgressService) SetCompleted(userID, mediaType, mediaID string, completed bool) error {
	if !completed {
		return s.repo.Delete(userID, mediaType, mediaID)
	}
	p := &models.WatchProgress{
		UserID:    userID,
		MediaType: mediaType,
		MediaID:   mediaID,
		Completed: true,
	}
	if existing, err := s.repo.Find(userID, mediaType, mediaID); err == nil {
		p.Position = existing.Position
		p.Duration = existing.Duration
	}
	return s.repo.Upsert(p)
}

type NextEpisodeResult struct {
	SeasonID  string `json:"season_id"`
	EpisodeID string `json:"episode_id"`
}

type ActiveSessionItem struct {
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	MediaType string  `json:"media_type"`
	Title     string  `json:"title"`
	// ShowTitle doubles as "parent title" — used for the TV show name on
	// episode sessions and the audiobook title on chapter sessions, so
	// the admin UI can render "<Title> (<ShowTitle>)" uniformly.
	ShowTitle string  `json:"show_title,omitempty"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	UpdatedAt string  `json:"updated_at"`
}

func (s *ProgressService) ActiveSessions() ([]ActiveSessionItem, error) {
	since := time.Now().Add(-5 * time.Minute)
	records, err := s.repo.FindAllActive(since, 50)
	if err != nil {
		return nil, err
	}
	items := make([]ActiveSessionItem, 0, len(records))
	for _, p := range records {
		user, err := s.users.FindByID(p.UserID)
		if err != nil {
			continue
		}
		item := ActiveSessionItem{
			UserID:    p.UserID,
			Username:  user.Username,
			MediaType: p.MediaType,
			Position:  p.Position,
			Duration:  p.Duration,
			UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
		}
		switch p.MediaType {
		case "movie":
			m, err := s.movies.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			item.Title = m.Title
		case "episode":
			ep, err := s.episodes.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			show, err := s.shows.FindByID(ep.TVShowID.String())
			if err != nil {
				continue
			}
			if ep.Title != "" {
				item.Title = ep.Title
			} else {
				item.Title = fmt.Sprintf("Episode %d", ep.Number)
			}
			item.ShowTitle = show.Title
		case "chapter":
			ch, err := s.chapters.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			book, err := s.audiobooks.FindByID(ch.AudiobookID.String())
			if err != nil {
				continue
			}
			if ch.Title != "" {
				item.Title = ch.Title
			} else {
				item.Title = fmt.Sprintf("Chapter %d", ch.Number)
			}
			item.ShowTitle = book.Title
		default:
			continue
		}
		items = append(items, item)
	}
	return items, nil
}

type ActivityItem struct {
	MediaType string  `json:"media_type"`
	MediaID   string  `json:"media_id"`
	Title     string  `json:"title"`
	ShowTitle string  `json:"show_title,omitempty"`
	ShowID    string  `json:"show_id,omitempty"`
	Position  float64 `json:"position"`
	Duration  float64 `json:"duration"`
	Completed bool    `json:"completed"`
	UpdatedAt string  `json:"updated_at"`
}

func (s *ProgressService) GetUserActivity(userID string) ([]ActivityItem, error) {
	records, err := s.repo.FindByUser(userID, 50)
	if err != nil {
		return nil, err
	}
	items := make([]ActivityItem, 0, len(records))
	for _, p := range records {
		item := ActivityItem{
			MediaType: p.MediaType,
			MediaID:   p.MediaID,
			Position:  p.Position,
			Duration:  p.Duration,
			Completed: p.Completed,
			UpdatedAt: p.UpdatedAt.Format(time.RFC3339),
		}
		switch p.MediaType {
		case "movie":
			m, err := s.movies.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			item.Title = m.Title
		case "episode":
			ep, err := s.episodes.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			show, err := s.shows.FindByID(ep.TVShowID.String())
			if err != nil {
				continue
			}
			if ep.Title != "" {
				item.Title = ep.Title
			} else {
				item.Title = fmt.Sprintf("Episode %d", ep.Number)
			}
			item.ShowTitle = show.Title
			item.ShowID = show.ID.String()
		case "chapter":
			ch, err := s.chapters.FindByID(p.MediaID)
			if err != nil {
				continue
			}
			book, err := s.audiobooks.FindByID(ch.AudiobookID.String())
			if err != nil {
				continue
			}
			if ch.Title != "" {
				item.Title = ch.Title
			} else {
				item.Title = fmt.Sprintf("Chapter %d", ch.Number)
			}
			item.ShowTitle = book.Title
			item.ShowID = book.ID.String()
		default:
			item.Title = p.MediaType
		}
		items = append(items, item)
	}
	return items, nil
}

// NextUpItem is one row in the home-page "Next Up" rail. Shape mirrors
// ContinueWatchingItem so the client can render both with the same card
// component — the meaningful difference is that Next Up refers to an
// episode the user hasn't started (Position/Duration are always 0).
type NextUpItem struct {
	MediaType     string    `json:"media_type"` // always "episode" in v1
	MediaID       string    `json:"media_id"`   // the next episode's ID
	Title         string    `json:"title"`      // "Sxx Eyy" or the episode title
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path,omitempty"`
	ShowTitle     string    `json:"show_title"`
	ShowID        string    `json:"show_id"`
	SeasonID      string    `json:"season_id"`
	SeasonNumber  int       `json:"season_number"`
	EpisodeNumber int       `json:"episode_number"`
	// UpdatedAt is the timestamp of the *last-completed* episode of the
	// show — that's the anchor for recency ordering across the row.
	UpdatedAt time.Time `json:"updated_at"`
}

// NextUp returns the "next episode to start" for every show the user
// has recently completed an episode of, capped at limit and ordered by
// how recently the anchoring episode was completed.
//
// Semantics:
//   - Anchor = the most-recently-completed episode of each show.
//   - Next   = the next episode in canonical order (same season, higher
//              number; else earliest episode of the next non-special
//              season with any episodes). Season 0 / specials are
//              excluded from the ordering.
//   - Skip   = next is itself completed, has any watch_progress row
//              (would live in Continue Watching), or is dismissed.
//
// If a show has no eligible "next" (e.g. finale watched, or the next
// episode is dismissed) it drops out of the row entirely — the user
// will see it reappear once they resume the show and complete another
// episode past the current tail.
func (s *ProgressService) NextUp(userID string, limit int) ([]NextUpItem, error) {
	if limit <= 0 {
		limit = 16
	}
	completed, err := s.repo.FindCompletedEpisodes(userID)
	if err != nil {
		return nil, err
	}
	// Fastest set of "don't show me these" checks: pre-load the user's
	// dismissals and any in-progress episode IDs so the per-show loop
	// below is O(1) per lookup.
	dismissedList, err := s.dismissedNext.ListEpisodeIDs(userID)
	if err != nil {
		return nil, err
	}
	dismissed := make(map[string]bool, len(dismissedList))
	for _, id := range dismissedList {
		dismissed[id] = true
	}
	inProgress, err := s.repo.FindAllByType(userID, "episode")
	if err != nil {
		return nil, err
	}
	// Any progress row (completed or not) means the episode belongs on
	// Continue Watching, not Next Up — Next Up is specifically "haven't
	// touched this one yet." The completed set is intentionally in
	// there too: if the user finished the next episode already, it
	// shouldn't reappear as "next."
	seen := make(map[string]bool, len(inProgress))
	for _, p := range inProgress {
		seen[p.MediaID] = true
	}

	// Group completed rows by show, taking the most recent per show as
	// the anchor. FindCompletedEpisodes returns rows ordered by
	// updated_at DESC, so the first sighting per show wins.
	type anchor struct {
		ep        models.Episode
		completed time.Time
		showID    string
	}
	anchorByShow := map[string]anchor{}
	orderedShows := make([]string, 0)
	for _, p := range completed {
		ep, err := s.episodes.FindByID(p.MediaID)
		if err != nil {
			continue // stale reference
		}
		showID := ep.TVShowID.String()
		if _, ok := anchorByShow[showID]; ok {
			continue
		}
		anchorByShow[showID] = anchor{ep: *ep, completed: p.UpdatedAt, showID: showID}
		orderedShows = append(orderedShows, showID)
	}

	items := make([]NextUpItem, 0, len(orderedShows))
	for _, showID := range orderedShows {
		if len(items) >= limit {
			break
		}
		a := anchorByShow[showID]
		next, seasonNum, ok := s.resolveNextEpisode(a.ep)
		if !ok {
			continue
		}
		nextID := next.ID.String()
		if seen[nextID] || dismissed[nextID] {
			continue
		}
		show, err := s.shows.FindByID(showID)
		if err != nil {
			continue
		}
		title := next.Title
		if title == "" {
			title = fmt.Sprintf("Episode %d", next.Number)
		}
		items = append(items, NextUpItem{
			MediaType:     "episode",
			MediaID:       nextID,
			Title:         title,
			PosterPath:    show.PosterPath,
			BackdropPath:  show.BackdropPath,
			ShowTitle:     show.Title,
			ShowID:        show.ID.String(),
			SeasonID:      next.SeasonID.String(),
			SeasonNumber:  seasonNum,
			EpisodeNumber: next.Number,
			UpdatedAt:     a.completed,
		})
	}
	return items, nil
}

// resolveNextEpisode finds the episode that comes after the given one in
// the show's canonical order:
//
//  1. Same season, next-highest non-special episode number.
//  2. Else earliest non-special episode of the next non-special season
//     that has any playable episodes.
//
// Specials (Number-in-season-0 or IsSpecial=true) are excluded from
// ordering entirely — they're not linearly playable in TMDB's sense.
// Returns the next episode + its season number + true on success.
func (s *ProgressService) resolveNextEpisode(current models.Episode) (models.Episode, int, bool) {
	seasons, err := s.seasons.FindByShowID(current.TVShowID.String())
	if err != nil {
		return models.Episode{}, 0, false
	}
	// Filter and sort non-special seasons by number.
	nonSpecial := make([]models.Season, 0, len(seasons))
	for _, sn := range seasons {
		if sn.Number > 0 {
			nonSpecial = append(nonSpecial, sn)
		}
	}
	sort.Slice(nonSpecial, func(i, j int) bool { return nonSpecial[i].Number < nonSpecial[j].Number })

	// Locate the current season's index within the ordered list.
	currentIdx := -1
	for i, sn := range nonSpecial {
		if sn.ID == current.SeasonID {
			currentIdx = i
			break
		}
	}
	if currentIdx == -1 {
		// Current episode is a special (or in a season we don't know
		// about). Nothing sensible to recommend as "next" from a special.
		return models.Episode{}, 0, false
	}

	// Same-season lookup: next-higher episode number, IsSpecial excluded.
	sameSeason := nonSpecial[currentIdx]
	eps, err := s.episodes.FindBySeasonID(sameSeason.ID.String())
	if err == nil {
		next, ok := pickNextInSeason(eps, current.Number)
		if ok {
			return next, sameSeason.Number, true
		}
	}

	// Fall through: earliest non-special episode of the next season that
	// has any playable content.
	for _, sn := range nonSpecial[currentIdx+1:] {
		eps, err := s.episodes.FindBySeasonID(sn.ID.String())
		if err != nil {
			continue
		}
		first, ok := pickFirstInSeason(eps)
		if ok {
			return first, sn.Number, true
		}
	}
	return models.Episode{}, 0, false
}

// pickNextInSeason returns the lowest-numbered non-special episode with
// Number > current within eps, sorted ascending.
func pickNextInSeason(eps []models.Episode, current int) (models.Episode, bool) {
	best := models.Episode{}
	found := false
	for _, ep := range eps {
		if ep.IsSpecial || ep.Number <= current {
			continue
		}
		if !found || ep.Number < best.Number {
			best = ep
			found = true
		}
	}
	return best, found
}

// pickFirstInSeason returns the lowest-numbered non-special episode
// within eps. Used when we roll over from the end of one season into
// the next.
func pickFirstInSeason(eps []models.Episode) (models.Episode, bool) {
	best := models.Episode{}
	found := false
	for _, ep := range eps {
		if ep.IsSpecial {
			continue
		}
		if !found || ep.Number < best.Number {
			best = ep
			found = true
		}
	}
	return best, found
}

// DismissNextUp records the user's decision to hide a specific episode
// from their Next Up rail. Idempotent.
func (s *ProgressService) DismissNextUp(userID, episodeID string) error {
	return s.dismissedNext.Create(userID, episodeID)
}

// UndismissNextUp reverses a dismissal. Returns ErrNotFound if the
// episode wasn't dismissed to begin with — lets the client distinguish
// "undo done" from "nothing to undo."
func (s *ProgressService) UndismissNextUp(userID, episodeID string) error {
	return s.dismissedNext.Delete(userID, episodeID)
}

// NextEpisode returns the first unwatched/incomplete episode for a show, or the
// first episode if everything has been completed.
func (s *ProgressService) NextEpisode(userID, showID string) (*NextEpisodeResult, error) {
	seasons, err := s.seasons.FindByShowID(showID)
	if err != nil {
		return nil, err
	}
	sort.Slice(seasons, func(i, j int) bool { return seasons[i].Number < seasons[j].Number })

	var first *NextEpisodeResult
	for _, season := range seasons {
		eps, err := s.episodes.FindBySeasonID(season.ID.String())
		if err != nil {
			continue
		}
		sort.Slice(eps, func(i, j int) bool { return eps[i].Number < eps[j].Number })
		for _, ep := range eps {
			if ep.FilePath == "" {
				continue
			}
			result := &NextEpisodeResult{
				SeasonID:  season.ID.String(),
				EpisodeID: ep.ID.String(),
			}
			if first == nil {
				first = result
			}
			prog, err := s.repo.Find(userID, "episode", ep.ID.String())
			if err != nil || !prog.Completed {
				return result, nil
			}
		}
	}
	if first == nil {
		return nil, apperrors.ErrNotFound
	}
	// All episodes completed — restart from the first one
	return first, nil
}
