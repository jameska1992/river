package repository

import (
	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type SubtitleInput struct {
	MediaType string
	MediaID   string
	Language  string
	Label     string
	FilePath  string
}

type SubtitleRepository interface {
	Create(input SubtitleInput) (*models.Subtitle, error)
	ListByMedia(mediaType, mediaID string) ([]models.Subtitle, error)
	FindByID(id string) (*models.Subtitle, error)
	Delete(id string) error
}

type subtitleRepository struct{ db *gorm.DB }

func NewSubtitleRepository(db *gorm.DB) SubtitleRepository {
	return &subtitleRepository{db: db}
}

func (r *subtitleRepository) Create(input SubtitleInput) (*models.Subtitle, error) {
	s := &models.Subtitle{
		MediaType: input.MediaType,
		MediaID:   input.MediaID,
		Language:  input.Language,
		Label:     input.Label,
		FilePath:  input.FilePath,
	}
	if err := r.db.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

func (r *subtitleRepository) ListByMedia(mediaType, mediaID string) ([]models.Subtitle, error) {
	var subs []models.Subtitle
	err := r.db.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Order("language").Find(&subs).Error
	return subs, err
}

func (r *subtitleRepository) FindByID(id string) (*models.Subtitle, error) {
	var s models.Subtitle
	err := r.db.First(&s, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	return &s, err
}

func (r *subtitleRepository) Delete(id string) error {
	res := r.db.Delete(&models.Subtitle{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
