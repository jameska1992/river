package models

// DismissedNextUp records that a user has explicitly hidden a specific
// episode from their "Next Up" home rail. The presence of a row is
// enough — no state beyond (user_id, episode_id) is meaningful.
//
// One row per (user, episode). If the user later watches the episode
// through the normal player flow the dismissal becomes moot (Next Up
// derives from watch progress, not from this table). If they never do,
// the show simply stops surfacing in Next Up until they resume it and
// finish another episode past the dismissed one.
type DismissedNextUp struct {
	Base
	UserID    string `gorm:"type:varchar(36);not null;uniqueIndex:idx_dismissed_next_up_user_episode" json:"user_id"`
	EpisodeID string `gorm:"type:varchar(36);not null;uniqueIndex:idx_dismissed_next_up_user_episode" json:"episode_id"`
}
