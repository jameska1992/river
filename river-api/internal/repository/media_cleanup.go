package repository

import (
	"river-api/internal/models"

	"gorm.io/gorm"
)

// MediaCleanupRepository deletes cross-reference rows that point at a
// top-level media entity by (media_type, media_id) — watchlist items,
// watch progress, collection items, subtitles, alternate audio tracks
// — plus the per-domain cast/crew tables that key directly on the
// movie/show id.
//
// Without this, deleting a movie / show / audiobook / chapter leaves
// dangling references that surface as:
//   - stale entries on the watchlist page
//   - cast cards on a person's filmography pointing at a deleted show
//   - "Continue Watching" rails serving a media id that no longer
//     resolves
//   - admin/search results showing residual collection memberships
//
// All purges run inside a single transaction so a partial failure
// rolls back rather than leaving the DB half-cleaned.
type MediaCleanupRepository interface {
	PurgeMovie(movieID string) error
	PurgeShow(showID string) error
	PurgeEpisode(episodeID string) error
	PurgeAudiobook(audiobookID string) error
	PurgeChapter(chapterID string) error
}

type mediaCleanupRepository struct{ db *gorm.DB }

func NewMediaCleanupRepository(db *gorm.DB) MediaCleanupRepository {
	return &mediaCleanupRepository{db: db}
}

// purgeMediaTyped deletes every (media_type, media_id) cross-reference
// row for one media row. Shared by the four media-type-keyed entities.
func purgeMediaTyped(tx *gorm.DB, mediaType, mediaID string) error {
	if err := tx.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Delete(&models.WatchlistItem{}).Error; err != nil {
		return err
	}
	if err := tx.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Delete(&models.WatchProgress{}).Error; err != nil {
		return err
	}
	if err := tx.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Delete(&models.CollectionItem{}).Error; err != nil {
		return err
	}
	if err := tx.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Delete(&models.Subtitle{}).Error; err != nil {
		return err
	}
	if err := tx.Where("media_type = ? AND media_id = ?", mediaType, mediaID).
		Delete(&models.AudioTrack{}).Error; err != nil {
		return err
	}
	return nil
}

func (r *mediaCleanupRepository) PurgeMovie(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := purgeMediaTyped(tx, "movie", id); err != nil {
			return err
		}
		// Cast/crew tables key directly on movie_id, not (media_type,
		// media_id). The Person rows themselves are shared across many
		// titles so they MUST NOT be touched here — only the join
		// rows go.
		if err := tx.Where("movie_id = ?", id).Delete(&models.MovieCast{}).Error; err != nil {
			return err
		}
		if err := tx.Where("movie_id = ?", id).Delete(&models.MovieCrew{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *mediaCleanupRepository) PurgeShow(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Show-level cross-refs (watchlist, collections — progress at
		// show level isn't a thing, but is purged for safety).
		if err := purgeMediaTyped(tx, "tvshow", id); err != nil {
			return err
		}
		// Cast/crew rows linking people to this show. Person rows
		// themselves are preserved.
		if err := tx.Where("tv_show_id = ?", id).Delete(&models.TVShowCast{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tv_show_id = ?", id).Delete(&models.TVShowCrew{}).Error; err != nil {
			return err
		}
		// Episode-level cross-refs for every episode under this show.
		// Subquery so we don't have to round-trip the id list through
		// Go. After the show row delete cascades the episode rows,
		// these media_id values would be dangling otherwise.
		epSubquery := tx.Model(&models.Episode{}).
			Select("id").
			Where("tv_show_id = ?", id)
		if err := tx.Where("media_type = ? AND media_id IN (?)", "episode", epSubquery).
			Delete(&models.WatchProgress{}).Error; err != nil {
			return err
		}
		if err := tx.Where("media_type = ? AND media_id IN (?)", "episode", epSubquery).
			Delete(&models.Subtitle{}).Error; err != nil {
			return err
		}
		if err := tx.Where("media_type = ? AND media_id IN (?)", "episode", epSubquery).
			Delete(&models.AudioTrack{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *mediaCleanupRepository) PurgeEpisode(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Episodes don't appear in watchlists, collections, or
		// cast/crew. Only progress + per-track refs need purging.
		if err := tx.Where("media_type = ? AND media_id = ?", "episode", id).
			Delete(&models.WatchProgress{}).Error; err != nil {
			return err
		}
		if err := tx.Where("media_type = ? AND media_id = ?", "episode", id).
			Delete(&models.Subtitle{}).Error; err != nil {
			return err
		}
		if err := tx.Where("media_type = ? AND media_id = ?", "episode", id).
			Delete(&models.AudioTrack{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *mediaCleanupRepository) PurgeAudiobook(id string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := purgeMediaTyped(tx, "audiobook", id); err != nil {
			return err
		}
		// Chapter-level progress for every chapter under this book.
		chSubquery := tx.Model(&models.AudiobookChapter{}).
			Select("id").
			Where("audiobook_id = ?", id)
		if err := tx.Where("media_type = ? AND media_id IN (?)", "chapter", chSubquery).
			Delete(&models.WatchProgress{}).Error; err != nil {
			return err
		}
		return nil
	})
}

func (r *mediaCleanupRepository) PurgeChapter(id string) error {
	return r.db.Where("media_type = ? AND media_id = ?", "chapter", id).
		Delete(&models.WatchProgress{}).Error
}
