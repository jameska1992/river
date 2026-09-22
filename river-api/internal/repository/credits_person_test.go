package repository

import (
	"testing"

	"river-api/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindOrCreatePersonByName_ReusesManualPerson(t *testing.T) {
	db := newCreditsTestDB(t)
	repo := NewCreditsRepository(db)

	p1, err := repo.FindOrCreatePersonByName("Jane Doe", "")
	require.NoError(t, err)

	// Same name, different case + a profile path we didn't have before.
	p2, err := repo.FindOrCreatePersonByName("jane doe", "/jane.jpg")
	require.NoError(t, err)

	assert.Equal(t, p1.ID, p2.ID, "case-insensitive name match reuses the existing row")
	assert.Equal(t, int64(1), personCount(t, db), "no duplicate row created")

	var got models.Person
	require.NoError(t, db.First(&got, "id = ?", p1.ID).Error)
	assert.Equal(t, "/jane.jpg", got.ProfilePath, "profile path backfilled when previously empty")
}

func TestFindOrCreatePersonByName_DoesNotFoldIntoTmdbPerson(t *testing.T) {
	db := newCreditsTestDB(t)
	repo := NewCreditsRepository(db)

	tmdb := 42
	require.NoError(t, db.Create(&models.Person{Name: "Jane Doe", TmdbID: &tmdb}).Error)

	p, err := repo.FindOrCreatePersonByName("Jane Doe", "")
	require.NoError(t, err)
	assert.Nil(t, p.TmdbID, "a manual credit gets a new manual row, never the TMDB one")
	assert.Equal(t, int64(2), personCount(t, db))
}

func TestFindOrCreatePersonByName_BlankNameNotDeduped(t *testing.T) {
	db := newCreditsTestDB(t)
	repo := NewCreditsRepository(db)

	_, err := repo.FindOrCreatePersonByName("", "")
	require.NoError(t, err)
	_, err = repo.FindOrCreatePersonByName("   ", "")
	require.NoError(t, err)

	assert.Equal(t, int64(2), personCount(t, db), "blank/whitespace names are never merged together")
}
