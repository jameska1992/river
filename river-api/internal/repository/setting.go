package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SettingRepository interface {
	// Get returns the value for key, or apperrors.ErrNotFound if unset.
	Get(key string) (string, error)
	// Set upserts key to value.
	Set(key, value string) error
	// SetIfAbsent inserts key=value only when key does not already exist.
	// Returns true when a row was inserted. Used to seed config from env
	// without clobbering values an admin has since edited.
	SetIfAbsent(key, value string) (bool, error)
}

type settingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) SettingRepository { return &settingRepository{db} }

func (r *settingRepository) Get(key string) (string, error) {
	var s models.Setting
	if err := r.db.Where("key = ?", key).First(&s).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", apperrors.ErrNotFound
		}
		return "", err
	}
	return s.Value, nil
}

func (r *settingRepository) Set(key, value string) error {
	s := models.Setting{Key: key, Value: value}
	return r.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "updated_at"}),
	}).Create(&s).Error
}

func (r *settingRepository) SetIfAbsent(key, value string) (bool, error) {
	s := models.Setting{Key: key, Value: value}
	res := r.db.Clauses(clause.OnConflict{DoNothing: true}).Create(&s)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected > 0, nil
}
