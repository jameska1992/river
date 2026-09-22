package repository

import (
	"errors"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type ServiceKeyRepository interface {
	// Create inserts a new key. Fails on a duplicate name (unique index).
	Create(k *models.ServiceKey) error
	// CreateIfAbsent inserts only when no key with this name exists yet.
	// Returns true when a row was actually created. Powers env-seed.
	CreateIfAbsent(k *models.ServiceKey) (bool, error)
	// FindByHash returns the (non-revoked) key matching the given hash, or
	// ErrNotFound. Used on every key-authenticated request.
	FindByHash(hash string) (*models.ServiceKey, error)
	FindByName(name string) (*models.ServiceKey, error)
	List() ([]models.ServiceKey, error)
	Revoke(id string) error
	// TouchLastUsed records that a key was just used. Best-effort.
	TouchLastUsed(id string, at time.Time) error
}

type gormServiceKeyRepository struct{ db *gorm.DB }

func NewServiceKeyRepository(db *gorm.DB) ServiceKeyRepository {
	return &gormServiceKeyRepository{db: db}
}

func (r *gormServiceKeyRepository) Create(k *models.ServiceKey) error {
	return r.db.Create(k).Error
}

func (r *gormServiceKeyRepository) CreateIfAbsent(k *models.ServiceKey) (bool, error) {
	var existing models.ServiceKey
	err := r.db.Where("name = ?", k.Name).First(&existing).Error
	if err == nil {
		return false, nil // name already present — leave it untouched
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return false, err
	}
	if err := r.db.Create(k).Error; err != nil {
		return false, err
	}
	return true, nil
}

func (r *gormServiceKeyRepository) FindByHash(hash string) (*models.ServiceKey, error) {
	var k models.ServiceKey
	err := r.db.Where("key_hash = ? AND revoked = ?", hash, false).First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *gormServiceKeyRepository) FindByName(name string) (*models.ServiceKey, error) {
	var k models.ServiceKey
	err := r.db.Where("name = ?", name).First(&k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &k, nil
}

func (r *gormServiceKeyRepository) List() ([]models.ServiceKey, error) {
	var keys []models.ServiceKey
	return keys, r.db.Order("name").Find(&keys).Error
}

func (r *gormServiceKeyRepository) Revoke(id string) error {
	res := r.db.Model(&models.ServiceKey{}).Where("id = ?", id).Update("revoked", true)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

func (r *gormServiceKeyRepository) TouchLastUsed(id string, at time.Time) error {
	return r.db.Model(&models.ServiceKey{}).Where("id = ?", id).UpdateColumn("last_used_at", at).Error
}
