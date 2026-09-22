package backfill

import (
	"testing"

	"river-api/internal/models"

	"github.com/glebarez/sqlite"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newPeopleTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.Person{}, &models.MovieCast{}, &models.MovieCrew{},
		&models.TVShowCast{}, &models.TVShowCrew{},
	))
	return db
}

func personExists(db *gorm.DB, id uuid.UUID) bool {
	var n int64
	db.Model(&models.Person{}).Where("id = ?", id).Count(&n)
	return n > 0
}

func TestMergeDuplicatePeople(t *testing.T) {
	db := newPeopleTestDB(t)

	// Three manual "Jane Doe" rows (varying case) that should collapse to one.
	jane1 := models.Person{Base: models.Base{ID: uuid.New()}, Name: "Jane Doe"}
	jane2 := models.Person{Base: models.Base{ID: uuid.New()}, Name: "jane doe"}
	jane3 := models.Person{Base: models.Base{ID: uuid.New()}, Name: "JANE DOE"}
	// A TMDB-backed "Jane Doe" must be left untouched (different identity space).
	tmdb := 99
	janeTmdb := models.Person{Base: models.Base{ID: uuid.New()}, Name: "Jane Doe", TmdbID: &tmdb}
	// A lone manual person with no duplicate must be left alone.
	solo := models.Person{Base: models.Base{ID: uuid.New()}, Name: "Solo"}
	for _, p := range []*models.Person{&jane1, &jane2, &jane3, &janeTmdb, &solo} {
		require.NoError(t, db.Create(p).Error)
	}

	movieA := uuid.New()
	movieB := uuid.New()
	// jane1 AND jane2 are both credited on movie A → a collision the merge must
	// de-dupe rather than double. jane3 is credited on movie B.
	require.NoError(t, db.Create(&models.MovieCast{MovieID: movieA, PersonID: jane1.ID, Character: "Self", CastOrder: 1}).Error)
	require.NoError(t, db.Create(&models.MovieCast{MovieID: movieA, PersonID: jane2.ID, Character: "Self", CastOrder: 1}).Error)
	require.NoError(t, db.Create(&models.MovieCast{MovieID: movieB, PersonID: jane3.ID, Character: "Extra", CastOrder: 2}).Error)

	MergeDuplicatePeople(db)

	// Manual Janes collapsed to one; TMDB Jane + Solo remain → 3 total.
	var total int64
	require.NoError(t, db.Model(&models.Person{}).Count(&total).Error)
	assert.Equal(t, int64(3), total)
	assert.True(t, personExists(db, janeTmdb.ID), "TMDB person untouched")
	assert.True(t, personExists(db, solo.ID), "non-duplicated person untouched")

	// Exactly one manual Jane survives — assert against whichever it is.
	var survivors []models.Person
	require.NoError(t, db.Where("tmdb_id IS NULL AND LOWER(name) = ?", "jane doe").Find(&survivors).Error)
	require.Len(t, survivors, 1)
	survivor := survivors[0].ID

	var aCount, bCount int64
	db.Model(&models.MovieCast{}).Where("movie_id = ? AND person_id = ?", movieA, survivor).Count(&aCount)
	db.Model(&models.MovieCast{}).Where("movie_id = ? AND person_id = ?", movieB, survivor).Count(&bCount)
	assert.Equal(t, int64(1), aCount, "movie A collision de-duped to a single credit")
	assert.Equal(t, int64(1), bCount, "movie B credit re-pointed to the survivor")

	// No credits left pointing at a removed row.
	var totalCredits int64
	db.Model(&models.MovieCast{}).Count(&totalCredits)
	assert.Equal(t, int64(2), totalCredits, "only the survivor's two credits remain")

	// Idempotent: a second pass changes nothing.
	MergeDuplicatePeople(db)
	require.NoError(t, db.Model(&models.Person{}).Count(&total).Error)
	assert.Equal(t, int64(3), total)
}
