package models

import "github.com/google/uuid"

type Audiobook struct {
	Base
	LibraryID uuid.UUID          `gorm:"type:varchar(36);not null;index" json:"library_id"`
	Library   Library            `gorm:"foreignKey:LibraryID" json:"-"`
	Title     string             `gorm:"not null" json:"title"`
	Author    string             `json:"author"`
	Narrator  string             `json:"narrator"`
	Description string           `json:"description"`
	Year      int                `json:"year"`
	Genre     string             `json:"genre"`
	CoverPath string             `json:"cover_path"`
	Duration  int                `json:"duration"` // seconds
	// OpenLibraryKey is the resolved Open Library work key (e.g.
	// "/works/OL45804W"). It's the stable anchor for re-enrichment — sticky
	// once set (Update only overwrites it when non-empty) so a rescan can't
	// drift to a different title-search result. Empty until first enrichment.
	OpenLibraryKey string `gorm:"index" json:"open_library_key"`
	// ISBNs is a JSON-encoded []string of the ISBNs Open Library reports for
	// the work (edition-level, often empty for audiobooks). Same JSON-in-a-
	// string convention as Genres/Paths. Also sticky in Update.
	ISBNs    string             `json:"isbns"`
	Chapters []AudiobookChapter `gorm:"foreignKey:AudiobookID" json:"chapters,omitempty"`
}

type AudiobookChapter struct {
	Base
	AudiobookID uuid.UUID `gorm:"type:varchar(36);not null;index" json:"audiobook_id"`
	Number      int       `gorm:"not null" json:"number"`
	Title       string    `json:"title"`
	Duration    int       `json:"duration"` // seconds
	FilePath    string    `gorm:"not null" json:"file_path"`
}
