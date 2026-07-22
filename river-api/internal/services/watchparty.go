package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

type WatchPartyInput struct {
	HostID    string
	MediaType string
	MediaID   string
	ShowID    string
	SeasonID  string
}

type WatchPartyService struct {
	repo repository.WatchPartyRepository
}

func NewWatchPartyService(repo repository.WatchPartyRepository) *WatchPartyService {
	return &WatchPartyService{repo: repo}
}

func (s *WatchPartyService) Create(input WatchPartyInput) (*models.WatchParty, error) {
	hostID, err := uuid.Parse(input.HostID)
	if err != nil {
		return nil, ErrUnauthorized
	}
	party := &models.WatchParty{
		HostID:    hostID,
		MediaType: input.MediaType,
		MediaID:   input.MediaID,
		ShowID:    input.ShowID,
		SeasonID:  input.SeasonID,
	}
	if err := s.repo.Create(party); err != nil {
		return nil, err
	}
	return party, nil
}

func (s *WatchPartyService) GetByID(id string) (*models.WatchParty, error) {
	return s.repo.FindByID(id)
}

func (s *WatchPartyService) Delete(hostID, id string) error {
	party, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if party.HostID.String() != hostID {
		return ErrUnauthorized
	}
	return s.repo.Delete(id)
}
