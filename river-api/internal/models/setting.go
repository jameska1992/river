package models

import "time"

// Setting is a single namespaced configuration value stored in the
// database, e.g. "radarr.url" -> "http://radarr:7878". The key is the
// primary key, so any future config area (integrations, scanning, …)
// can be added without a schema change. Secret values (API keys) are
// stored as-is; the API masks them on read rather than encrypting at
// rest.
type Setting struct {
	Key       string    `gorm:"primaryKey;type:varchar(191)" json:"key"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}
