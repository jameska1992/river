package models

import "github.com/google/uuid"

type WatchParty struct {
	Base
	HostID    uuid.UUID `gorm:"not null" json:"host_id"`
	MediaType string    `gorm:"not null" json:"media_type"` // "movie" | "episode"
	MediaID   string    `gorm:"not null" json:"media_id"`
	ShowID    string    `json:"show_id"`
	SeasonID  string    `json:"season_id"`
}
