package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"
)

type AudioTrackService struct {
	repo repository.AudioTrackRepository
}

func NewAudioTrackService(repo repository.AudioTrackRepository) *AudioTrackService {
	return &AudioTrackService{repo: repo}
}

func (s *AudioTrackService) Create(input repository.AudioTrackInput) (*models.AudioTrack, error) {
	return s.repo.Create(input)
}

func (s *AudioTrackService) ListByMedia(mediaType, mediaID string) ([]models.AudioTrack, error) {
	return s.repo.ListByMedia(mediaType, mediaID)
}

func (s *AudioTrackService) FindByID(id string) (*models.AudioTrack, error) {
	return s.repo.FindByID(id)
}

func (s *AudioTrackService) Delete(id string) error {
	return s.repo.Delete(id)
}
