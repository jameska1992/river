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
	tvCast    []models.TVShowCast
	tvCrew    []models.TVShowCrew
	// Set* capture what the service persisted.
	setCast   []models.MovieCast
	setCrew   []models.MovieCrew
	setTVCast []models.TVShowCast
	setTVCrew []models.TVShowCrew
	// Person lookup + filmography, seeded per GetPerson test.
	person     *models.Person
	personErr  error
	filmMovieC []repository.PersonMovieCastRow
	filmMovieW []repository.PersonMovieCrewRow
	filmTVC    []repository.PersonTVShowCastRow
	filmTVW    []repository.PersonTVShowCrewRow
	// Resolution call counters + tmdb-id -> stable person id.
	tmdbLookups int
	creates     int
	byTmdb      map[int]uuid.UUID
}

func (m *memCreditsRepo) GetMovieCredits(id uuid.UUID) ([]models.MovieCast, []models.MovieCrew, error) {
	return m.movieCast, m.movieCrew, nil
}

func (m *memCreditsRepo) FindOrCreatePersonByTmdbID(tmdbID int, name, profilePath, bio string) (*models.Person, error) {
	m.tmdbLookups++
	if m.byTmdb == nil {
		m.byTmdb = map[int]uuid.UUID{}
	}
	id, ok := m.byTmdb[tmdbID]
	if !ok {
		id = uuid.New()
		m.byTmdb[tmdbID] = id
	}
	return &models.Person{Base: models.Base{ID: id}, Name: name, ProfilePath: profilePath}, nil
}
func (m *memCreditsRepo) CreatePerson(name, profilePath string) (*models.Person, error) {
	m.creates++
	return &models.Person{Base: models.Base{ID: uuid.New()}, Name: name, ProfilePath: profilePath}, nil
}
func (m *memCreditsRepo) FindPersonByID(uuid.UUID) (*models.Person, error) {
	return m.person, m.personErr
}
func (m *memCreditsRepo) GetPersonFilmography(uuid.UUID) ([]repository.PersonMovieCastRow, []repository.PersonMovieCrewRow, []repository.PersonTVShowCastRow, []repository.PersonTVShowCrewRow, error) {
	return m.filmMovieC, m.filmMovieW, m.filmTVC, m.filmTVW, nil
}
func (m *memCreditsRepo) SetMovieCredits(id uuid.UUID, cast []models.MovieCast, crew []models.MovieCrew) error {
	m.setCast = cast
	m.setCrew = crew
	return nil
}
func (m *memCreditsRepo) SetTVShowCredits(_ uuid.UUID, cast []models.TVShowCast, crew []models.TVShowCrew) error {
	m.setTVCast = cast
	m.setTVCrew = crew
	return nil
}
func (m *memCreditsRepo) GetTVShowCredits(uuid.UUID) ([]models.TVShowCast, []models.TVShowCrew, error) {
	return m.tvCast, m.tvCrew, nil
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

func TestCreditsService_SetMovieCredits_InvalidID(t *testing.T) {
	svc := NewCreditsService(&memCreditsRepo{})
	err := svc.SetMovieCredits("not-a-uuid", nil, nil)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreditsService_SetMovieCredits_ResolvesPersonsAndPersists(t *testing.T) {
	repo := &memCreditsRepo{}
	svc := NewCreditsService(repo)

	cast := []CastInput{{TmdbID: 100, Name: "Boris Karloff", Character: "The Monster", Order: 1}}
	crew := []CrewInput{{TmdbID: 0, Name: "James Whale", Job: "Director", Department: "Directing"}}

	require.NoError(t, svc.SetMovieCredits(uuid.New().String(), cast, crew))

	// tmdb-backed cast member is resolved via the tmdb lookup; the
	// tmdb-less crew member is created fresh.
	assert.Equal(t, 1, repo.tmdbLookups, "cast with a TMDB id uses FindOrCreatePersonByTmdbID")
	assert.Equal(t, 1, repo.creates, "crew without a TMDB id uses CreatePerson")

	require.Len(t, repo.setCast, 1)
	assert.Equal(t, "The Monster", repo.setCast[0].Character)
	assert.Equal(t, 1, repo.setCast[0].CastOrder)
	assert.NotEqual(t, uuid.Nil, repo.setCast[0].PersonID)

	require.Len(t, repo.setCrew, 1)
	assert.Equal(t, "Director", repo.setCrew[0].Job)
	assert.Equal(t, "Directing", repo.setCrew[0].Department)
}

func TestCreditsService_SetMovieCredits_SameTmdbIDReusesPerson(t *testing.T) {
	repo := &memCreditsRepo{}
	svc := NewCreditsService(repo)

	// Two cast rows for the same TMDB id should resolve to the same person.
	cast := []CastInput{
		{TmdbID: 42, Name: "Actor", Character: "A", Order: 1},
		{TmdbID: 42, Name: "Actor", Character: "B", Order: 2},
	}
	require.NoError(t, svc.SetMovieCredits(uuid.New().String(), cast, nil))
	require.Len(t, repo.setCast, 2)
	assert.Equal(t, repo.setCast[0].PersonID, repo.setCast[1].PersonID, "same TMDB id => one person")
}

// --- GetPerson ---

func TestCreditsService_GetPerson_InvalidID(t *testing.T) {
	svc := NewCreditsService(&memCreditsRepo{})
	_, err := svc.GetPerson("not-a-uuid")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreditsService_GetPerson_PropagatesLookupError(t *testing.T) {
	repo := &memCreditsRepo{personErr: ErrNotFound}
	svc := NewCreditsService(repo)
	_, err := svc.GetPerson(uuid.New().String())
	assert.ErrorIs(t, err, ErrNotFound, "an unknown (but well-formed) id surfaces the repo error")
}

func TestCreditsService_GetPerson_MapsFilmography(t *testing.T) {
	pid := uuid.New()
	tmdb := 1234
	repo := &memCreditsRepo{
		person: &models.Person{Base: models.Base{ID: pid}, Name: "Sigourney Weaver", ProfilePath: "/sw.jpg", Biography: "Actor.", TmdbID: &tmdb},
		filmMovieC: []repository.PersonMovieCastRow{
			{MovieID: "m1", Title: "Alien", Year: 1979, PosterPath: "/a.jpg", Character: "Ripley"},
		},
		filmMovieW: []repository.PersonMovieCrewRow{
			{MovieID: "m2", Title: "Producer Pic", Year: 2000, Job: "Producer", Department: "Production"},
		},
		filmTVC: []repository.PersonTVShowCastRow{
			{TVShowID: "s1", Title: "Some Show", Year: 2015, Character: "Herself"},
		},
		filmTVW: []repository.PersonTVShowCrewRow{
			{TVShowID: "s2", Title: "Doc Series", Year: 2018, Job: "Narrator", Department: "Sound"},
		},
	}
	svc := NewCreditsService(repo)

	res, err := svc.GetPerson(pid.String())
	require.NoError(t, err)

	assert.Equal(t, pid.String(), res.ID)
	assert.Equal(t, "Sigourney Weaver", res.Name)
	assert.Equal(t, "Actor.", res.Biography)
	require.NotNil(t, res.TmdbID)
	assert.Equal(t, 1234, *res.TmdbID)

	require.Len(t, res.MovieCast, 1)
	assert.Equal(t, "Alien", res.MovieCast[0].Title)
	assert.Equal(t, "Ripley", res.MovieCast[0].Character)

	require.Len(t, res.MovieCrew, 1)
	assert.Equal(t, "Producer", res.MovieCrew[0].Job)

	require.Len(t, res.TVShowCast, 1)
	assert.Equal(t, "Herself", res.TVShowCast[0].Character)

	require.Len(t, res.TVShowCrew, 1)
	assert.Equal(t, "Narrator", res.TVShowCrew[0].Job)
	assert.Equal(t, "Sound", res.TVShowCrew[0].Department)
}

// --- TV show credits ---

func TestCreditsService_SetTVShowCredits_InvalidID(t *testing.T) {
	svc := NewCreditsService(&memCreditsRepo{})
	err := svc.SetTVShowCredits("not-a-uuid", nil, nil)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreditsService_SetTVShowCredits_ResolvesPersonsAndPersists(t *testing.T) {
	repo := &memCreditsRepo{}
	svc := NewCreditsService(repo)

	cast := []CastInput{{TmdbID: 100, Name: "Gillian Anderson", Character: "Scully", Order: 1}}
	crew := []CrewInput{{TmdbID: 0, Name: "Chris Carter", Job: "Creator", Department: "Writing"}}

	require.NoError(t, svc.SetTVShowCredits(uuid.New().String(), cast, crew))

	assert.Equal(t, 1, repo.tmdbLookups, "cast with a TMDB id uses FindOrCreatePersonByTmdbID")
	assert.Equal(t, 1, repo.creates, "crew without a TMDB id uses CreatePerson")

	require.Len(t, repo.setTVCast, 1)
	assert.Equal(t, "Scully", repo.setTVCast[0].Character)
	assert.Equal(t, 1, repo.setTVCast[0].CastOrder)
	assert.NotEqual(t, uuid.Nil, repo.setTVCast[0].PersonID)

	require.Len(t, repo.setTVCrew, 1)
	assert.Equal(t, "Creator", repo.setTVCrew[0].Job)
	assert.Equal(t, "Writing", repo.setTVCrew[0].Department)
}

func TestCreditsService_GetTVShowCredits_InvalidID(t *testing.T) {
	svc := NewCreditsService(&memCreditsRepo{})
	_, err := svc.GetTVShowCredits("not-a-uuid")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestCreditsService_GetTVShowCredits_MapsCastAndCrew(t *testing.T) {
	personA := uuid.New()
	personB := uuid.New()
	tmdb := 77
	repo := &memCreditsRepo{
		tvCast: []models.TVShowCast{{
			PersonID:  personA,
			Character: "Mulder",
			CastOrder: 2,
			Person:    models.Person{Base: models.Base{ID: personA}, Name: "David Duchovny", ProfilePath: "/dd.jpg", TmdbID: &tmdb},
		}},
		tvCrew: []models.TVShowCrew{{
			PersonID:   personB,
			Job:        "Composer",
			Department: "Sound",
			Person:     models.Person{Base: models.Base{ID: personB}, Name: "Mark Snow"},
		}},
	}
	svc := NewCreditsService(repo)

	res, err := svc.GetTVShowCredits(uuid.New().String())
	require.NoError(t, err)

	require.Len(t, res.Cast, 1)
	assert.Equal(t, "David Duchovny", res.Cast[0].Name)
	assert.Equal(t, "Mulder", res.Cast[0].Character)
	assert.Equal(t, 2, res.Cast[0].Order)
	require.NotNil(t, res.Cast[0].TmdbID)
	assert.Equal(t, 77, *res.Cast[0].TmdbID)

	require.Len(t, res.Crew, 1)
	assert.Equal(t, "Mark Snow", res.Crew[0].Name)
	assert.Equal(t, "Composer", res.Crew[0].Job)
	assert.Equal(t, "Sound", res.Crew[0].Department)
}
