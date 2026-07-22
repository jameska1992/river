package models

import "github.com/google/uuid"

type Person struct {
	Base
	Name        string `gorm:"not null" json:"name"`
	ProfilePath string `json:"profile_path"`
	Biography   string `json:"biography"`
	TmdbID      *int   `gorm:"uniqueIndex" json:"tmdb_id"`
}

type MovieCast struct {
	MovieID   uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	PersonID  uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	Character string    `json:"character"`
	CastOrder int       `gorm:"column:cast_order" json:"order"`
	Person    Person    `gorm:"foreignKey:PersonID;references:ID" json:"person"`
}

type MovieCrew struct {
	MovieID    uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	PersonID   uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	Job        string    `gorm:"primaryKey;type:varchar(100)" json:"job"`
	Department string    `json:"department"`
	Person     Person    `gorm:"foreignKey:PersonID;references:ID" json:"person"`
}

type TVShowCast struct {
	TVShowID  uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	PersonID  uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	Character string    `json:"character"`
	CastOrder int       `gorm:"column:cast_order" json:"order"`
	Person    Person    `gorm:"foreignKey:PersonID;references:ID" json:"person"`
}

type TVShowCrew struct {
	TVShowID   uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	PersonID   uuid.UUID `gorm:"type:varchar(36);primaryKey" json:"-"`
	Job        string    `gorm:"primaryKey;type:varchar(100)" json:"job"`
	Department string    `json:"department"`
	Person     Person    `gorm:"foreignKey:PersonID;references:ID" json:"person"`
}
