package services

import (
	"river-api/internal/repository"
)

type WatchlistService struct {
	repo       repository.WatchlistRepository
	movies     repository.MovieRepository
	shows      repository.TVShowRepository
	audiobooks repository.AudiobookRepository
}

func NewWatchlistService(
	repo repository.WatchlistRepository,
	movies repository.MovieRepository,
	shows repository.TVShowRepository,
	audiobooks repository.AudiobookRepository,
) *WatchlistService {
	return &WatchlistService{repo: repo, movies: movies, shows: shows, audiobooks: audiobooks}
}

type WatchlistEntry struct {
	ID         string `json:"id"`
	MediaType  string `json:"media_type"`
	MediaID    string `json:"media_id"`
	Title      string `json:"title"`
	Year       int    `json:"year"`
	PosterPath string `json:"poster_path"`
	AddedAt    string `json:"added_at"`
}

func (s *WatchlistService) Add(userID, mediaType, mediaID string) (*WatchlistEntry, error) {
	item, err := s.repo.Add(userID, mediaType, mediaID)
	if err != nil {
		return nil, err
	}
	entry := &WatchlistEntry{
		ID:        item.ID.String(),
		MediaType: item.MediaType,
		MediaID:   item.MediaID,
		AddedAt:   item.CreatedAt.String(),
	}
	s.enrichEntry(entry)
	return entry, nil
}

func (s *WatchlistService) Remove(userID, itemID string) error {
	return s.repo.Remove(userID, itemID)
}

func (s *WatchlistService) List(userID string) ([]WatchlistEntry, error) {
	items, err := s.repo.List(userID)
	if err != nil {
		return nil, err
	}
	entries := make([]WatchlistEntry, 0, len(items))
	for _, item := range items {
		entry := WatchlistEntry{
			ID:        item.ID.String(),
			MediaType: item.MediaType,
			MediaID:   item.MediaID,
			AddedAt:   item.CreatedAt.String(),
		}
		s.enrichEntry(&entry)
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *WatchlistService) enrichEntry(e *WatchlistEntry) {
	switch e.MediaType {
	case "movie":
		if m, err := s.movies.FindByID(e.MediaID); err == nil {
			e.Title = m.Title
			e.Year = m.Year
			e.PosterPath = m.PosterPath
		}
	case "tvshow":
		if sh, err := s.shows.FindByID(e.MediaID); err == nil {
			e.Title = sh.Title
			e.Year = sh.Year
			e.PosterPath = sh.PosterPath
		}
	case "audiobook":
		if b, err := s.audiobooks.FindByID(e.MediaID); err == nil {
			e.Title = b.Title
			e.Year = b.Year
			e.PosterPath = b.CoverPath
		}
	}
}
