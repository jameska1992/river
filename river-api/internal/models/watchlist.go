package models

type WatchlistItem struct {
	Base
	UserID    string `gorm:"type:varchar(36);not null;uniqueIndex:idx_watchlist_user_media"`
	MediaType string `gorm:"not null;uniqueIndex:idx_watchlist_user_media"` // "movie" | "tvshow" | "audiobook"
	MediaID   string `gorm:"type:varchar(36);not null;uniqueIndex:idx_watchlist_user_media"`
}
