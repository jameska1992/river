package models

type WatchProgress struct {
	Base
	UserID    string  `gorm:"type:varchar(36);not null;uniqueIndex:idx_watch_progress_user_media" json:"user_id"`
	MediaType string  `gorm:"not null;uniqueIndex:idx_watch_progress_user_media" json:"media_type"` // "movie" | "episode"
	MediaID   string  `gorm:"type:varchar(36);not null;uniqueIndex:idx_watch_progress_user_media" json:"media_id"`
	Position  float64 `json:"position"`  // seconds
	Duration  float64 `json:"duration"`  // seconds
	Completed bool    `json:"completed"` // position/duration >= 0.9
}
