package repository

import (
	"strings"

	"river-api/internal/models"

	"gorm.io/gorm"
)

type SearchRepository interface {
	SearchMovies(query, genre string, limit int) ([]models.Movie, error)
	SearchTVShows(query, genre string, limit int) ([]models.TVShow, error)
	SearchAudiobooks(query, genre string, limit int) ([]models.Audiobook, error)
	SearchPeople(query string, limit int) ([]models.Person, error)
}

type gormSearchRepository struct{ db *gorm.DB }

func NewSearchRepository(db *gorm.DB) SearchRepository {
	return &gormSearchRepository{db: db}
}

func (r *gormSearchRepository) SearchMovies(query, genre string, limit int) ([]models.Movie, error) {
	var movies []models.Movie
	db := r.db.Preload("Library")
	if query != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(query)+"%")
	}
	if genre != "" {
		// Match exact JSON-encoded genre strings, e.g. ["Action","Drama"]
		db = db.Where("genres LIKE ?", `%"`+genre+`"%`)
	}
	err := db.Order("title").Limit(limit).Find(&movies).Error
	return movies, err
}

func (r *gormSearchRepository) SearchTVShows(query, genre string, limit int) ([]models.TVShow, error) {
	var shows []models.TVShow
	db := r.db.Preload("Library")
	if query != "" {
		db = db.Where("LOWER(title) LIKE ?", "%"+strings.ToLower(query)+"%")
	}
	if genre != "" {
		db = db.Where("genres LIKE ?", `%"`+genre+`"%`)
	}
	err := db.Order("title").Limit(limit).Find(&shows).Error
	return shows, err
}

func (r *gormSearchRepository) SearchAudiobooks(query, genre string, limit int) ([]models.Audiobook, error) {
	var books []models.Audiobook
	db := r.db.Preload("Library")
	if query != "" {
		pat := "%" + strings.ToLower(query) + "%"
		db = db.Where("LOWER(title) LIKE ? OR LOWER(author) LIKE ?", pat, pat)
	}
	if genre != "" {
		db = db.Where("LOWER(genre) LIKE ?", "%"+strings.ToLower(genre)+"%")
	}
	err := db.Order("title").Limit(limit).Find(&books).Error
	return books, err
}

func (r *gormSearchRepository) SearchPeople(query string, limit int) ([]models.Person, error) {
	var people []models.Person
	pat := "%" + strings.ToLower(query) + "%"
	err := r.db.
		Where("LOWER(name) LIKE ?", pat).
		Order("name").
		Limit(limit).
		Find(&people).Error
	return people, err
}
