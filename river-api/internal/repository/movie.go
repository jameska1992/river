package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type MovieRepository interface {
	// FindAll returns a page of movies, optionally filtered by libraryID
	// and ordered by the supplied SQL ORDER BY expression. The caller is
	// responsible for whitelisting the expression (see services/sort.go).
	// An empty orderBy preserves the storage engine's natural order.
	FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Movie, error)
	// Count returns the total number of movies matching the libraryID
	// filter applied by FindAll. Used by paginated list endpoints to
	// expose a total so the UI can render proper page navigation.
	Count(libraryID string) (int64, error)
	FindRecent(limit int) ([]models.Movie, error)
	FindUnidentified() ([]models.Movie, error)
	FindByID(id string) (*models.Movie, error)
	Create(movie *models.Movie) error
	Save(movie *models.Movie) error
	Delete(id string) error
}

type movieRepository struct{ db *gorm.DB }

func NewMovieRepository(db *gorm.DB) MovieRepository { return &movieRepository{db} }

func (r *movieRepository) FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Movie, error) {
	var movies []models.Movie
	q := r.db.Model(&models.Movie{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	if orderBy != "" {
		q = q.Order(orderBy)
	}
	return movies, q.Offset(offset).Limit(limit).Find(&movies).Error
}

func (r *movieRepository) Count(libraryID string) (int64, error) {
	var n int64
	q := r.db.Model(&models.Movie{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	return n, q.Count(&n).Error
}

func (r *movieRepository) FindRecent(limit int) ([]models.Movie, error) {
	var movies []models.Movie
	return movies, r.db.
		Where("poster_path <> '' OR backdrop_path <> ''").
		Order("created_at DESC").
		Limit(limit).
		Find(&movies).Error
}

// FindUnidentified returns movies that the metadata enhancer hasn't
// populated yet — empty poster_path is the reliable discriminator since
// TMDB returns a poster URL for every real movie. This list includes both
// "freshly scanned, enrichment pending" and "enrichment ran but found no
// match" — both are equally actionable by an admin clicking Identify.
func (r *movieRepository) FindUnidentified() ([]models.Movie, error) {
	var movies []models.Movie
	return movies, r.db.
		Where("poster_path = ''").
		Order("created_at DESC").
		Find(&movies).Error
}

func (r *movieRepository) FindByID(id string) (*models.Movie, error) {
	var movie models.Movie
	if err := r.db.First(&movie, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &movie, nil
}

func (r *movieRepository) Create(movie *models.Movie) error {
	return r.db.Create(movie).Error
}

func (r *movieRepository) Save(movie *models.Movie) error {
	return r.db.Save(movie).Error
}

func (r *movieRepository) Delete(id string) error {
	return r.db.Delete(&models.Movie{}, "id = ?", id).Error
}
