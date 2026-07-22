package repository

import (
	"river-api/internal/models"

	"gorm.io/gorm"
)

type StatsRepository interface {
	CountMovies() (int64, error)
	CountTVShows() (int64, error)
	CountTracks() (int64, error)
	CountAudiobooks() (int64, error)
}

type gormStatsRepository struct{ db *gorm.DB }

func NewStatsRepository(db *gorm.DB) StatsRepository {
	return &gormStatsRepository{db: db}
}

func (r *gormStatsRepository) CountMovies() (int64, error) {
	var n int64
	return n, r.db.Model(&models.Movie{}).Count(&n).Error
}

func (r *gormStatsRepository) CountTVShows() (int64, error) {
	var n int64
	return n, r.db.Model(&models.TVShow{}).Count(&n).Error
}

func (r *gormStatsRepository) CountTracks() (int64, error) {
	var n int64
	return n, r.db.Model(&models.Track{}).Count(&n).Error
}

func (r *gormStatsRepository) CountAudiobooks() (int64, error) {
	var n int64
	return n, r.db.Model(&models.Audiobook{}).Count(&n).Error
}
