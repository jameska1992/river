package repository

import (
	"testing"

	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newCreditsTestDB migrates just the tables the credits transaction and its
// orphan cleanup touch. The cleanup SQL is portable (NOT EXISTS subqueries,
// no DELETE aliases), so SQLite exercises the real transaction.
func newCreditsTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Person{},
		&models.MovieCast{}, &models.MovieCrew{},
		&models.TVShowCast{}, &models.TVShowCrew{},
	))
	return db
}

func personCount(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Person{}).Count(&n).Error)
	return n
}

func exists(db *gorm.DB, id uuid.UUID) bool {
	var n int64
	db.Model(&models.Person{}).Where("id = ?", id).Count(&n)
	return n > 0
}

func TestSetMovieCredits_ReapsOrphanedPersonOnReEdit(t *testing.T) {
	db := newCreditsTestDB(t)
	repo := NewCreditsRepository(db)
	movieID := uuid.New()

	// Manually-added person A, linked to the movie.
	a := models.Person{Name: "Manual A"}
	require.NoError(t, db.Create(&a).Error)
	require.NoError(t, repo.SetMovieCredits(movieID,
		[]models.MovieCast{{MovieID: movieID, PersonID: a.ID, Character: "Hero"}}, nil))

	// Re-edit the cast, replacing A with a new manual person B (the shape the
	// credits PUT produces for manual entries — a fresh Person every save).
	b := models.Person{Name: "Manual B"}
	require.NoError(t, db.Create(&b).Error)
	require.NoError(t, repo.SetMovieCredits(movieID,
		[]models.MovieCast{{MovieID: movieID, PersonID: b.ID, Character: "Hero"}}, nil))

	assert.Equal(t, int64(1), personCount(t, db), "A should be reaped, only B remains")
	assert.False(t, exists(db, a.ID), "orphaned person A should be deleted")
	assert.True(t, exists(db, b.ID), "person B still referenced, must remain")
}

func TestSetMovieCredits_KeepsPersonStillUsedElsewhere(t *testing.T) {
	db := newCreditsTestDB(t)
	repo := NewCreditsRepository(db)
	movie1, movie2 := uuid.New(), uuid.New()

	tmdbID := 100
	shared := models.Person{Name: "Shared", TmdbID: &tmdbID}
	require.NoError(t, db.Create(&shared).Error)
	require.NoError(t, repo.SetMovieCredits(movie1,
		[]models.MovieCast{{MovieID: movie1, PersonID: shared.ID}}, nil))
	require.NoError(t, repo.SetMovieCredits(movie2,
		[]models.MovieCast{{MovieID: movie2, PersonID: shared.ID}}, nil))

	// Drop `shared` from movie1; it's still on movie2, so it must survive.
	other := models.Person{Name: "Other"}
	require.NoError(t, db.Create(&other).Error)
	require.NoError(t, repo.SetMovieCredits(movie1,
		[]models.MovieCast{{MovieID: movie1, PersonID: other.ID}}, nil))

	assert.True(t, exists(db, shared.ID), "person still used by movie2 must not be reaped")
	assert.True(t, exists(db, other.ID))
}

func TestSetTVShowCredits_ReapsOrphanedPersonOnReEdit(t *testing.T) {
	db := newCreditsTestDB(t)
	repo := NewCreditsRepository(db)
	showID := uuid.New()

	a := models.Person{Name: "Manual A"}
	require.NoError(t, db.Create(&a).Error)
	require.NoError(t, repo.SetTVShowCredits(showID,
		[]models.TVShowCast{{TVShowID: showID, PersonID: a.ID}}, nil))

	b := models.Person{Name: "Manual B"}
	require.NoError(t, db.Create(&b).Error)
	require.NoError(t, repo.SetTVShowCredits(showID,
		[]models.TVShowCast{{TVShowID: showID, PersonID: b.ID}}, nil))

	assert.Equal(t, int64(1), personCount(t, db))
	assert.False(t, exists(db, a.ID))
	assert.True(t, exists(db, b.ID))
}
