package services

import (
	"river-api/internal/models"
	"river-api/internal/repository"
)

type LibraryService struct {
	repo repository.LibraryRepository
}

func NewLibraryService(repo repository.LibraryRepository) *LibraryService {
	return &LibraryService{repo: repo}
}

type LibraryInput struct {
	Name          string
	Type          models.LibraryType
	Paths         string
	PreTranscoded bool
}

func (s *LibraryService) List() ([]models.Library, error) {
	return s.repo.FindAll()
}

func (s *LibraryService) Create(input LibraryInput) (*models.Library, error) {
	paths := input.Paths
	if paths == "" {
		paths = "[]"
	}
	lib := models.Library{
		Name:          input.Name,
		Type:          input.Type,
		Paths:         paths,
		PreTranscoded: input.PreTranscoded,
	}
	return &lib, s.repo.Create(&lib)
}

func (s *LibraryService) GetByID(id string) (*models.Library, error) {
	return s.repo.FindByID(id)
}

func (s *LibraryService) Update(id string, input LibraryInput) (*models.Library, error) {
	lib, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	lib.Name = input.Name
	lib.Type = input.Type
	if input.Paths != "" {
		lib.Paths = input.Paths
	}
	lib.PreTranscoded = input.PreTranscoded
	return lib, s.repo.Save(lib)
}

func (s *LibraryService) Delete(id string) error {
	return s.repo.Delete(id)
}
