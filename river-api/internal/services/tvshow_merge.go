package services

import (
	"errors"

	"river-api/internal/models"
	"river-api/internal/repository"
)

// ShowMergeService folds two duplicate TV show records into one. It's a manual,
// admin-driven action: the caller supplies two show ids and the older row (by
// created_at) always survives, absorbing the other's seasons and episodes.
type ShowMergeService struct {
	shows    repository.TVShowRepository
	seasons  repository.SeasonRepository
	episodes repository.EpisodeRepository
	merge    repository.TVShowMergeRepository
}

func NewShowMergeService(
	shows repository.TVShowRepository,
	seasons repository.SeasonRepository,
	episodes repository.EpisodeRepository,
	merge repository.TVShowMergeRepository,
) *ShowMergeService {
	return &ShowMergeService{shows: shows, seasons: seasons, episodes: episodes, merge: merge}
}

// MergeConflict is an episode on the merged show that would collide with an
// existing episode on the survivor — same season number, same episode number,
// same is_special. Its presence blocks the merge for manual resolution.
type MergeConflict struct {
	SeasonNumber  int    `json:"season_number"`
	EpisodeNumber int    `json:"episode_number"`
	IsSpecial     bool   `json:"is_special"`
	Title         string `json:"title"`
}

// MergePreview is a dry run: which show survives, how much moves, and any
// blocking conflicts. CanMerge is true only when there are no conflicts.
type MergePreview struct {
	Survivor      *models.TVShow  `json:"survivor"`
	Merged        *models.TVShow  `json:"merged"`
	SeasonsMoved  int             `json:"seasons_moved"`
	EpisodesMoved int             `json:"episodes_moved"`
	Conflicts     []MergeConflict `json:"conflicts"`
	CanMerge      bool            `json:"can_merge"`
}

// resolve loads both shows and orients them so the survivor is the older row.
func (s *ShowMergeService) resolve(idA, idB string) (survivor, merged *models.TVShow, err error) {
	if idA == idB {
		return nil, nil, ErrConflict
	}
	a, err := s.shows.FindByID(idA)
	if err != nil {
		return nil, nil, err
	}
	b, err := s.shows.FindByID(idB)
	if err != nil {
		return nil, nil, err
	}
	if b.CreatedAt.Before(a.CreatedAt) {
		return b, a, nil
	}
	return a, b, nil
}

func (s *ShowMergeService) buildPreview(survivor, merged *models.TVShow) (*MergePreview, error) {
	survivorSeasons, err := s.seasons.FindByShowID(survivor.ID.String())
	if err != nil {
		return nil, err
	}
	survivorByNum := make(map[int]models.Season, len(survivorSeasons))
	for _, ss := range survivorSeasons {
		survivorByNum[ss.Number] = ss
	}

	mergedSeasons, err := s.seasons.FindByShowID(merged.ID.String())
	if err != nil {
		return nil, err
	}

	prev := &MergePreview{Survivor: survivor, Merged: merged, Conflicts: []MergeConflict{}}
	prev.SeasonsMoved = len(mergedSeasons)
	for _, ms := range mergedSeasons {
		eps, err := s.episodes.FindBySeasonID(ms.ID.String())
		if err != nil {
			return nil, err
		}
		prev.EpisodesMoved += len(eps)

		ss, shared := survivorByNum[ms.Number]
		if !shared {
			continue // whole season moves; nothing to collide with
		}
		for _, ep := range eps {
			_, err := s.episodes.FindBySeasonAndNumber(ss.ID.String(), ep.Number, ep.IsSpecial)
			switch {
			case err == nil:
				prev.Conflicts = append(prev.Conflicts, MergeConflict{
					SeasonNumber:  ms.Number,
					EpisodeNumber: ep.Number,
					IsSpecial:     ep.IsSpecial,
					Title:         ep.Title,
				})
			case errors.Is(err, ErrNotFound):
				// no collision for this episode
			default:
				return nil, err
			}
		}
	}
	prev.CanMerge = len(prev.Conflicts) == 0
	return prev, nil
}

// ResolveShowByPath returns the id of the show that owns folderPath within
// libraryID via a recorded merge alias, or "" when there's no such mapping.
// river-scan calls this so episodes discovered under an absorbed root attach to
// the surviving show instead of re-creating the duplicate. "no mapping" is a
// normal result (empty id, nil error), not an error, so callers don't have to
// treat a miss as a failure.
func (s *ShowMergeService) ResolveShowByPath(libraryID, folderPath string) (string, error) {
	id, err := s.merge.FindShowIDByPath(libraryID, folderPath)
	if errors.Is(err, ErrNotFound) {
		return "", nil
	}
	return id, err
}

// PreviewMerge reports what merging the two shows would do without changing
// anything.
func (s *ShowMergeService) PreviewMerge(idA, idB string) (*MergePreview, error) {
	survivor, merged, err := s.resolve(idA, idB)
	if err != nil {
		return nil, err
	}
	return s.buildPreview(survivor, merged)
}

// Merge folds the newer show into the older one and returns the survivor. It
// refuses (ErrConflict) when the preview finds colliding episodes.
func (s *ShowMergeService) Merge(idA, idB string) (*models.TVShow, error) {
	survivor, merged, err := s.resolve(idA, idB)
	if err != nil {
		return nil, err
	}
	prev, err := s.buildPreview(survivor, merged)
	if err != nil {
		return nil, err
	}
	if !prev.CanMerge {
		return nil, ErrConflict
	}

	var alias *models.TVShowPath
	if merged.FolderPath != "" {
		alias = &models.TVShowPath{
			TVShowID:         survivor.ID,
			FolderPath:       merged.FolderPath,
			LibraryID:        merged.LibraryID,
			MergedFromShowID: merged.ID.String(),
		}
	}
	if err := s.merge.Merge(survivor.ID.String(), merged.ID.String(), alias); err != nil {
		return nil, err
	}
	return s.shows.FindByID(survivor.ID.String())
}
