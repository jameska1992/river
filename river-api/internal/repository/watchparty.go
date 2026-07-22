package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type WatchPartyRepository interface {
	Create(party *models.WatchParty) error
	FindByID(id string) (*models.WatchParty, error)
	Delete(id string) error
}

type watchPartyRepository struct{ db *gorm.DB }

func NewWatchPartyRepository(db *gorm.DB) WatchPartyRepository {
	return &watchPartyRepository{db}
}

func (r *watchPartyRepository) Create(party *models.WatchParty) error {
	return r.db.Create(party).Error
}

func (r *watchPartyRepository) FindByID(id string) (*models.WatchParty, error) {
	var party models.WatchParty
	if err := r.db.First(&party, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &party, nil
}

func (r *watchPartyRepository) Delete(id string) error {
	return r.db.Delete(&models.WatchParty{}, "id = ?", id).Error
}
