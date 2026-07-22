// Package services — Collections are intentionally global. Any
// authenticated user can list / view / edit / delete any collection. The
// Collection.UserID column on the model is preserved as an audit field
// ("who created this") but is not used for access control.
package services

import (
	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"
)

type CollectionService struct {
	repo       repository.CollectionRepository
	movies     repository.MovieRepository
	shows      repository.TVShowRepository
	audiobooks repository.AudiobookRepository
}

func NewCollectionService(
	repo repository.CollectionRepository,
	movies repository.MovieRepository,
	shows repository.TVShowRepository,
	audiobooks repository.AudiobookRepository,
) *CollectionService {
	return &CollectionService{repo: repo, movies: movies, shows: shows, audiobooks: audiobooks}
}

type CollectionItemDetail struct {
	models.CollectionItem
	Title      string `json:"title"`
	PosterPath string `json:"poster_path"`
	Year       int    `json:"year,omitempty"`
}

type CollectionDetail struct {
	models.Collection
	Items []CollectionItemDetail `json:"items"`
}

type CollectionSummary struct {
	models.Collection
	ItemCount int      `json:"item_count"`
	Covers    []string `json:"covers"`
}

func (s *CollectionService) List() ([]CollectionSummary, error) {
	cols, err := s.repo.FindAll()
	if err != nil {
		return nil, err
	}
	summaries := make([]CollectionSummary, 0, len(cols))
	for _, col := range cols {
		items, _ := s.repo.FindItems(col.ID.String())
		covers := make([]string, 0, 4)
		for _, item := range items {
			if len(covers) >= 4 {
				break
			}
			switch item.MediaType {
			case "movie":
				if m, err := s.movies.FindByID(item.MediaID); err == nil && m.PosterPath != "" {
					covers = append(covers, m.PosterPath)
				}
			case "tvshow":
				if show, err := s.shows.FindByID(item.MediaID); err == nil && show.PosterPath != "" {
					covers = append(covers, show.PosterPath)
				}
			case "audiobook":
				// Audiobooks use CoverPath rather than PosterPath; the
				// CollectionItemDetail / summary layer is media-type-agnostic
				// (it just renders an image URL), so we surface whichever
				// the source model calls "the cover".
				if b, err := s.audiobooks.FindByID(item.MediaID); err == nil && b.CoverPath != "" {
					covers = append(covers, b.CoverPath)
				}
			}
		}
		summaries = append(summaries, CollectionSummary{
			Collection: col,
			ItemCount:  len(items),
			Covers:     covers,
		})
	}
	return summaries, nil
}

// Create stamps the caller's user id as the audit "created by" — it does
// not gate any later operation. Anyone authenticated can mutate the
// collection afterwards.
func (s *CollectionService) Create(createdByUserID, name, description string) (*models.Collection, error) {
	c := &models.Collection{UserID: createdByUserID, Name: name, Description: description}
	return c, s.repo.Create(c)
}

func (s *CollectionService) GetWithItems(id string) (*CollectionDetail, error) {
	col, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	rawItems, err := s.repo.FindItems(id)
	if err != nil {
		return nil, err
	}
	items := make([]CollectionItemDetail, 0, len(rawItems))
	for _, item := range rawItems {
		detail := CollectionItemDetail{CollectionItem: item}
		switch item.MediaType {
		case "movie":
			if m, err := s.movies.FindByID(item.MediaID); err == nil {
				detail.Title = m.Title
				detail.PosterPath = m.PosterPath
				detail.Year = m.Year
			}
		case "tvshow":
			if show, err := s.shows.FindByID(item.MediaID); err == nil {
				detail.Title = show.Title
				detail.PosterPath = show.PosterPath
				detail.Year = show.Year
			}
		case "audiobook":
			if b, err := s.audiobooks.FindByID(item.MediaID); err == nil {
				detail.Title = b.Title
				detail.PosterPath = b.CoverPath
				detail.Year = b.Year
			}
		}
		items = append(items, detail)
	}
	return &CollectionDetail{Collection: *col, Items: items}, nil
}

func (s *CollectionService) Update(id, name, description string) (*models.Collection, error) {
	col, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	col.Name = name
	col.Description = description
	return col, s.repo.Save(col)
}

func (s *CollectionService) Delete(id string) error {
	if _, err := s.repo.FindByID(id); err != nil {
		return err
	}
	return s.repo.Delete(id)
}

func (s *CollectionService) AddItem(collectionID, mediaType, mediaID string) (*CollectionItemDetail, error) {
	if _, err := s.repo.FindByID(collectionID); err != nil {
		return nil, err
	}

	// Verify media exists and get details
	detail := CollectionItemDetail{}
	switch mediaType {
	case "movie":
		m, err := s.movies.FindByID(mediaID)
		if err != nil {
			return nil, apperrors.ErrNotFound
		}
		detail.Title = m.Title
		detail.PosterPath = m.PosterPath
		detail.Year = m.Year
	case "tvshow":
		show, err := s.shows.FindByID(mediaID)
		if err != nil {
			return nil, apperrors.ErrNotFound
		}
		detail.Title = show.Title
		detail.PosterPath = show.PosterPath
		detail.Year = show.Year
	case "audiobook":
		b, err := s.audiobooks.FindByID(mediaID)
		if err != nil {
			return nil, apperrors.ErrNotFound
		}
		detail.Title = b.Title
		detail.PosterPath = b.CoverPath
		detail.Year = b.Year
	default:
		return nil, apperrors.ErrNotFound
	}

	// Check for duplicate
	if _, err := s.repo.FindItemByMedia(collectionID, mediaType, mediaID); err == nil {
		return nil, apperrors.ErrConflict
	}

	item := &models.CollectionItem{
		CollectionID: collectionID,
		MediaType:    mediaType,
		MediaID:      mediaID,
	}
	if err := s.repo.AddItem(item); err != nil {
		return nil, err
	}
	detail.CollectionItem = *item
	return &detail, nil
}

func (s *CollectionService) RemoveItem(collectionID, itemID string) error {
	if _, err := s.repo.FindByID(collectionID); err != nil {
		return err
	}
	item, err := s.repo.FindItem(itemID)
	if err != nil {
		return err
	}
	if item.CollectionID != collectionID {
		return apperrors.ErrNotFound
	}
	return s.repo.RemoveItem(itemID)
}
