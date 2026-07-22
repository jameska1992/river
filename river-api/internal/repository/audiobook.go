package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

// --- Audiobooks ---

type AudiobookRepository interface {
	// FindAll returns a page of audiobooks. See MovieRepository.FindAll
	// for the orderBy contract.
	FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Audiobook, error)
	// Count returns the total number of audiobooks matching the libraryID
	// filter applied by FindAll. See MovieRepository.Count.
	Count(libraryID string) (int64, error)
	FindByID(id string) (*models.Audiobook, error)
	Create(book *models.Audiobook) error
	Save(book *models.Audiobook) error
	Delete(id string) error
}

type audiobookRepository struct{ db *gorm.DB }

func NewAudiobookRepository(db *gorm.DB) AudiobookRepository { return &audiobookRepository{db} }

func (r *audiobookRepository) FindAll(libraryID string, offset, limit int, orderBy string) ([]models.Audiobook, error) {
	var books []models.Audiobook
	q := r.db.Model(&models.Audiobook{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	if orderBy != "" {
		q = q.Order(orderBy)
	}
	return books, q.Offset(offset).Limit(limit).Find(&books).Error
}

func (r *audiobookRepository) Count(libraryID string) (int64, error) {
	var n int64
	q := r.db.Model(&models.Audiobook{})
	if libraryID != "" {
		q = q.Where("library_id = ?", libraryID)
	}
	return n, q.Count(&n).Error
}

func (r *audiobookRepository) FindByID(id string) (*models.Audiobook, error) {
	var book models.Audiobook
	if err := r.db.First(&book, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &book, nil
}

func (r *audiobookRepository) Create(book *models.Audiobook) error { return r.db.Create(book).Error }
func (r *audiobookRepository) Save(book *models.Audiobook) error   { return r.db.Save(book).Error }
func (r *audiobookRepository) Delete(id string) error              { return r.db.Delete(&models.Audiobook{}, "id = ?", id).Error }

// --- Chapters ---

type ChapterRepository interface {
	FindByAudiobookID(audiobookID string) ([]models.AudiobookChapter, error)
	FindByID(id string) (*models.AudiobookChapter, error)
	Create(chapter *models.AudiobookChapter) error
}

type chapterRepository struct{ db *gorm.DB }

func NewChapterRepository(db *gorm.DB) ChapterRepository { return &chapterRepository{db} }

func (r *chapterRepository) FindByAudiobookID(audiobookID string) ([]models.AudiobookChapter, error) {
	var chapters []models.AudiobookChapter
	return chapters, r.db.Where("audiobook_id = ?", audiobookID).Order("number").Find(&chapters).Error
}

func (r *chapterRepository) FindByID(id string) (*models.AudiobookChapter, error) {
	var chapter models.AudiobookChapter
	if err := r.db.First(&chapter, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &chapter, nil
}

func (r *chapterRepository) Create(chapter *models.AudiobookChapter) error {
	return r.db.Create(chapter).Error
}
