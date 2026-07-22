package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"
)

type SubtitleService struct {
	repo repository.SubtitleRepository
}

func NewSubtitleService(repo repository.SubtitleRepository) *SubtitleService {
	return &SubtitleService{repo: repo}
}

func (s *SubtitleService) Create(input repository.SubtitleInput) (*models.Subtitle, error) {
	return s.repo.Create(input)
}

func (s *SubtitleService) ListByMedia(mediaType, mediaID string) ([]models.Subtitle, error) {
	return s.repo.ListByMedia(mediaType, mediaID)
}

func (s *SubtitleService) FindByID(id string) (*models.Subtitle, error) {
	return s.repo.FindByID(id)
}

func (s *SubtitleService) Delete(id string) error {
	return s.repo.Delete(id)
}
