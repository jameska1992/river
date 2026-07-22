package models

import "github.com/google/uuid"

type Artist struct {
	Base
	LibraryID uuid.UUID `gorm:"type:varchar(36);not null;index" json:"library_id"`
	Library   Library   `gorm:"foreignKey:LibraryID" json:"-"`
	Name      string    `gorm:"not null" json:"name"`
	Bio       string    `json:"bio"`
	ImagePath string    `json:"image_path"`
	Albums    []Album   `gorm:"foreignKey:ArtistID" json:"albums,omitempty"`
}

type Album struct {
	Base
	LibraryID  uuid.UUID `gorm:"type:varchar(36);not null;index" json:"library_id"`
	Library    Library   `gorm:"foreignKey:LibraryID" json:"-"`
	ArtistID   uuid.UUID `gorm:"type:varchar(36);index" json:"artist_id"`
	Artist     Artist    `gorm:"foreignKey:ArtistID" json:"-"`
	Title      string    `gorm:"not null" json:"title"`
	Year       int       `json:"year"`
	Genre      string    `json:"genre"`
	CoverPath  string    `json:"cover_path"`
	Tracks     []Track   `gorm:"foreignKey:AlbumID" json:"tracks,omitempty"`
}

type Track struct {
	Base
	LibraryID  uuid.UUID `gorm:"type:varchar(36);not null;index" json:"library_id"`
	AlbumID    uuid.UUID `gorm:"type:varchar(36);not null;index" json:"album_id"`
	Album      Album     `gorm:"foreignKey:AlbumID" json:"-"`
	ArtistID   uuid.UUID `gorm:"type:varchar(36);index" json:"artist_id"`
	Title      string    `gorm:"not null" json:"title"`
	Number     int       `json:"number"`
	DiscNumber int       `json:"disc_number"`
	Duration   int       `json:"duration"` // seconds
	FilePath   string    `gorm:"not null" json:"file_path"`
}
