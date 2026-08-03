package models

import "github.com/google/uuid"

// TVShowPath records an *additional* on-disk root directory that maps to a TV
// show, beyond the show's own TVShow.FolderPath. Rows are created when two show
// records are merged: the absorbed show's folder root is preserved here and
// pointed at the surviving show, so a later scan of that path resolves to the
// survivor instead of re-creating the duplicate.
//
// FolderPath is unique across all shows — a directory root belongs to exactly
// one show. LibraryID is retained because merged roots can live in a different
// library than the survivor. MergedFromShowID is audit-only: the id of the show
// that was folded in when this alias was recorded.
type TVShowPath struct {
	Base
	TVShowID         uuid.UUID `gorm:"type:varchar(36);not null;index" json:"tv_show_id"`
	FolderPath       string    `gorm:"not null;uniqueIndex" json:"folder_path"`
	LibraryID        uuid.UUID `gorm:"type:varchar(36);not null;index" json:"library_id"`
	MergedFromShowID string    `gorm:"type:varchar(36)" json:"merged_from_show_id"`
}
