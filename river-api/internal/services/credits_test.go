package services

import (
	"testing"

	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memCreditsRepo struct {
	movieCast []models.MovieCast
	movieCrew []models.MovieCrew
}

func (m *memCreditsRepo) GetMovieCredits(id uuid.UUID) ([]models.MovieCast, []models.MovieCrew, error) {
	return m.movieCast, m.movieCrew, nil
}

// Unused-by-these-tests methods, stubbed to satisfy the interface.
func (m *memCreditsRepo) FindOrCreatePersonByTmdbID(int, string, string, string) (*models.Person, error) {
	return nil, nil
}
func (m *memCreditsRepo) CreatePerson(string, string) (*models.Person, error) { return nil, nil }
func (m *memCreditsRepo) FindPersonByID(uuid.UUID) (*models.Person, error)     { return nil, nil }
func (m *memCreditsRepo) GetPersonFilmography(uuid.UUID) ([]repository.PersonMovieCastRow, []repository.PersonMovieCrewRow, []repository.PersonTVShowCastRow, []repository.PersonTVShowCrewRow, error) {
	return nil, nil, nil, nil, nil
}
func (m *memCreditsRepo) SetMovieCredits(uuid.UUID, []models.MovieCast, []models.MovieCrew) error {
	return nil
}
func (m *memCreditsRepo) SetTVShowCredits(uuid.UUID, []models.TVShowCast, []models.TVShowCrew) error {
	return nil
}
func (m *memCreditsRepo) GetTVShowCredits(uuid.UUID) ([]models.TVShowCast, []models.TVShowCrew, error) {
	return nil, nil, nil
}

func TestCreditsService_GetMovieCredits_InvalidID(t *testing.T) {
	svc := NewCreditsService(&memCreditsRepo{})
	_, err := svc.GetMovieCredits("not-a-uuid")
	assert.ErrorIs(t, err, ErrNotFound, "a non-uuid movie id should be not-found")
}

func TestCreditsService_GetMovieCredits_MapsCastAndCrew(t *testing.T) {
	personA := uuid.New()
	personB := uuid.New()
	repo := &memCreditsRepo{
		movieCast: []models.MovieCast{{
			PersonID:  personA,
			Character: "The Monster",
			CastOrder: 1,
			Person:    models.Person{Base: models.Base{ID: personA}, Name: "Boris Karloff", ProfilePath: "/bk.jpg"},
		}},
		movieCrew: []models.MovieCrew{{
			PersonID:   personB,
			Job:        "Director",
			Department: "Directing",
			Person:     models.Person{Base: models.Base{ID: personB}, Name: "James Whale"},
		}},
	}
	svc := NewCreditsService(repo)

	res, err := svc.GetMovieCredits(uuid.New().String())
	require.NoError(t, err)
	require.Len(t, res.Cast, 1)
	require.Len(t, res.Crew, 1)

	assert.Equal(t, "Boris Karloff", res.Cast[0].Name)
	assert.Equal(t, "The Monster", res.Cast[0].Character)
	assert.Equal(t, 1, res.Cast[0].Order)
	assert.Equal(t, "/bk.jpg", res.Cast[0].ProfilePath)

	assert.Equal(t, "James Whale", res.Crew[0].Name)
	assert.Equal(t, "Director", res.Crew[0].Job)
	assert.Equal(t, "Directing", res.Crew[0].Department)
}
