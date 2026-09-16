package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type ListFailedJobsFilter struct {
	Service   string
	MediaType string
	Offset    int
	Limit     int
}

type FailedJobRepository interface {
	Create(job *models.FailedJob) error
	List(filter ListFailedJobsFilter) ([]models.FailedJob, int64, error)
	FindByID(id string) (*models.FailedJob, error)
	Delete(id string) error
}

type failedJobRepository struct{ db *gorm.DB }

func NewFailedJobRepository(db *gorm.DB) FailedJobRepository {
	return &failedJobRepository{db}
}

func (r *failedJobRepository) Create(job *models.FailedJob) error {
	return r.db.Create(job).Error
}

func (r *failedJobRepository) List(f ListFailedJobsFilter) ([]models.FailedJob, int64, error) {
	q := r.db.Model(&models.FailedJob{})
	if f.Service != "" {
		q = q.Where("service = ?", f.Service)
	}
	if f.MediaType != "" {
		q = q.Where("media_type = ?", f.MediaType)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var jobs []models.FailedJob
	if err := q.Order("created_at DESC").Offset(f.Offset).Limit(f.Limit).Find(&jobs).Error; err != nil {
		return nil, 0, err
	}
	return jobs, total, nil
}

func (r *failedJobRepository) FindByID(id string) (*models.FailedJob, error) {
	var job models.FailedJob
	if err := r.db.First(&job, "id = ?", id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &job, nil
}

func (r *failedJobRepository) Delete(id string) error {
	result := r.db.Delete(&models.FailedJob{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}
