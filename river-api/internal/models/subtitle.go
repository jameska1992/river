package models

type Subtitle struct {
	Base
	MediaType string `gorm:"not null;index:idx_subtitle_media" json:"media_type"` // "movie" or "episode"
	MediaID   string `gorm:"type:varchar(36);not null;index:idx_subtitle_media" json:"media_id"`
	Language  string `gorm:"not null" json:"language"` // BCP-47 tag e.g. "en"
	Label     string `json:"label"`                    // human-readable e.g. "English"
	FilePath  string `gorm:"not null" json:"file_path"`
}
