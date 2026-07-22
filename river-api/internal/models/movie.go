package models

import "github.com/google/uuid"

type Movie struct {
	Base
	LibraryID     uuid.UUID `gorm:"type:varchar(36);not null;index" json:"library_id"`
	Library       Library   `gorm:"foreignKey:LibraryID" json:"-"`
	Title         string    `gorm:"not null" json:"title"`
	OriginalTitle string    `json:"original_title"`
	Description   string    `json:"description"`
	Year          int       `json:"year"`
	Genres        string    `gorm:"default:'[]'" json:"genres"`  // JSON-encoded []string
	Rating        float32   `json:"rating"`
	Runtime       int       `json:"runtime"` // minutes
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path"`
	TrailerURL    string    `json:"trailer_url"`
	// TMDBID is the movie's TMDB identifier, set once enrichment resolves
	// the movie (whether via title search, IMDb hint, or admin override).
	// Subsequent enrichments use it directly so a rescan or refresh can't
	// re-search by title and revert to a different popular match. 0 means
	// "not yet resolved".
	TMDBID   int    `gorm:"index" json:"tmdb_id"`
	FilePath string `json:"file_path"`
	// SourcePath is the original on-disk location the scanner discovered
	// the movie at (typically under MEDIA_PATH). FilePath is the canonical
	// post-transcode/post-copy location (typically under OUTPUT_PATH).
	// Stream/download endpoints prefer FilePath but fall back to SourcePath
	// when FilePath is empty or the file isn't on disk yet — this lets the
	// UI offer playback before the transcode/copy step has finished.
	SourcePath string `json:"source_path"`
}
