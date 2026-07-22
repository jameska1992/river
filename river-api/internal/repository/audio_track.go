package repository

import (
	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type AudioTrackInput struct {
	MediaType   string
	MediaID     string
	Language    string
	Label       string
	StreamIndex int
	FilePath    string
}

type AudioTrackRepository interface {
	Create(input AudioTrackInput) (*models.AudioTrack, error)
	ListByMedia(mediaType, mediaID string) ([]models.AudioTrack, error)
	FindByID(id string) (*models.AudioTrack, error)
	Delete(id string) error
}

type audioTrackRepository struct{ db *gorm.DB }

func NewAudioTrackRepository(db *gorm.DB) AudioTrackRepository {
	return &audioTrackRepository{db: db}
}

func (r *audioTrackRepository) Create(input AudioTrackInput) (*models.AudioTrack, error) {
	t := &models.AudioTrack{
		MediaType:   input.MediaType,
		MediaID:     input.MediaID,
		Language:    input.Language,
		Label:       input.Label,
		StreamIndex: input.StreamIndex,
		FilePath:    input.FilePath,
	}
	if err := r.db.Create(t).Error; err != nil {
		return nil, err
	}
	return t, nil
}

func (r *audioTrackRepository) ListByMedia(mediaType, mediaID string) ([]models.AudioTrack, error) {
	var tracks []models.AudioTrack
	err := r.db.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Order("stream_index").Find(&tracks).Error
	return tracks, err
}

func (r *audioTrackRepository) FindByID(id string) (*models.AudioTrack, error) {
	var t models.AudioTrack
	err := r.db.First(&t, "id = ?", id).Error
	if err == gorm.ErrRecordNotFound {
		return nil, apperrors.ErrNotFound
	}
	return &t, err
}

func (r *audioTrackRepository) Delete(id string) error {
	res := r.db.Delete(&models.AudioTrack{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
