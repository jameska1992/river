package repository

import (
	"errors"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type APITokenRepository interface {
	Create(t *models.APIToken) error
	// FindByHash returns the non-revoked token for a hash, or ErrNotFound.
	// Expiry is checked by the caller so an expired token can be reported
	// distinctly if needed.
	FindByHash(hash string) (*models.APIToken, error)
	List() ([]models.APIToken, error)
	Revoke(id string) error
	TouchLastUsed(id string, at time.Time) error
}

type gormAPITokenRepository struct{ db *gorm.DB }

func NewAPITokenRepository(db *gorm.DB) APITokenRepository {
	return &gormAPITokenRepository{db: db}
}

func (r *gormAPITokenRepository) Create(t *models.APIToken) error {
	return r.db.Create(t).Error
}

func (r *gormAPITokenRepository) FindByHash(hash string) (*models.APIToken, error) {
	var t models.APIToken
	err := r.db.Where("token_hash = ? AND revoked = ?", hash, false).First(&t).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *gormAPITokenRepository) List() ([]models.APIToken, error) {
	var out []models.APIToken
	return out, r.db.Order("created_at DESC").Find(&out).Error
}

func (r *gormAPITokenRepository) Revoke(id string) error {
	res := r.db.Model(&models.APIToken{}).Where("id = ?", id).Update("revoked", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *gormAPITokenRepository) TouchLastUsed(id string, at time.Time) error {
	return r.db.Model(&models.APIToken{}).Where("id = ?", id).UpdateColumn("last_used_at", at).Error
}
