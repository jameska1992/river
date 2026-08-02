package repository

import (
	"strings"

	"river-api/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
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

// applyTitleSearch adds fuzzy relevance matching and ranking on the title
// column. With an empty query (e.g. a genre-only browse) it falls back to plain
// alphabetical order.
func applyTitleSearch(db *gorm.DB, query string) *gorm.DB {
	if query == "" {
		return db.Order("title")
	}
	q := strings.ToLower(query)
	col := "LOWER(title)"
	where, wargs := fuzzyMatch(col, q)
	order, oargs := relevanceOrder(col, "title", q)
	return db.Where(where, wargs...).
		Clauses(clause.OrderBy{Expression: gorm.Expr(order, oargs...)})
}

func (r *gormSearchRepository) SearchMovies(query, genre string, limit int) ([]models.Movie, error) {
	var movies []models.Movie
	db := applyTitleSearch(r.db.Preload("Library"), query)
	if genre != "" {
		// Match exact JSON-encoded genre strings, e.g. ["Action","Drama"]
		db = db.Where("genres LIKE ?", `%"`+genre+`"%`)
	}
	err := db.Limit(limit).Find(&movies).Error
	return movies, err
}

func (r *gormSearchRepository) SearchTVShows(query, genre string, limit int) ([]models.TVShow, error) {
	var shows []models.TVShow
	db := applyTitleSearch(r.db.Preload("Library"), query)
	if genre != "" {
		db = db.Where("genres LIKE ?", `%"`+genre+`"%`)
	}
	err := db.Limit(limit).Find(&shows).Error
	return shows, err
}

func (r *gormSearchRepository) SearchAudiobooks(query, genre string, limit int) ([]models.Audiobook, error) {
	var books []models.Audiobook
	db := r.db.Preload("Library")
	if query != "" {
		q := strings.ToLower(query)
		where, wargs := fuzzyMatchAny([]string{"LOWER(title)", "LOWER(author)"}, q)
		// Rank by title relevance; author still participates in matching above.
		order, oargs := relevanceOrder("LOWER(title)", "title", q)
		db = db.Where(where, wargs...).
			Clauses(clause.OrderBy{Expression: gorm.Expr(order, oargs...)})
	} else {
		db = db.Order("title")
	}
	if genre != "" {
		db = db.Where("LOWER(genre) LIKE ?", "%"+strings.ToLower(genre)+"%")
	}
	err := db.Limit(limit).Find(&books).Error
	return books, err
}

func (r *gormSearchRepository) SearchPeople(query string, limit int) ([]models.Person, error) {
	var people []models.Person
	q := strings.ToLower(query)
	col := "LOWER(name)"
	where, wargs := fuzzyMatch(col, q)
	order, oargs := relevanceOrder(col, "name", q)
	err := r.db.
		Where(where, wargs...).
		Clauses(clause.OrderBy{Expression: gorm.Expr(order, oargs...)}).
		Limit(limit).
		Find(&people).Error
	return people, err
}
