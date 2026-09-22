package repository

import (
	"errors"
	"strings"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PersonMovieCastRow is returned by GetPersonFilmography for movie cast credits.
type PersonMovieCastRow struct {
	MovieID    string `gorm:"column:movie_id"`
	Title      string `gorm:"column:title"`
	Year       int    `gorm:"column:year"`
	PosterPath string `gorm:"column:poster_path"`
	Character  string `gorm:"column:character"`
}

// PersonMovieCrewRow is returned by GetPersonFilmography for movie crew credits.
type PersonMovieCrewRow struct {
	MovieID    string `gorm:"column:movie_id"`
	Title      string `gorm:"column:title"`
	Year       int    `gorm:"column:year"`
	PosterPath string `gorm:"column:poster_path"`
	Job        string `gorm:"column:job"`
	Department string `gorm:"column:department"`
}

// PersonTVShowCastRow is returned by GetPersonFilmography for TV cast credits.
type PersonTVShowCastRow struct {
	TVShowID   string `gorm:"column:tv_show_id"`
	Title      string `gorm:"column:title"`
	Year       int    `gorm:"column:year"`
	PosterPath string `gorm:"column:poster_path"`
	Character  string `gorm:"column:character"`
}

// PersonTVShowCrewRow is returned by GetPersonFilmography for TV crew credits.
type PersonTVShowCrewRow struct {
	TVShowID   string `gorm:"column:tv_show_id"`
	Title      string `gorm:"column:title"`
	Year       int    `gorm:"column:year"`
	PosterPath string `gorm:"column:poster_path"`
	Job        string `gorm:"column:job"`
	Department string `gorm:"column:department"`
}

type CreditsRepository interface {
	FindOrCreatePersonByTmdbID(tmdbID int, name, profilePath, biography string) (*models.Person, error)
	FindOrCreatePersonByName(name, profilePath string) (*models.Person, error)
	FindPersonByID(personID uuid.UUID) (*models.Person, error)
	GetPersonFilmography(personID uuid.UUID) ([]PersonMovieCastRow, []PersonMovieCrewRow, []PersonTVShowCastRow, []PersonTVShowCrewRow, error)
	SetMovieCredits(movieID uuid.UUID, cast []models.MovieCast, crew []models.MovieCrew, locked *bool) error
	GetMovieCredits(movieID uuid.UUID) ([]models.MovieCast, []models.MovieCrew, error)
	SetTVShowCredits(tvShowID uuid.UUID, cast []models.TVShowCast, crew []models.TVShowCrew, locked *bool) error
	GetTVShowCredits(tvShowID uuid.UUID) ([]models.TVShowCast, []models.TVShowCrew, error)
}

type gormCreditsRepository struct{ db *gorm.DB }

func NewCreditsRepository(db *gorm.DB) CreditsRepository {
	return &gormCreditsRepository{db: db}
}

// creditLink names a credit join table and its media foreign-key column.
type creditLink struct{ table, idCol string }

// linkedPersonIDs returns the distinct person ids currently linked to a
// media item across the given credit tables. Used to snapshot who was
// attached before a full-replace so orphans can be cleaned up afterward.
func linkedPersonIDs(tx *gorm.DB, links []creditLink, mediaID uuid.UUID) ([]string, error) {
	seen := map[string]struct{}{}
	for _, l := range links {
		var ids []string
		if err := tx.Table(l.table).
			Where(l.idCol+" = ?", mediaID).
			Pluck("person_id", &ids).Error; err != nil {
			return nil, err
		}
		for _, id := range ids {
			seen[id] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	return out, nil
}

// deleteOrphanedPeople removes any of the given people that are no longer
// referenced by ANY cast/crew credit (movie or TV). Manually-added people
// (tmdb_id NULL) accumulate on every re-edit because the credits PUT can
// only create them, never re-link — this reaps the ones a replace left
// dangling. Scoped to the just-unlinked candidates so a Person still used
// by another title is never touched.
func deleteOrphanedPeople(tx *gorm.DB, candidateIDs []string) error {
	if len(candidateIDs) == 0 {
		return nil
	}
	return tx.Where("id IN ?", candidateIDs).
		Where("NOT EXISTS (SELECT 1 FROM movie_casts WHERE movie_casts.person_id = people.id)").
		Where("NOT EXISTS (SELECT 1 FROM movie_crews WHERE movie_crews.person_id = people.id)").
		Where("NOT EXISTS (SELECT 1 FROM tv_show_casts WHERE tv_show_casts.person_id = people.id)").
		Where("NOT EXISTS (SELECT 1 FROM tv_show_crews WHERE tv_show_crews.person_id = people.id)").
		Delete(&models.Person{}).Error
}

func (r *gormCreditsRepository) FindPersonByID(personID uuid.UUID) (*models.Person, error) {
	var p models.Person
	if err := r.db.First(&p, "id = ?", personID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, apperrors.ErrNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *gormCreditsRepository) GetPersonFilmography(personID uuid.UUID) (
	[]PersonMovieCastRow, []PersonMovieCrewRow, []PersonTVShowCastRow, []PersonTVShowCrewRow, error,
) {
	var mc []PersonMovieCastRow
	if err := r.db.Table("movie_casts").
		Select("movie_casts.movie_id, movies.title, movies.year, movies.poster_path, movie_casts.character").
		Joins("JOIN movies ON movies.id = movie_casts.movie_id").
		Where("movie_casts.person_id = ?", personID).
		Order("movies.year DESC, movies.title").
		Scan(&mc).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var mw []PersonMovieCrewRow
	if err := r.db.Table("movie_crews").
		Select("movie_crews.movie_id, movies.title, movies.year, movies.poster_path, movie_crews.job, movie_crews.department").
		Joins("JOIN movies ON movies.id = movie_crews.movie_id").
		Where("movie_crews.person_id = ?", personID).
		Order("movies.year DESC, movies.title").
		Scan(&mw).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var sc []PersonTVShowCastRow
	if err := r.db.Table("tv_show_casts").
		Select("tv_show_casts.tv_show_id, tv_shows.title, tv_shows.year, tv_shows.poster_path, tv_show_casts.character").
		Joins("JOIN tv_shows ON tv_shows.id = tv_show_casts.tv_show_id").
		Where("tv_show_casts.person_id = ?", personID).
		Order("tv_shows.year DESC, tv_shows.title").
		Scan(&sc).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	var sw []PersonTVShowCrewRow
	if err := r.db.Table("tv_show_crews").
		Select("tv_show_crews.tv_show_id, tv_shows.title, tv_shows.year, tv_shows.poster_path, tv_show_crews.job, tv_show_crews.department").
		Joins("JOIN tv_shows ON tv_shows.id = tv_show_crews.tv_show_id").
		Where("tv_show_crews.person_id = ?", personID).
		Order("tv_shows.year DESC, tv_shows.title").
		Scan(&sw).Error; err != nil {
		return nil, nil, nil, nil, err
	}

	return mc, mw, sc, sw, nil
}

func (r *gormCreditsRepository) FindOrCreatePersonByTmdbID(tmdbID int, name, profilePath, biography string) (*models.Person, error) {
	var existing models.Person
	if err := r.db.Where("tmdb_id = ?", tmdbID).First(&existing).Error; err == nil {
		needsUpdate := existing.Name != name || existing.ProfilePath != profilePath ||
			(biography != "" && existing.Biography != biography)
		if needsUpdate {
			updates := map[string]any{"name": name, "profile_path": profilePath}
			if biography != "" {
				updates["biography"] = biography
			}
			r.db.Model(&existing).Updates(updates)
		}
		return &existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	p := models.Person{Name: name, ProfilePath: profilePath, Biography: biography, TmdbID: &tmdbID}
	if err := r.db.Create(&p).Error; err != nil {
		// Handle race: another worker may have created the same person.
		var existing2 models.Person
		if r.db.Where("tmdb_id = ?", tmdbID).First(&existing2).Error == nil {
			return &existing2, nil
		}
		return nil, err
	}
	return &p, nil
}

// FindOrCreatePersonByName resolves a manually-entered person (no TMDB id).
// It reuses an existing manual person whose name matches case-insensitively —
// scoped to tmdb_id IS NULL so a manual credit is never folded into a
// TMDB-sourced person, and vice-versa. Without this the manual path inserted a
// brand-new Person on every credit, so the same human added to several titles
// surfaced multiple times in search (#193).
//
// A blank name is never deduped (it would collapse unrelated unnamed rows) —
// it falls straight through to a plain insert.
func (r *gormCreditsRepository) FindOrCreatePersonByName(name, profilePath string) (*models.Person, error) {
	if strings.TrimSpace(name) == "" {
		p := models.Person{Name: name, ProfilePath: profilePath}
		return &p, r.db.Create(&p).Error
	}

	var existing models.Person
	err := r.db.Where("tmdb_id IS NULL AND LOWER(name) = LOWER(?)", name).First(&existing).Error
	if err == nil {
		// Backfill a profile image if the existing row has none and we now
		// have one — best-effort enrichment that never blanks a set value.
		if existing.ProfilePath == "" && profilePath != "" {
			r.db.Model(&existing).Update("profile_path", profilePath)
		}
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	p := models.Person{Name: name, ProfilePath: profilePath}
	if err := r.db.Create(&p).Error; err != nil {
		// Race: another writer may have just created the same manual person.
		var existing2 models.Person
		if r.db.Where("tmdb_id IS NULL AND LOWER(name) = LOWER(?)", name).First(&existing2).Error == nil {
			return &existing2, nil
		}
		return nil, err
	}
	return &p, nil
}

func (r *gormCreditsRepository) SetMovieCredits(movieID uuid.UUID, cast []models.MovieCast, crew []models.MovieCrew, locked *bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Snapshot the people currently linked to this movie before we
		// replace the credits, so we can clean up any that end up orphaned.
		prev, err := linkedPersonIDs(tx, []creditLink{
			{"movie_casts", "movie_id"}, {"movie_crews", "movie_id"},
		}, movieID)
		if err != nil {
			return err
		}
		if err := tx.Where("movie_id = ?", movieID).Delete(&models.MovieCast{}).Error; err != nil {
			return err
		}
		if err := tx.Where("movie_id = ?", movieID).Delete(&models.MovieCrew{}).Error; err != nil {
			return err
		}
		if len(cast) > 0 {
			if err := tx.Create(&cast).Error; err != nil {
				return err
			}
		}
		if len(crew) > 0 {
			if err := tx.Create(&crew).Error; err != nil {
				return err
			}
		}
		// Set the manual-edit lock in the same transaction when requested.
		if locked != nil {
			if err := tx.Model(&models.Movie{}).Where("id = ?", movieID).
				Update("credits_locked", *locked).Error; err != nil {
				return err
			}
		}
		return deleteOrphanedPeople(tx, prev)
	})
}

func (r *gormCreditsRepository) GetMovieCredits(movieID uuid.UUID) ([]models.MovieCast, []models.MovieCrew, error) {
	var cast []models.MovieCast
	if err := r.db.Preload("Person").Where("movie_id = ?", movieID).Order("cast_order").Find(&cast).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil, apperrors.ErrNotFound
		}
		return nil, nil, err
	}
	var crew []models.MovieCrew
	if err := r.db.Preload("Person").Where("movie_id = ?", movieID).Order("department, job").Find(&crew).Error; err != nil {
		return nil, nil, err
	}
	return cast, crew, nil
}

func (r *gormCreditsRepository) SetTVShowCredits(tvShowID uuid.UUID, cast []models.TVShowCast, crew []models.TVShowCrew, locked *bool) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		prev, err := linkedPersonIDs(tx, []creditLink{
			{"tv_show_casts", "tv_show_id"}, {"tv_show_crews", "tv_show_id"},
		}, tvShowID)
		if err != nil {
			return err
		}
		if err := tx.Where("tv_show_id = ?", tvShowID).Delete(&models.TVShowCast{}).Error; err != nil {
			return err
		}
		if err := tx.Where("tv_show_id = ?", tvShowID).Delete(&models.TVShowCrew{}).Error; err != nil {
			return err
		}
		if len(cast) > 0 {
			if err := tx.Create(&cast).Error; err != nil {
				return err
			}
		}
		if len(crew) > 0 {
			if err := tx.Create(&crew).Error; err != nil {
				return err
			}
		}
		if locked != nil {
			if err := tx.Model(&models.TVShow{}).Where("id = ?", tvShowID).
				Update("credits_locked", *locked).Error; err != nil {
				return err
			}
		}
		return deleteOrphanedPeople(tx, prev)
	})
}

func (r *gormCreditsRepository) GetTVShowCredits(tvShowID uuid.UUID) ([]models.TVShowCast, []models.TVShowCrew, error) {
	var cast []models.TVShowCast
	if err := r.db.Preload("Person").Where("tv_show_id = ?", tvShowID).Order("cast_order").Find(&cast).Error; err != nil {
		return nil, nil, err
	}
	var crew []models.TVShowCrew
	if err := r.db.Preload("Person").Where("tv_show_id = ?", tvShowID).Order("department, job").Find(&crew).Error; err != nil {
		return nil, nil, err
	}
	return cast, crew, nil
}
