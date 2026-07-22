package repository

import (
	"errors"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type ProgressRepository interface {
	Upsert(p *models.WatchProgress) error
	Find(userID, mediaType, mediaID string) (*models.WatchProgress, error)
	FindInProgress(userID string, limit int) ([]models.WatchProgress, error)
	FindAllByType(userID, mediaType string) ([]models.WatchProgress, error)
	FindAllActive(since time.Time, limit int) ([]models.WatchProgress, error)
	FindByUser(userID string, limit int) ([]models.WatchProgress, error)
	// FindCompletedEpisodes returns every completed episode-progress row
	// for the user, ordered by updated_at DESC. Powers the Next Up rail:
	// callers scan the result once and take each show's first sighting
	// as the anchor for "what to play next."
	FindCompletedEpisodes(userID string) ([]models.WatchProgress, error)
	Delete(userID, mediaType, mediaID string) error
}

type progressRepository struct{ db *gorm.DB }

func NewProgressRepository(db *gorm.DB) ProgressRepository { return &progressRepository{db} }

func (r *progressRepository) Upsert(p *models.WatchProgress) error {
	var existing models.WatchProgress
	// Unscoped so a previously soft-deleted row is matched and resurrected
	// rather than colliding with the (user_id, media_type, media_id)
	// unique index on a fresh Create.
	err := r.db.Unscoped().
		Where("user_id = ? AND media_type = ? AND media_id = ?", p.UserID, p.MediaType, p.MediaID).
		First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(p).Error
		}
		return err
	}
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	// Unscoped Save also clears any deleted_at, completing the restore.
	return r.db.Unscoped().Save(p).Error
}

func (r *progressRepository) Find(userID, mediaType, mediaID string) (*models.WatchProgress, error) {
	var p models.WatchProgress
	err := r.db.Where("user_id = ? AND media_type = ? AND media_id = ?", userID, mediaType, mediaID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *progressRepository) FindInProgress(userID string, limit int) ([]models.WatchProgress, error) {
	var items []models.WatchProgress
	return items, r.db.
		Where("user_id = ? AND completed = false", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&items).Error
}

func (r *progressRepository) FindAllByType(userID, mediaType string) ([]models.WatchProgress, error) {
	var items []models.WatchProgress
	return items, r.db.
		Where("user_id = ? AND media_type = ?", userID, mediaType).
		Find(&items).Error
}

func (r *progressRepository) FindAllActive(since time.Time, limit int) ([]models.WatchProgress, error) {
	var items []models.WatchProgress
	return items, r.db.
		Where("completed = false AND updated_at > ?", since).
		Order("updated_at DESC").
		Limit(limit).
		Find(&items).Error
}

func (r *progressRepository) FindByUser(userID string, limit int) ([]models.WatchProgress, error) {
	var items []models.WatchProgress
	return items, r.db.
		Where("user_id = ?", userID).
		Order("updated_at DESC").
		Limit(limit).
		Find(&items).Error
}

func (r *progressRepository) FindCompletedEpisodes(userID string) ([]models.WatchProgress, error) {
	var items []models.WatchProgress
	return items, r.db.
		Where("user_id = ? AND media_type = ? AND completed = true", userID, "episode").
		Order("updated_at DESC").
		Find(&items).Error
}

func (r *progressRepository) Delete(userID, mediaType, mediaID string) error {
	// Hard delete — watch progress doesn't need soft-delete history, and a
	// soft-deleted row would otherwise block a future mark-watched Upsert
	// via the (user_id, media_type, media_id) unique index.
	return r.db.Unscoped().
		Where("user_id = ? AND media_type = ? AND media_id = ?", userID, mediaType, mediaID).
		Delete(&models.WatchProgress{}).Error
}
