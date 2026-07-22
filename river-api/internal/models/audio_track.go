package models

type AudioTrack struct {
	Base
	MediaType   string `gorm:"not null;index:idx_audio_track_media" json:"media_type"` // "movie" or "episode"
	MediaID     string `gorm:"type:varchar(36);not null;index:idx_audio_track_media" json:"media_id"`
	Language    string `gorm:"not null" json:"language"` // BCP-47 tag e.g. "en"
	Label       string `json:"label"`                    // human-readable e.g. "English"
	StreamIndex int    `json:"stream_index"`             // 0-based audio stream index in the video file
	FilePath    string `gorm:"not null" json:"file_path"` // variant MP4 (video + this audio only)
}
