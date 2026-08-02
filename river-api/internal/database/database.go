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
	// pg_trgm backs the fuzzy/relevance search in the repository layer. Must
	// exist before the GIN indexes below reference gin_trgm_ops.
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS pg_trgm").Error; err != nil {
		return err
	}
	if err := db.AutoMigrate(
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
		&models.Setting{},
	); err != nil {
		return err
	}
	return createSearchIndexes(db)
}

// createSearchIndexes builds GIN trigram indexes backing fuzzy search. Each is
// on the same LOWER(col) expression the search queries use, so it accelerates
// both LIKE '%...%' and similarity(). IF NOT EXISTS keeps this idempotent
// across restarts.
func createSearchIndexes(db *gorm.DB) error {
	stmts := []string{
		"CREATE INDEX IF NOT EXISTS idx_movies_title_trgm ON movies USING gin ((LOWER(title)) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_tv_shows_title_trgm ON tv_shows USING gin ((LOWER(title)) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_audiobooks_title_trgm ON audiobooks USING gin ((LOWER(title)) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_audiobooks_author_trgm ON audiobooks USING gin ((LOWER(author)) gin_trgm_ops)",
		"CREATE INDEX IF NOT EXISTS idx_people_name_trgm ON people USING gin ((LOWER(name)) gin_trgm_ops)",
	}
	for _, s := range stmts {
		if err := db.Exec(s).Error; err != nil {
			return err
		}
	}
	return nil
}
