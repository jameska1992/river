package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type LibraryRepository interface {
	FindAll() ([]models.Library, error)
	FindByID(id string) (*models.Library, error)
	Create(lib *models.Library) error
	Save(lib *models.Library) error
	Delete(id string) error
}

type libraryRepository struct{ db *gorm.DB }

func NewLibraryRepository(db *gorm.DB) LibraryRepository { return &libraryRepository{db} }

func (r *libraryRepository) FindAll() ([]models.Library, error) {
	var libs []models.Library
	return libs, r.db.Find(&libs).Error
}

func (r *libraryRepository) FindByID(id string) (*models.Library, error) {
	var lib models.Library
	if err := r.db.First(&lib, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &lib, nil
}

func (r *libraryRepository) Create(lib *models.Library) error {
	return r.db.Create(lib).Error
}

func (r *libraryRepository) Save(lib *models.Library) error {
	return r.db.Save(lib).Error
}

func (r *libraryRepository) Delete(id string) error {
	return r.db.Delete(&models.Library{}, "id = ?", id).Error
}
