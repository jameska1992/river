package repository

import (
	"time"

	"river-api/internal/models"

	"gorm.io/gorm"
)

type ListLogsFilter struct {
	Level   string
	Service string
	From    time.Time
	To      time.Time
	Offset  int
	Limit   int
}

type ServiceLogRepository interface {
	Create(entry *models.ServiceLog) error
	List(filter ListLogsFilter) ([]models.ServiceLog, int64, error)
}

type serviceLogRepository struct{ db *gorm.DB }

func NewServiceLogRepository(db *gorm.DB) ServiceLogRepository {
	return &serviceLogRepository{db}
}

func (r *serviceLogRepository) Create(entry *models.ServiceLog) error {
	return r.db.Create(entry).Error
}

func (r *serviceLogRepository) List(f ListLogsFilter) ([]models.ServiceLog, int64, error) {
	q := r.db.Model(&models.ServiceLog{})
	if f.Level != "" {
		q = q.Where("level = ?", f.Level)
	}
	if f.Service != "" {
		q = q.Where("service = ?", f.Service)
	}
	if !f.From.IsZero() {
		q = q.Where("created_at >= ?", f.From)
	}
	if !f.To.IsZero() {
		q = q.Where("created_at <= ?", f.To)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var entries []models.ServiceLog
	if err := q.Order("created_at DESC").Offset(f.Offset).Limit(f.Limit).Find(&entries).Error; err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}
