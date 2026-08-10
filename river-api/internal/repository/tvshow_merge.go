package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

// TVShowMergeRepository folds one TV show record into another and records the
// absorbed folder root as an alias of the survivor. The write spans several
// tables and runs in a single transaction so a merge is all-or-nothing.
type TVShowMergeRepository interface {
	// Merge folds mergedID into survivorID: re-parents the merged show's
	// seasons/episodes onto the survivor (moving episodes into a same-numbered
	// survivor season when one exists, else moving the whole season), repoints
	// references (watchlist, collections, watch parties), drops the merged
	// show's cast/crew, records `alias` as an extra root of the survivor, and
	// soft-deletes the merged show. The caller must have verified there are no
	// episode-number collisions within shared seasons.
	Merge(survivorID, mergedID string, alias *models.TVShowPath) error
	// FindShowIDByPath returns the id of the show whose alias path equals
	// folderPath within libraryID, or ErrNotFound. Lets the scanner route a
	// merged-away directory to its surviving show.
	FindShowIDByPath(libraryID, folderPath string) (string, error)
	// FindSurvivorID returns the id of the show that absorbed mergedID via a
	// recorded merge alias, or ErrNotFound when mergedID was never merged away.
	// Lets an episode write that still carries a merged-away show id — e.g. a
	// transcode that finished after an admin merged the show — redirect onto
	// the survivor instead of failing.
	FindSurvivorID(mergedID string) (string, error)
}

type tvShowMergeRepository struct{ db *gorm.DB }

func NewTVShowMergeRepository(db *gorm.DB) TVShowMergeRepository {
	return &tvShowMergeRepository{db}
}

func (r *tvShowMergeRepository) FindShowIDByPath(libraryID, folderPath string) (string, error) {
	var p models.TVShowPath
	err := r.db.
		Where("library_id = ? AND folder_path = ?", libraryID, folderPath).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", apperrors.ErrNotFound
		}
		return "", err
	}
	return p.TVShowID.String(), nil
}

func (r *tvShowMergeRepository) FindSurvivorID(mergedID string) (string, error) {
	var p models.TVShowPath
	err := r.db.
		Where("merged_from_show_id = ?", mergedID).
		First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", apperrors.ErrNotFound
		}
		return "", err
	}
	return p.TVShowID.String(), nil
}

func (r *tvShowMergeRepository) Merge(survivorID, mergedID string, alias *models.TVShowPath) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Re-parent seasons/episodes. A merged season whose number already
		// exists on the survivor has its episodes moved into that survivor
		// season (then the emptied merged season is removed); a season the
		// survivor lacks is moved wholesale.
		var survivorSeasons, mergedSeasons []models.Season
		if err := tx.Where("tv_show_id = ?", survivorID).Find(&survivorSeasons).Error; err != nil {
			return err
		}
		if err := tx.Where("tv_show_id = ?", mergedID).Find(&mergedSeasons).Error; err != nil {
			return err
		}
		survivorSeasonByNum := make(map[int]models.Season, len(survivorSeasons))
		for _, s := range survivorSeasons {
			survivorSeasonByNum[s.Number] = s
		}
		for _, ms := range mergedSeasons {
			if ss, ok := survivorSeasonByNum[ms.Number]; ok {
				if err := tx.Model(&models.Episode{}).
					Where("season_id = ?", ms.ID).
					Updates(map[string]any{"season_id": ss.ID, "tv_show_id": survivorID}).Error; err != nil {
					return err
				}
				if err := tx.Delete(&models.Season{}, "id = ?", ms.ID).Error; err != nil {
					return err
				}
			} else {
				if err := tx.Model(&models.Season{}).
					Where("id = ?", ms.ID).
					Update("tv_show_id", survivorID).Error; err != nil {
					return err
				}
				if err := tx.Model(&models.Episode{}).
					Where("season_id = ?", ms.ID).
					Update("tv_show_id", survivorID).Error; err != nil {
					return err
				}
			}
		}

		// The survivor already carries its own cast/crew from the same TMDB
		// match, so drop the merged show's rather than merge duplicates.
		if err := tx.Delete(&models.TVShowCast{}, "tv_show_id = ?", mergedID).Error; err != nil {
			return err
		}
		if err := tx.Delete(&models.TVShowCrew{}, "tv_show_id = ?", mergedID).Error; err != nil {
			return err
		}

		// Repoint show-id references. Watchlist and collection rows carry a
		// unique (user|collection, media_type, media_id) index, so first drop
		// any merged-show row that would collide with an existing survivor row,
		// then repoint the rest.
		if err := tx.Exec(
			`DELETE FROM watchlist_items WHERE media_type = 'tvshow' AND media_id = ? AND EXISTS (
				SELECT 1 FROM watchlist_items o
				WHERE o.user_id = watchlist_items.user_id AND o.media_type = 'tvshow' AND o.media_id = ?)`,
			mergedID, survivorID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.WatchlistItem{}).
			Where("media_type = 'tvshow' AND media_id = ?", mergedID).
			Update("media_id", survivorID).Error; err != nil {
			return err
		}
		if err := tx.Exec(
			`DELETE FROM collection_items WHERE media_type = 'tvshow' AND media_id = ? AND EXISTS (
				SELECT 1 FROM collection_items o
				WHERE o.collection_id = collection_items.collection_id AND o.media_type = 'tvshow' AND o.media_id = ?)`,
			mergedID, survivorID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.CollectionItem{}).
			Where("media_type = 'tvshow' AND media_id = ?", mergedID).
			Update("media_id", survivorID).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.WatchParty{}).
			Where("show_id = ?", mergedID).
			Update("show_id", survivorID).Error; err != nil {
			return err
		}

		// Preserve the absorbed root so future scans resolve it to the survivor.
		if alias != nil && alias.FolderPath != "" {
			if err := tx.Create(alias).Error; err != nil {
				return err
			}
		}

		// Soft-delete the merged show (Base carries gorm.DeletedAt).
		return tx.Delete(&models.TVShow{}, "id = ?", mergedID).Error
	})
}
