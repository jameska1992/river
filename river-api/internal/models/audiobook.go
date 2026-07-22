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
	Chapters  []AudiobookChapter `gorm:"foreignKey:AudiobookID" json:"chapters,omitempty"`
}

type AudiobookChapter struct {
	Base
	AudiobookID uuid.UUID `gorm:"type:varchar(36);not null;index" json:"audiobook_id"`
	Number      int       `gorm:"not null" json:"number"`
	Title       string    `json:"title"`
	Duration    int       `json:"duration"` // seconds
	FilePath    string    `gorm:"not null" json:"file_path"`
}
