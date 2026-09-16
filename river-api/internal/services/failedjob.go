package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"
)

type ReportFailedJobInput struct {
	Service    string
	MediaType  string
	SourcePath string
	Reason     string
	Attempts   int
	RoutingKey string
	Event      string
}

type ListFailedJobsInput struct {
	Service   string
	MediaType string
	Page      int
	Limit     int
}

type FailedJobService struct {
	repo repository.FailedJobRepository
}

func NewFailedJobService(repo repository.FailedJobRepository) *FailedJobService {
	return &FailedJobService{repo: repo}
}

func (s *FailedJobService) Report(in ReportFailedJobInput) (*models.FailedJob, error) {
	job := &models.FailedJob{
		Service:    in.Service,
		MediaType:  in.MediaType,
		SourcePath: in.SourcePath,
		Reason:     in.Reason,
		Attempts:   in.Attempts,
		RoutingKey: in.RoutingKey,
		Event:      in.Event,
	}
	if err := s.repo.Create(job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *FailedJobService) List(in ListFailedJobsInput) ([]models.FailedJob, int64, error) {
	offset, limit := paginationOffsetLimit(in.Page, in.Limit)
	return s.repo.List(repository.ListFailedJobsFilter{
		Service:   in.Service,
		MediaType: in.MediaType,
		Offset:    offset,
		Limit:     limit,
	})
}

func (s *FailedJobService) Get(id string) (*models.FailedJob, error) {
	return s.repo.FindByID(id)
}

func (s *FailedJobService) Dismiss(id string) error {
	return s.repo.Delete(id)
}
