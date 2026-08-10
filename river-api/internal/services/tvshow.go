package services

import (
	"errors"
	"sort"
	"time"

	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

type TVShowService struct {
	shows    repository.TVShowRepository
	seasons  repository.SeasonRepository
	episodes repository.EpisodeRepository
	cleanup  repository.MediaCleanupRepository
	// merge resolves a merged-away show id to its survivor so writes that
	// still carry a pre-merge id (e.g. a transcode finishing after an admin
	// merged the show) can be redirected. May be nil in unit tests that don't
	// exercise the redirect path.
	merge repository.TVShowMergeRepository
}

func NewTVShowService(
	shows repository.TVShowRepository,
	seasons repository.SeasonRepository,
	episodes repository.EpisodeRepository,
	cleanup repository.MediaCleanupRepository,
	merge repository.TVShowMergeRepository,
) *TVShowService {
	return &TVShowService{shows: shows, seasons: seasons, episodes: episodes, cleanup: cleanup, merge: merge}
}

// --- TV Shows ---

type TVShowFilter struct {
	LibraryID string
	Page      int
	Limit     int
	Sort      string // title | year | rating | added (default: title)
	Order     string // asc | desc (default: asc)
}

// tvShowSortColumns maps user-facing sort keys to allowed SQL expressions.
// Title uses titleSortExpr (lowercased + leading article stripped); see
// movie.go for the rationale.
var tvShowSortColumns = map[string]string{
	"title":  titleSortExpr("title"),
	"year":   "year",
	"rating": "rating",
	"added":  "created_at",
}

type TVShowInput struct {
	LibraryID     uuid.UUID
	Title         string
	OriginalTitle string
	Description   string
	Year          int
	Status        string
	Genres        string
	Rating        float32
	PosterPath    string
	BackdropPath  string
	TrailerURL    string
	// TMDBID is sticky once set — UpdateShow only overwrites it when the
	// incoming value is non-zero, so generic edits (admin metadata form,
	// scanner backfills) can't accidentally erase it. Enrichment writes a
	// real id; everyone else passes zero.
	TMDBID     int
	FolderPath string
}

func (s *TVShowService) ListShows(f TVShowFilter) ([]models.TVShow, error) {
	offset, limit := paginationOffsetLimit(f.Page, f.Limit)
	return s.shows.FindAll(f.LibraryID, offset, limit, sortClause(f.Sort, f.Order, tvShowSortColumns, titleSortExpr("title")))
}

func (s *TVShowService) CountShows(libraryID string) (int64, error) {
	return s.shows.Count(libraryID)
}

func (s *TVShowService) ListRecentShows(limit int) ([]models.TVShow, error) {
	return s.shows.FindRecent(limit)
}

// SimilarShows returns up to limit shows whose genres overlap with the
// given show's. Ranked by shared count desc → rating desc → created_at
// desc. See MovieService.Similar for design notes on the in-memory
// approach.
func (s *TVShowService) SimilarShows(id string, limit int) ([]SimilarItem, error) {
	source, err := s.shows.FindByID(id)
	if err != nil {
		return nil, err
	}
	sourceGenres := parseGenresJSON(source.Genres)
	if len(sourceGenres) == 0 {
		return []SimilarItem{}, nil
	}
	all, err := s.shows.FindAll("", 0, similarSourceLoadCap, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]similarCandidate, 0, len(all))
	for _, sh := range all {
		candidates = append(candidates, similarCandidate{
			ID:           sh.ID.String(),
			Genres:       parseGenresJSON(sh.Genres),
			Rating:       sh.Rating,
			CreatedAt:    sh.CreatedAt,
			Title:        sh.Title,
			Year:         sh.Year,
			PosterPath:   sh.PosterPath,
			BackdropPath: sh.BackdropPath,
		})
	}
	ranked := rankBySharedGenres(source.ID.String(), sourceGenres, candidates, limit)
	return candidatesToSimilarItems(ranked, "tvshow"), nil
}

func (s *TVShowService) ListUnidentifiedShows() ([]models.TVShow, error) {
	return s.shows.FindUnidentified()
}

func (s *TVShowService) CreateShow(input TVShowInput) (*models.TVShow, error) {
	show := models.TVShow{
		LibraryID:     input.LibraryID,
		Title:         input.Title,
		OriginalTitle: input.OriginalTitle,
		Description:   input.Description,
		Year:          input.Year,
		Status:        input.Status,
		Genres:        defaultJSON(input.Genres),
		Rating:        input.Rating,
		PosterPath:    input.PosterPath,
		BackdropPath:  input.BackdropPath,
		TrailerURL:    input.TrailerURL,
		TMDBID:        input.TMDBID,
		FolderPath:    input.FolderPath,
	}
	return &show, s.shows.Create(&show)
}

func (s *TVShowService) GetShow(id string) (*models.TVShow, error) {
	show, err := s.shows.FindByID(id)
	if errors.Is(err, ErrNotFound) {
		// The id may belong to a show that was merged into another. Follow the
		// redirect so a caller holding a pre-merge id — notably river-video-trans
		// finishing a transcode that was queued before the merge — resolves the
		// surviving show instead of failing outright.
		if survivorID, ok := s.followMergedShow(id); ok {
			return s.shows.FindByID(survivorID)
		}
	}
	return show, err
}

// followMergedShow walks the merge-alias chain from a possibly-merged-away show
// id to the id of the show that ultimately absorbed it. Returns ok=false when
// showID was never merged (or no merge repo is wired). The walk is bounded and
// cycle-guarded so a self- or loop-referencing alias can't spin.
func (s *TVShowService) followMergedShow(showID string) (string, bool) {
	if s.merge == nil {
		return "", false
	}
	seen := map[string]bool{showID: true}
	cur := showID
	for i := 0; i < 16; i++ {
		next, err := s.merge.FindSurvivorID(cur)
		if err != nil || seen[next] {
			break
		}
		seen[next] = true
		cur = next
	}
	if cur == showID {
		return "", false
	}
	return cur, true
}

// resolveSeasonForWrite loads the season identified by (showID, seasonID) for a
// write, transparently following a TV-show merge when the pair no longer
// resolves. This covers the race where an admin merges a show while one of its
// episodes is still transcoding: river-video-trans finishes holding the
// pre-merge show/season ids, and without this the late CreateEpisode would 404
// and the transcoded file would be orphaned. Returns the live survivor season
// so the episode still registers.
func (s *TVShowService) resolveSeasonForWrite(showID, seasonID string) (*models.Season, error) {
	season, err := s.seasons.FindByIDAndShowID(seasonID, showID)
	if !errors.Is(err, ErrNotFound) {
		return season, err
	}
	survivorID, ok := s.followMergedShow(showID)
	if !ok {
		return nil, err
	}
	// Case 1: the whole season was re-parented onto the survivor (id unchanged).
	if moved, merr := s.seasons.FindByIDAndShowID(seasonID, survivorID); merr == nil {
		return moved, nil
	}
	// Case 2: the merged season collided with a same-numbered survivor season
	// and was soft-deleted, its episodes folded in. Match the survivor season
	// by the (now soft-deleted) merged season's number.
	merged, merr := s.seasons.FindByIDIncludingDeleted(seasonID)
	if merr != nil {
		return nil, err
	}
	survivorSeasons, merr := s.seasons.FindByShowID(survivorID)
	if merr != nil {
		return nil, merr
	}
	for i := range survivorSeasons {
		if survivorSeasons[i].Number == merged.Number {
			return &survivorSeasons[i], nil
		}
	}
	return nil, err
}

func (s *TVShowService) UpdateShow(id string, input TVShowInput) (*models.TVShow, error) {
	show, err := s.shows.FindByID(id)
	if err != nil {
		return nil, err
	}
	show.Title = input.Title
	show.OriginalTitle = input.OriginalTitle
	show.Description = input.Description
	show.Year = input.Year
	show.Status = input.Status
	show.Rating = input.Rating
	show.PosterPath = input.PosterPath
	show.BackdropPath = input.BackdropPath
	show.TrailerURL = input.TrailerURL
	if input.Genres != "" {
		show.Genres = input.Genres
	}
	if input.FolderPath != "" {
		show.FolderPath = input.FolderPath
	}
	// TMDBID is intentionally only overwritten when the caller provides
	// one — generic metadata edits send 0 and must not erase the resolved
	// id, otherwise the next enrichment would re-search by title and
	// could land on a different popular candidate.
	if input.TMDBID > 0 {
		show.TMDBID = input.TMDBID
	}
	return show, s.shows.Save(show)
}

// UpdateFolderPath sets only the folder_path on the show. Used by river-scan
// to backfill the path on existing rows without clobbering meta-tv's title.
func (s *TVShowService) UpdateFolderPath(id, path string) (*models.TVShow, error) {
	show, err := s.shows.FindByID(id)
	if err != nil {
		return nil, err
	}
	show.FolderPath = path
	return show, s.shows.Save(show)
}

func (s *TVShowService) DeleteShow(id string) error {
	// Drop watchlist entries, collection memberships, show + episode
	// cast/crew + progress/sub/audio refs before the row itself so
	// cross-references never outlive the target.
	if err := s.cleanup.PurgeShow(id); err != nil {
		return err
	}
	return s.shows.Delete(id)
}

// ClearTMDBID resets the resolved TMDB id back to 0 so the next
// enrichment runs a fresh title+year search rather than re-fetching the
// previously-resolved record. Used by the admin Identify flow when the
// admin changes title or year — the stored id from the prior (now
// incorrect) enrichment would otherwise short-circuit the new search
// via the sticky-id branch in river-meta-tv's fetchShow.
func (s *TVShowService) ClearTMDBID(id string) error {
	show, err := s.shows.FindByID(id)
	if err != nil {
		return err
	}
	show.TMDBID = 0
	return s.shows.Save(show)
}

// --- Seasons ---

type SeasonInput struct {
	Number      int
	Title       string
	Description string
	Year        int
	PosterPath  string
}

func (s *TVShowService) ListSeasons(showID string) ([]models.Season, error) {
	return s.seasons.FindByShowID(showID)
}

func (s *TVShowService) CreateSeason(showID string, input SeasonInput) (*models.Season, error) {
	show, err := s.shows.FindByID(showID)
	if err != nil {
		return nil, err
	}
	season := models.Season{
		TVShowID:    show.ID,
		Number:      input.Number,
		Title:       input.Title,
		Description: input.Description,
		Year:        input.Year,
		PosterPath:  input.PosterPath,
	}
	return &season, s.seasons.Create(&season)
}

// UpdateSeason applies admin metadata edits to an existing season. Empty
// inputs are skipped (PATCH semantics) so a partial form doesn't clobber
// fields meta-tv has already populated.
func (s *TVShowService) UpdateSeason(showID, seasonID string, input SeasonInput) (*models.Season, error) {
	season, err := s.seasons.FindByIDAndShowID(seasonID, showID)
	if err != nil {
		return nil, err
	}
	if input.Number > 0 {
		season.Number = input.Number
	}
	if input.Title != "" {
		season.Title = input.Title
	}
	if input.Description != "" {
		season.Description = input.Description
	}
	if input.Year > 0 {
		season.Year = input.Year
	}
	if input.PosterPath != "" {
		season.PosterPath = input.PosterPath
	}
	return season, s.seasons.Save(season)
}

// --- Episodes ---

type EpisodeInput struct {
	Number      int
	// SeasonID lets an admin move an episode to a different season — used
	// to fix mis-detected season assignment without recreating the row.
	// Empty means "leave as-is". When non-empty the service verifies the
	// new season belongs to the same show.
	SeasonID    string
	Title       string
	Description string
	Runtime     int
	FilePath    string
	SourcePath  string
	AiredAt     string // RFC3339; empty means zero time
	// IsSpecial marks an episode whose filename didn't match a standard
	// number pattern. The CreateEpisode upsert disambiguates regular ep N
	// from special N via this flag.
	IsSpecial bool
}

func (s *TVShowService) ListEpisodes(seasonID string) ([]models.Episode, error) {
	eps, err := s.episodes.FindBySeasonID(seasonID)
	if err != nil {
		return nil, err
	}
	result := deduplicateEpisodes(eps)
	// Persist any file_path values that deduplication merged from a duplicate
	// record. Without this, GetEpisode (used by the stream handler) returns the
	// DB row which still has an empty file_path, causing 404 on stream.
	origPath := make(map[string]string, len(eps))
	for _, ep := range eps {
		origPath[ep.ID.String()] = ep.FilePath
	}
	for i := range result {
		ep := &result[i]
		if ep.FilePath != "" && origPath[ep.ID.String()] == "" {
			_ = s.episodes.Save(ep)
		}
	}
	return result, nil
}

func (s *TVShowService) CreateEpisode(showID, seasonID string, input EpisodeInput) (*models.Episode, error) {
	season, err := s.resolveSeasonForWrite(showID, seasonID)
	if err != nil {
		return nil, err
	}
	// Dedupe against the resolved season's episodes — after a merge redirect
	// this is the survivor season, so an episode river-meta-tv already created
	// there gets its file_path backfilled instead of a duplicate being made.
	if existing, err := s.episodes.FindBySeasonAndNumber(season.ID.String(), input.Number, input.IsSpecial); err == nil {
		return s.updateEpisodeFields(existing, input)
	}
	episode := models.Episode{
		SeasonID:    season.ID,
		TVShowID:    season.TVShowID,
		Number:      input.Number,
		Title:       input.Title,
		Description: input.Description,
		Runtime:     input.Runtime,
		FilePath:    input.FilePath,
		SourcePath:  input.SourcePath,
		IsSpecial:   input.IsSpecial,
	}
	if input.AiredAt != "" {
		if t, err := time.Parse(time.RFC3339, input.AiredAt); err == nil {
			episode.AiredAt = t
		}
	}
	return &episode, s.episodes.Create(&episode)
}

func (s *TVShowService) GetEpisode(id string) (*models.Episode, error) {
	return s.episodes.FindByID(id)
}

// DeleteEpisode removes a single episode row. Caller (the handler) is
// responsible for any on-disk file removal + scanner-state cleanup —
// this layer only touches the DB.
func (s *TVShowService) DeleteEpisode(id string) error {
	if err := s.cleanup.PurgeEpisode(id); err != nil {
		return err
	}
	return s.episodes.Delete(id)
}

func (s *TVShowService) UpdateEpisode(id string, input EpisodeInput) (*models.Episode, error) {
	episode, err := s.episodes.FindByID(id)
	if err != nil {
		return nil, err
	}
	return s.updateEpisodeFields(episode, input)
}

// UpdateEpisodeSourcePath sets only the source_path on the episode. Used by
// river-scan to backfill the original on-disk location on rows that pre-date
// the field (or were created via a path that didn't populate it). Targeted
// so it can't race-clobber any other field meta-tv has written.
func (s *TVShowService) UpdateEpisodeSourcePath(id, path string) (*models.Episode, error) {
	episode, err := s.episodes.FindByID(id)
	if err != nil {
		return nil, err
	}
	episode.SourcePath = path
	return episode, s.episodes.Save(episode)
}

func (s *TVShowService) updateEpisodeFields(episode *models.Episode, input EpisodeInput) (*models.Episode, error) {
	if input.Title != "" {
		episode.Title = input.Title
	}
	if input.Description != "" {
		episode.Description = input.Description
	}
	if input.Runtime > 0 {
		episode.Runtime = input.Runtime
	}
	if input.Number > 0 {
		episode.Number = input.Number
	}
	if input.SeasonID != "" {
		// Verify the new season exists and belongs to the same show before
		// reparenting — otherwise an admin could orphan the episode onto a
		// season from an unrelated show.
		newSeason, err := s.seasons.FindByIDAndShowID(input.SeasonID, episode.TVShowID.String())
		if err != nil {
			return nil, err
		}
		episode.SeasonID = newSeason.ID
	}
	if input.SourcePath != "" {
		episode.SourcePath = input.SourcePath
	}
	if input.FilePath != "" {
		episode.FilePath = input.FilePath
	}
	if input.AiredAt != "" {
		if t, err := time.Parse(time.RFC3339, input.AiredAt); err == nil {
			episode.AiredAt = t
		}
	}
	return episode, s.episodes.Save(episode)
}

// deduplicateEpisodes merges duplicate episodes (same season+number) that can
// arise from a race between the transcoder and metadata services both creating
// the same episode. Prefers the copy with the richer title, merges file_path.
// Specials are keyed separately from regular episodes so special #1 doesn't
// collapse into regular E01.
func deduplicateEpisodes(eps []models.Episode) []models.Episode {
	type key struct {
		number    int
		isSpecial bool
	}
	type group struct {
		best     models.Episode
		filePath string
	}
	byKey := make(map[key]*group, len(eps))
	for _, ep := range eps {
		k := key{ep.Number, ep.IsSpecial}
		g, exists := byKey[k]
		if !exists {
			g = &group{best: ep}
			byKey[k] = g
		} else if episodeScore(ep) > episodeScore(g.best) {
			// Keep file_path from either copy
			if g.filePath == "" {
				g.filePath = g.best.FilePath
			}
			g.best = ep
		}
		if ep.FilePath != "" {
			g.filePath = ep.FilePath
		}
	}
	result := make([]models.Episode, 0, len(byKey))
	for _, g := range byKey {
		if g.filePath != "" {
			g.best.FilePath = g.filePath
		}
		result = append(result, g.best)
	}
	// Regular eps first (sorted by number), then specials (sorted by their
	// sequence). Keeps the list visually ordered when a season has both.
	sort.Slice(result, func(i, j int) bool {
		if result[i].IsSpecial != result[j].IsSpecial {
			return !result[i].IsSpecial
		}
		return result[i].Number < result[j].Number
	})
	return result
}

// episodeScore rates how "complete" an episode record is — used when picking
// the canonical copy during deduplication.
func episodeScore(ep models.Episode) int {
	score := 0
	if ep.Title != "" {
		score += 2
	}
	if ep.Description != "" {
		score += 2
	}
	if ep.Runtime > 0 {
		score++
	}
	if !ep.AiredAt.IsZero() {
		score++
	}
	return score
}
