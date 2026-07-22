package repository

import (
	"errors"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"gorm.io/gorm"
)

type DismissedNextUpRepository interface {
	Create(userID, episodeID string) error
	Delete(userID, episodeID string) error
	ListEpisodeIDs(userID string) ([]string, error)
}

type dismissedNextUpRepository struct{ db *gorm.DB }

func NewDismissedNextUpRepository(db *gorm.DB) DismissedNextUpRepository {
	return &dismissedNextUpRepository{db: db}
}

// Create inserts a dismissal row. Idempotent — a duplicate for the same
// (user, episode) is silently absorbed so the caller doesn't have to
// pre-check with a SELECT.
func (r *dismissedNextUpRepository) Create(userID, episodeID string) error {
	d := &models.DismissedNextUp{UserID: userID, EpisodeID: episodeID}
	if err := r.db.Create(d).Error; err != nil {
		// A unique-index violation just means someone already dismissed
		// this episode. Treat as success — the row is already there.
		if isUniqueViolation(err) {
			return nil
		}
		return err
	}
	return nil
}

func (r *dismissedNextUpRepository) Delete(userID, episodeID string) error {
	res := r.db.
		Where("user_id = ? AND episode_id = ?", userID, episodeID).
		Delete(&models.DismissedNextUp{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return apperrors.ErrNotFound
	}
	return nil
}

// ListEpisodeIDs returns every dismissed episode ID for a user, in no
// particular order. Callers use it as a set for filtering — avoids
// running the "was this dismissed?" check per episode.
func (r *dismissedNextUpRepository) ListEpisodeIDs(userID string) ([]string, error) {
	var ids []string
	err := r.db.Model(&models.DismissedNextUp{}).
		Where("user_id = ?", userID).
		Pluck("episode_id", &ids).Error
	return ids, err
}

// isUniqueViolation detects Postgres and SQLite unique-constraint
// violations so the caller can treat Create as idempotent. GORM v2
// wraps the underlying driver error; text matching against the two
// error shapes is the pragmatic check.
func isUniqueViolation(err error) bool {
	var target = err.Error()
	return target != "" && (contains(target, "duplicate key") ||
		contains(target, "UNIQUE constraint failed") ||
		errors.Is(err, gorm.ErrDuplicatedKey))
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
