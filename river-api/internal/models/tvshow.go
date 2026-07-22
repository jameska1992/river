package models

import (
	"time"

	"github.com/google/uuid"
)

type TVShow struct {
	Base
	LibraryID     uuid.UUID `gorm:"type:varchar(36);not null;index" json:"library_id"`
	Library       Library   `gorm:"foreignKey:LibraryID" json:"-"`
	Title         string    `gorm:"not null" json:"title"`
	OriginalTitle string    `json:"original_title"`
	Description   string    `json:"description"`
	Year          int       `json:"year"`
	Status        string    `json:"status"` // e.g. "Ended", "Continuing"
	Genres        string    `gorm:"default:'[]'" json:"genres"` // JSON-encoded []string
	Rating        float32   `json:"rating"`
	PosterPath    string    `json:"poster_path"`
	BackdropPath  string    `json:"backdrop_path"`
	TrailerURL    string    `json:"trailer_url"`
	// TMDBID is the show's TMDB identifier, set once enrichment resolves
	// the show (whether via title search, IMDb hint, or admin override).
	// Subsequent enrichments use it directly so a rescan or refresh can't
	// re-search by title and revert to a different popular match. 0 means
	// "not yet resolved".
	TMDBID int `gorm:"index" json:"tmdb_id"`
	// FolderPath is the show's root directory on disk, recorded by
	// river-scan when it first discovers (or resolves) the show. The admin
	// "identify" flow uses this to trigger a targeted re-scan that picks
	// up episodes added after the initial scan.
	FolderPath string   `json:"folder_path"`
	Seasons    []Season `gorm:"foreignKey:TVShowID" json:"seasons,omitempty"`
}

type Season struct {
	Base
	TVShowID    uuid.UUID `gorm:"type:varchar(36);not null;index" json:"tv_show_id"`
	Number      int       `gorm:"not null" json:"number"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Year        int       `json:"year"`
	PosterPath  string    `json:"poster_path"`
	Episodes    []Episode `gorm:"foreignKey:SeasonID" json:"episodes,omitempty"`
}

type Episode struct {
	Base
	SeasonID    uuid.UUID `gorm:"type:varchar(36);not null;index" json:"season_id"`
	TVShowID    uuid.UUID `gorm:"type:varchar(36);not null;index" json:"tv_show_id"`
	Number      int       `gorm:"not null" json:"number"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Runtime     int       `json:"runtime"` // minutes
	FilePath    string    `gorm:"not null" json:"file_path"`
	// SourcePath is the original on-disk location of this episode before
	// transcoding/copying. Stream/download fall back to it when FilePath
	// isn't yet pointing at a real file (transcode hasn't finished). See
	// the matching field on Movie for the full rationale.
	SourcePath string    `json:"source_path"`
	AiredAt    time.Time `json:"aired_at"`
	// IsSpecial flags episodes whose filename didn't yield a SxxExx / NxNN /
	// Exx number. We still ingest them so they surface in the season's
	// episode list; clients render "SPEC" instead of "E${number}". Number is
	// a 1-based index among specials in the same season, so it can collide
	// with a regular ep's number without ambiguity — (season_id, number,
	// is_special) is what's effectively unique.
	IsSpecial bool `gorm:"not null;default:false" json:"is_special"`
}
