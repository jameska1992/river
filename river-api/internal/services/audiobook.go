package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

type AudiobookService struct {
	books    repository.AudiobookRepository
	chapters repository.ChapterRepository
	cleanup  repository.MediaCleanupRepository
}

func NewAudiobookService(
	books repository.AudiobookRepository,
	chapters repository.ChapterRepository,
	cleanup repository.MediaCleanupRepository,
) *AudiobookService {
	return &AudiobookService{books: books, chapters: chapters, cleanup: cleanup}
}

type AudiobookFilter struct {
	LibraryID string
	Page      int
	Limit     int
	Sort      string // title | author | year | added (default: title)
	Order     string // asc | desc (default: asc)
}

var audiobookSortColumns = map[string]string{
	"title":  titleSortExpr("title"),
	"author": titleSortExpr("author"),
	"year":   "year",
	"added":  "created_at",
}

type AudiobookInput struct {
	LibraryID   uuid.UUID
	Title       string
	Author      string
	Narrator    string
	Description string
	Year        int
	Genre       string
	CoverPath   string
	Duration    int
}

func (s *AudiobookService) List(f AudiobookFilter) ([]models.Audiobook, error) {
	offset, limit := paginationOffsetLimit(f.Page, f.Limit)
	return s.books.FindAll(f.LibraryID, offset, limit, sortClause(f.Sort, f.Order, audiobookSortColumns, titleSortExpr("title")))
}

func (s *AudiobookService) Count(libraryID string) (int64, error) {
	return s.books.Count(libraryID)
}

// Similar returns up to limit audiobooks in the same Genre as the given
// one, ordered by created_at desc. Audiobooks store a single Genre
// string (not a JSON array like Movie / TVShow), and don't have a
// Rating, so the ranker's shared-count / rating dimensions degrade to
// a single-tier match — every candidate that shares the source's
// (only) genre scores 1, everything else 0. That's still the right
// shape for the row.
func (s *AudiobookService) Similar(id string, limit int) ([]SimilarItem, error) {
	source, err := s.books.FindByID(id)
	if err != nil {
		return nil, err
	}
	if source.Genre == "" {
		return []SimilarItem{}, nil
	}
	all, err := s.books.FindAll("", 0, similarSourceLoadCap, "")
	if err != nil {
		return nil, err
	}
	candidates := make([]similarCandidate, 0, len(all))
	for _, b := range all {
		var genres []string
		if b.Genre != "" {
			genres = []string{b.Genre}
		}
		candidates = append(candidates, similarCandidate{
			ID:         b.ID.String(),
			Genres:     genres,
			CreatedAt:  b.CreatedAt,
			Title:      b.Title,
			Year:       b.Year,
			PosterPath: b.CoverPath,
			// Audiobooks have no wide "backdrop" artwork — the client
			// falls back to the cover for the landscape card frame.
		})
	}
	ranked := rankBySharedGenres(source.ID.String(), []string{source.Genre}, candidates, limit)
	return candidatesToSimilarItems(ranked, "audiobook"), nil
}

func (s *AudiobookService) Create(input AudiobookInput) (*models.Audiobook, error) {
	book := models.Audiobook{
		LibraryID: input.LibraryID, Title: input.Title, Author: input.Author,
		Narrator: input.Narrator, Description: input.Description, Year: input.Year,
		Genre: input.Genre, CoverPath: input.CoverPath, Duration: input.Duration,
	}
	return &book, s.books.Create(&book)
}

func (s *AudiobookService) GetByID(id string) (*models.Audiobook, error) {
	return s.books.FindByID(id)
}

func (s *AudiobookService) Update(id string, input AudiobookInput) (*models.Audiobook, error) {
	book, err := s.books.FindByID(id)
	if err != nil {
		return nil, err
	}
	book.Title = input.Title
	book.Author = input.Author
	book.Narrator = input.Narrator
	book.Description = input.Description
	book.Year = input.Year
	book.Genre = input.Genre
	book.CoverPath = input.CoverPath
	book.Duration = input.Duration
	return book, s.books.Save(book)
}

func (s *AudiobookService) Delete(id string) error {
	// Drop watchlist entries, collection memberships, audiobook-level
	// progress, and chapter-level progress refs before removing the
	// row itself.
	if err := s.cleanup.PurgeAudiobook(id); err != nil {
		return err
	}
	return s.books.Delete(id)
}

// --- Chapters ---

type ChapterInput struct {
	Number   int
	Title    string
	Duration int
	FilePath string
}

func (s *AudiobookService) ListChapters(audiobookID string) ([]models.AudiobookChapter, error) {
	return s.chapters.FindByAudiobookID(audiobookID)
}

func (s *AudiobookService) CreateChapter(audiobookID string, input ChapterInput) (*models.AudiobookChapter, error) {
	book, err := s.books.FindByID(audiobookID)
	if err != nil {
		return nil, err
	}
	chapter := models.AudiobookChapter{
		AudiobookID: book.ID,
		Number:      input.Number,
		Title:       input.Title,
		Duration:    input.Duration,
		FilePath:    input.FilePath,
	}
	return &chapter, s.chapters.Create(&chapter)
}

func (s *AudiobookService) GetChapter(id string) (*models.AudiobookChapter, error) {
	return s.chapters.FindByID(id)
}
