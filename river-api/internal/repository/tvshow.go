package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

// --- TV Shows ---

type TVShowRepository interface {
	// FindAllUnpaged returns every TV show in libraryID (or every show
	// when libraryID is empty). Used by aggregation paths that need the
	// full set in one shot rather than paginating.
	FindAllUnpaged(libraryID string) ([]models.TVShow, error)
	// FindAll returns a page of TV shows. See MovieRepository.FindAll for
	// the orderBy contract.
	FindAll(libraryID string, offset, limit int, orderBy string) ([]models.TVShow, error)
	// Count returns the total number of shows matching the libraryID
	// filter applied by FindAll. See MovieRepository.Count.
	Count(libraryID string) (int64, error)
	FindRecent(limit int) ([]models.TVShow, error)
	FindUnidentified() ([]models.TVShow, error)
	FindByID(id string) (*models.TVShow, error)
	Create(show *models.TVShow) error
	Save(show *models.TVShow) error
	Delete(id string) error
}

type tvShowRepository struct{ db *gorm.DB }

func NewTVShowRepository(db *gorm.DB) TVShowRepository { return &tvShowRepository{db} }

func (r *tvShowRepository) FindAllUnpaged(libraryID string) ([]models.TVShow, error) {
	var shows []models.TVShow
	q := r.db.Model(&models.TVShow{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	return shows, q.Find(&shows).Error
}

func (r *tvShowRepository) FindAll(libraryID string, offset, limit int, orderBy string) ([]models.TVShow, error) {
	var shows []models.TVShow
	q := r.db.Model(&models.TVShow{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	if orderBy != "" {
		q = q.Order(orderBy)
	}
	return shows, q.Offset(offset).Limit(limit).Find(&shows).Error
}

func (r *tvShowRepository) Count(libraryID string) (int64, error) {
	var n int64
	q := r.db.Model(&models.TVShow{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	return n, q.Count(&n).Error
}

func (r *tvShowRepository) FindRecent(limit int) ([]models.TVShow, error) {
	var shows []models.TVShow
	return shows, r.db.
		Where("poster_path <> '' OR backdrop_path <> ''").
		Order("created_at DESC").
		Limit(limit).
		Find(&shows).Error
}

// FindUnidentified returns shows the metadata enhancer hasn't populated yet
// — empty poster_path is the reliable discriminator. Includes both
// freshly-scanned-pending and enrichment-failed cases.
func (r *tvShowRepository) FindUnidentified() ([]models.TVShow, error) {
	var shows []models.TVShow
	return shows, r.db.
		Where("poster_path = ''").
		Order("created_at DESC").
		Find(&shows).Error
}

func (r *tvShowRepository) FindByID(id string) (*models.TVShow, error) {
	var show models.TVShow
	if err := r.db.First(&show, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &show, nil
}

func (r *tvShowRepository) Create(show *models.TVShow) error  { return r.db.Create(show).Error }
func (r *tvShowRepository) Save(show *models.TVShow) error    { return r.db.Save(show).Error }
func (r *tvShowRepository) Delete(id string) error            { return r.db.Delete(&models.TVShow{}, "id = ?", id).Error }

// --- Seasons ---

type SeasonRepository interface {
	FindByShowID(showID string) ([]models.Season, error)
	FindByID(id string) (*models.Season, error)
	FindByIDAndShowID(id, showID string) (*models.Season, error)
	Create(season *models.Season) error
	Save(season *models.Season) error
}

type seasonRepository struct{ db *gorm.DB }

func NewSeasonRepository(db *gorm.DB) SeasonRepository { return &seasonRepository{db} }

func (r *seasonRepository) FindByShowID(showID string) ([]models.Season, error) {
	var seasons []models.Season
	return seasons, r.db.Where("tv_show_id = ?", showID).Find(&seasons).Error
}

func (r *seasonRepository) FindByID(id string) (*models.Season, error) {
	var season models.Season
	if err := r.db.First(&season, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &season, nil
}

func (r *seasonRepository) FindByIDAndShowID(id, showID string) (*models.Season, error) {
	var season models.Season
	if err := r.db.First(&season, "id = ? AND tv_show_id = ?", id, showID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &season, nil
}

func (r *seasonRepository) Create(season *models.Season) error { return r.db.Create(season).Error }
func (r *seasonRepository) Save(season *models.Season) error   { return r.db.Save(season).Error }

// --- Episodes ---

type EpisodeRepository interface {
	FindBySeasonID(seasonID string) ([]models.Episode, error)
	FindByShowID(showID string) ([]models.Episode, error)
	FindByID(id string) (*models.Episode, error)
	FindBySeasonAndNumber(seasonID string, number int, isSpecial bool) (*models.Episode, error)
	Create(episode *models.Episode) error
	Save(episode *models.Episode) error
	Delete(id string) error
}

type episodeRepository struct{ db *gorm.DB }

func NewEpisodeRepository(db *gorm.DB) EpisodeRepository { return &episodeRepository{db} }

func (r *episodeRepository) FindBySeasonID(seasonID string) ([]models.Episode, error) {
	var episodes []models.Episode
	return episodes, r.db.Where("season_id = ?", seasonID).Find(&episodes).Error
}

func (r *episodeRepository) FindByShowID(showID string) ([]models.Episode, error) {
	var episodes []models.Episode
	return episodes, r.db.Where("tv_show_id = ?", showID).Find(&episodes).Error
}

func (r *episodeRepository) FindByID(id string) (*models.Episode, error) {
	var episode models.Episode
	if err := r.db.First(&episode, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &episode, nil
}

func (r *episodeRepository) FindBySeasonAndNumber(seasonID string, number int, isSpecial bool) (*models.Episode, error) {
	var ep models.Episode
	if err := r.db.Where("season_id = ? AND number = ? AND is_special = ?", seasonID, number, isSpecial).First(&ep).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &ep, nil
}

func (r *episodeRepository) Create(episode *models.Episode) error { return r.db.Create(episode).Error }
func (r *episodeRepository) Save(episode *models.Episode) error   { return r.db.Save(episode).Error }
func (r *episodeRepository) Delete(id string) error               { return r.db.Delete(&models.Episode{}, "id = ?", id).Error }
