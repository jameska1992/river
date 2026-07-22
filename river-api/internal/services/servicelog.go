package services

import (
	"time"

	"river-api/internal/models"
	"river-api/internal/repository"
)

type CreateLogInput struct {
	Level   string
	Service string
	Message string
}

type ListLogsInput struct {
	Level   string
	Service string
	From    string
	To      string
	Page    int
	Limit   int
}

type ServiceLogService struct {
	repo repository.ServiceLogRepository
}

func NewServiceLogService(repo repository.ServiceLogRepository) *ServiceLogService {
	return &ServiceLogService{repo: repo}
}

func (s *ServiceLogService) Create(input CreateLogInput) error {
	entry := &models.ServiceLog{
		Level:   input.Level,
		Service: input.Service,
		Message: input.Message,
	}
	return s.repo.Create(entry)
}

func (s *ServiceLogService) List(input ListLogsInput) ([]models.ServiceLog, int64, error) {
	offset, limit := paginationOffsetLimit(input.Page, input.Limit)
	filter := repository.ListLogsFilter{
		Level:   input.Level,
		Service: input.Service,
		Offset:  offset,
		Limit:   limit,
	}
	if input.From != "" {
		if t, err := time.Parse(time.RFC3339, input.From); err == nil {
			filter.From = t
		}
	}
	if input.To != "" {
		if t, err := time.Parse(time.RFC3339, input.To); err == nil {
			filter.To = t
		}
	}
	return s.repo.List(filter)
}
