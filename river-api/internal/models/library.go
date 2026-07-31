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
	Name string      `gorm:"not null" json:"name"`
	Type LibraryType `gorm:"not null" json:"type"`
	// Paths is a JSON-encoded array of path configs, each
	// {"path":"…","pre_transcoded":bool}. The API stores and returns it
	// verbatim; river-scan parses it (and still accepts the legacy bare-
	// string form). The per-path pre_transcoded flag marks a path whose
	// contents are already in the canonical stream format (H.264/AAC/MP4
	// for video, AAC/M4A for audio): scanning and metadata enrichment
	// still run, but the video / audio transcoders short-circuit on its
	// events so no re-encode and no output-tree copy occurs.
	Paths string `gorm:"not null;default:'[]'" json:"paths"`
	// PreTranscoded is the legacy library-wide flag, superseded by the
	// per-path flag in Paths. It's retained only so libraries saved before
	// per-path flags existed keep short-circuiting: river-scan OR's it into
	// every path's effective flag until an admin re-saves the library.
	// Existing rows default to false via GORM auto-migrate.
	PreTranscoded bool `gorm:"not null;default:false" json:"pre_transcoded"`
}
