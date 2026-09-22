package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

type MovieService struct {
	repo    repository.MovieRepository
	cleanup repository.MediaCleanupRepository
}

func NewMovieService(repo repository.MovieRepository, cleanup repository.MediaCleanupRepository) *MovieService {
	return &MovieService{repo: repo, cleanup: cleanup}
}

type MovieFilter struct {
	LibraryID string
	Page      int
	Limit     int
	Sort      string // title | year | rating | added (default: title)
	Order     string // asc | desc (default: asc)
}

// movieSortColumns maps user-facing sort keys to the SQL expressions we
// allow callers to sort by. Anything not in this map falls back to title.
// Title is sorted via titleSortExpr (lowercased + leading article
// stripped) so the alphabetical order matches user expectations.
var movieSortColumns = map[string]string{
	"title":  titleSortExpr("title"),
	"year":   "year",
	"rating": "rating",
	"added":  "created_at",
}

type MovieInput struct {
	LibraryID     uuid.UUID
	Title         string
	OriginalTitle string
	Description   string
	Year          int
	Genres        string
	Rating        float32
	// Certification is sticky like TMDBID: Update only overwrites it when the
	// incoming value is non-empty, so admin metadata edits (which don't carry
	// it) and metadata fetches that find no GB rating can't blank it.
	Certification string
	Runtime       int
	PosterPath    string
	BackdropPath  string
	TrailerURL    string
	// TMDBID is sticky once set — Update only overwrites it when the
	// incoming value is non-zero, so generic edits (admin metadata form,
	// scanner backfills) can't accidentally erase it. Enrichment writes a
	// real id; everyone else passes zero.
	TMDBID     int
	FilePath   string
	SourcePath string
	// SizeBytes is the transcoded output size in bytes. Like FilePath it's
	// only overwritten by Update when non-zero, so a metadata edit that
	// carries no size can't zero out a measured value.
	SizeBytes int64
}

func (s *MovieService) List(f MovieFilter) ([]models.Movie, error) {
	offset, limit := paginationOffsetLimit(f.Page, f.Limit)
	return s.repo.FindAll(f.LibraryID, offset, limit, sortClause(f.Sort, f.Order, movieSortColumns, titleSortExpr("title")))
}

func (s *MovieService) Count(libraryID string) (int64, error) {
	return s.repo.Count(libraryID)
}

func (s *MovieService) ListRecent(limit int) ([]models.Movie, error) {
	return s.repo.FindRecent(limit)
}

func (s *MovieService) ListUnidentified() ([]models.Movie, error) {
	return s.repo.FindUnidentified()
}

// Similar returns up to limit movies whose genres overlap with the
// given movie's, ranked by shared-genre count desc → rating desc →
// created_at desc. Empty result when the source movie has no genres.
//
// Loads the whole movie table into memory and ranks in-Go; fine for
// libraries in the low thousands and simpler than teaching Postgres
// about the JSON-string genre column. If library size ever grows past
// ~50k, revisit — either move to an M:N genre_tags table or push the
// scoring into SQL.
func (s *MovieService) Similar(id string, limit int) ([]SimilarItem, error) {
	source, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	sourceGenres := parseGenresJSON(source.Genres)
	if len(sourceGenres) == 0 {
		return []SimilarItem{}, nil
	}
	// "" library filter → every movie. Large offset+limit sidesteps
	// pagination — we want the whole population for scoring.
	all, err := s.repo.FindAll("", 0, similarSourceLoadCap, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]similarCandidate, 0, len(all))
	for _, m := range all {
		candidates = append(candidates, similarCandidate{
			ID:           m.ID.String(),
			Genres:       parseGenresJSON(m.Genres),
			Rating:       m.Rating,
			CreatedAt:    m.CreatedAt,
			Title:        m.Title,
			Year:         m.Year,
			PosterPath:   m.PosterPath,
			BackdropPath: m.BackdropPath,
		})
	}
	ranked := rankBySharedGenres(source.ID.String(), sourceGenres, candidates, limit)
	return candidatesToSimilarItems(ranked, "movie"), nil
}

func (s *MovieService) Create(input MovieInput) (*models.Movie, error) {
	movie := models.Movie{
		LibraryID:     input.LibraryID,
		Title:         input.Title,
		OriginalTitle: input.OriginalTitle,
		Description:   input.Description,
		Year:          input.Year,
		Genres:        defaultJSON(input.Genres),
		Rating:        input.Rating,
		Certification: input.Certification,
		Runtime:       input.Runtime,
		PosterPath:    input.PosterPath,
		BackdropPath:  input.BackdropPath,
		TrailerURL:    input.TrailerURL,
		TMDBID:        input.TMDBID,
		FilePath:      input.FilePath,
		SourcePath:    input.SourcePath,
		SizeBytes:     input.SizeBytes,
	}
	return &movie, s.repo.Create(&movie)
}

func (s *MovieService) GetByID(id string) (*models.Movie, error) {
	return s.repo.FindByID(id)
}

func (s *MovieService) Update(id string, input MovieInput) (*models.Movie, error) {
	movie, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	movie.Title = input.Title
	movie.OriginalTitle = input.OriginalTitle
	movie.Description = input.Description
	movie.Year = input.Year
	movie.Rating = input.Rating
	if input.Certification != "" {
		movie.Certification = input.Certification
	}
	movie.Runtime = input.Runtime
	movie.PosterPath = input.PosterPath
	movie.BackdropPath = input.BackdropPath
	movie.TrailerURL = input.TrailerURL
	if input.FilePath != "" {
		movie.FilePath = input.FilePath
	}
	if input.SourcePath != "" {
		movie.SourcePath = input.SourcePath
	}
	if input.SizeBytes > 0 {
		movie.SizeBytes = input.SizeBytes
	}
	if input.Genres != "" {
		movie.Genres = input.Genres
	}
	// TMDBID is intentionally only overwritten when the caller provides
	// one — generic metadata edits send 0 and must not erase the resolved
	// id, otherwise the next enrichment would re-search by title/year
	// and could land on a different popular candidate.
	if input.TMDBID > 0 {
		movie.TMDBID = input.TMDBID
	}
	return movie, s.repo.Save(movie)
}

// UpdateSourcePath sets only the SourcePath. Used by river-scan to record
// the original on-disk location on rows it pre-created before video-trans
// finalized FilePath. Targeted so it can't race-clobber meta-movie writes.
func (s *MovieService) UpdateSourcePath(id, path string) (*models.Movie, error) {
	movie, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	movie.SourcePath = path
	return movie, s.repo.Save(movie)
}

// UpdateFilePath sets only the FilePath on the movie, leaving every other
// field untouched. Used by river-video-trans so a long-running transcode
// can't clobber metadata that river-meta-movie wrote during the window
// between the transcoder's GetMovie and its post-transcode update.
func (s *MovieService) UpdateFilePath(id, path string, sizeBytes int64) (*models.Movie, error) {
	movie, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	movie.FilePath = path
	// Record the transcoded output size alongside the path (they're set
	// together at transcode time). Only when >0 so a caller that doesn't
	// measure can't zero an existing value.
	if sizeBytes > 0 {
		movie.SizeBytes = sizeBytes
	}
	return movie, s.repo.Save(movie)
}

func (s *MovieService) Delete(id string) error {
	// Drop cross-reference rows (watchlist entries, cast/crew links,
	// collection memberships, watch-progress + per-track refs) before
	// the row itself so the references never outlive the target.
	if err := s.cleanup.PurgeMovie(id); err != nil {
		return err
	}
	return s.repo.Delete(id)
}

// ClearTMDBID resets the resolved TMDB id back to 0 so the next
// enrichment runs a fresh title+year search rather than re-fetching the
// previously-resolved record. Used by the admin Identify flow when the
// admin changes title or year — the stored id from the prior (now
// incorrect) enrichment would otherwise short-circuit the new search
// via the sticky-id branch in river-meta-movie's fetch.
func (s *MovieService) ClearTMDBID(id string) error {
	movie, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	movie.TMDBID = 0
	return s.repo.Save(movie)
}
