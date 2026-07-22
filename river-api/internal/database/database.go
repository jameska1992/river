package database

import (
	"river-api/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Connect(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	return db, nil
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&models.User{},
		&models.RefreshToken{},
		&models.Library{},
		&models.Movie{},
		&models.TVShow{},
		&models.Season{},
		&models.Episode{},
		&models.Artist{},
		&models.Album{},
		&models.Track{},
		&models.Audiobook{},
		&models.AudiobookChapter{},
		&models.WatchProgress{},
		&models.Person{},
		&models.MovieCast{},
		&models.MovieCrew{},
		&models.TVShowCast{},
		&models.TVShowCrew{},
		&models.Subtitle{},
		&models.AudioTrack{},
		&models.Collection{},
		&models.CollectionItem{},
		&models.WatchlistItem{},
		&models.WatchParty{},
		&models.DismissedNextUp{},
		&models.ServiceLog{},
	)
}
