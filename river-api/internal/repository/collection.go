package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type CollectionRepository interface {
	// FindAll returns every collection across all users. Collections are
	// global: any authenticated user can see/edit any of them. The
	// Collection.UserID column is preserved as a "created by" audit field
	// and is not used for filtering.
	FindAll() ([]models.Collection, error)
	FindByID(id string) (*models.Collection, error)
	Create(c *models.Collection) error
	Save(c *models.Collection) error
	Delete(id string) error
	FindItems(collectionID string) ([]models.CollectionItem, error)
	FindItem(id string) (*models.CollectionItem, error)
	FindItemByMedia(collectionID, mediaType, mediaID string) (*models.CollectionItem, error)
	AddItem(item *models.CollectionItem) error
	RemoveItem(id string) error
}

type collectionRepository struct{ db *gorm.DB }

func NewCollectionRepository(db *gorm.DB) CollectionRepository { return &collectionRepository{db} }

func (r *collectionRepository) FindAll() ([]models.Collection, error) {
	var cols []models.Collection
	return cols, r.db.Order("created_at DESC").Find(&cols).Error
}

func (r *collectionRepository) FindByID(id string) (*models.Collection, error) {
	var c models.Collection
	if err := r.db.First(&c, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &c, nil
}

func (r *collectionRepository) Create(c *models.Collection) error {
	return r.db.Create(c).Error
}

func (r *collectionRepository) Save(c *models.Collection) error {
	return r.db.Save(c).Error
}

func (r *collectionRepository) Delete(id string) error {
	if err := r.db.Where("collection_id = ?", id).Delete(&models.CollectionItem{}).Error; err != nil {
		return err
	}
	return r.db.Delete(&models.Collection{}, "id = ?", id).Error
}

func (r *collectionRepository) FindItems(collectionID string) ([]models.CollectionItem, error) {
	var items []models.CollectionItem
	return items, r.db.
		Where("collection_id = ?", collectionID).
		Order("sort_order ASC, created_at ASC").
		Find(&items).Error
}

func (r *collectionRepository) FindItem(id string) (*models.CollectionItem, error) {
	var item models.CollectionItem
	if err := r.db.First(&item, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *collectionRepository) FindItemByMedia(collectionID, mediaType, mediaID string) (*models.CollectionItem, error) {
	var item models.CollectionItem
	err := r.db.Where("collection_id = ? AND media_type = ? AND media_id = ?", collectionID, mediaType, mediaID).
		First(&item).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &item, nil
}

func (r *collectionRepository) AddItem(item *models.CollectionItem) error {
	return r.db.Create(item).Error
}

func (r *collectionRepository) RemoveItem(id string) error {
	return r.db.Delete(&models.CollectionItem{}, "id = ?", id).Error
}
