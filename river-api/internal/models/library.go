package models

type LibraryType string

const (
	LibraryTypeMovie     LibraryType = "movie"
	LibraryTypeTVShow    LibraryType = "tvshow"
	LibraryTypeMusic     LibraryType = "music"
	LibraryTypeAudiobook LibraryType = "audiobook"
)

type Library struct {
	Base
	Name  string      `gorm:"not null" json:"name"`
	Type  LibraryType `gorm:"not null" json:"type"`
	Paths string      `gorm:"not null;default:'[]'" json:"paths"` // JSON-encoded []string
	// PreTranscoded marks a library whose contents are already in the
	// canonical stream format (H.264/AAC/MP4 for video, AAC/M4A for
	// audio). Scanning and metadata enrichment still run; the video /
	// audio transcoders short-circuit on events for these libraries so
	// no re-encode and no output-tree copy occurs. Existing rows default
	// to false via GORM auto-migrate.
	PreTranscoded bool `gorm:"not null;default:false" json:"pre_transcoded"`
}
