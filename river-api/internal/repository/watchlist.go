package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type WatchlistRepository interface {
	Add(userID, mediaType, mediaID string) (*models.WatchlistItem, error)
	Remove(userID, itemID string) error
	List(userID string) ([]models.WatchlistItem, error)
}

type watchlistRepository struct{ db *gorm.DB }

func NewWatchlistRepository(db *gorm.DB) WatchlistRepository {
	return &watchlistRepository{db: db}
}

func (r *watchlistRepository) Add(userID, mediaType, mediaID string) (*models.WatchlistItem, error) {
	var item models.WatchlistItem
	err := r.db.Where("user_id = ? AND media_type = ? AND media_id = ?", userID, mediaType, mediaID).
		First(&item).Error
	if err == nil {
		return &item, nil // already exists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	item = models.WatchlistItem{UserID: userID, MediaType: mediaType, MediaID: mediaID}
	if err := r.db.Create(&item).Error; err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *watchlistRepository) Remove(userID, itemID string) error {
	result := r.db.Unscoped().Where("id = ? AND user_id = ?", itemID, userID).Delete(&models.WatchlistItem{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *watchlistRepository) List(userID string) ([]models.WatchlistItem, error) {
	var items []models.WatchlistItem
	return items, r.db.
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&items).Error
}
