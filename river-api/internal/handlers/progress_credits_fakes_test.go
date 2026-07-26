package handlers

import (
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

// --- ServiceLog ---

type fakeServiceLogRepo struct {
	entries   []models.ServiceLog
	createErr error
	listErr   error
}

func (f *fakeServiceLogRepo) Create(e *models.ServiceLog) error {
	if f.createErr != nil {
		return f.createErr
	}
	f.entries = append(f.entries, *e)
	return nil
}

func (f *fakeServiceLogRepo) List(_ repository.ListLogsFilter) ([]models.ServiceLog, int64, error) {
	if f.listErr != nil {
		return nil, 0, f.listErr
	}
	return f.entries, int64(len(f.entries)), nil
}

// --- Credits ---

type fakeCreditsRepo struct {
	person    *models.Person
	personErr error
	movieCast []models.MovieCast
	movieCrew []models.MovieCrew
	tvCast    []models.TVShowCast
	tvCrew    []models.TVShowCrew
}

func (f *fakeCreditsRepo) FindOrCreatePersonByTmdbID(_ int, name, profilePath, _ string) (*models.Person, error) {
	return &models.Person{Base: models.Base{ID: uuid.New()}, Name: name, ProfilePath: profilePath}, nil
}
func (f *fakeCreditsRepo) CreatePerson(name, profilePath string) (*models.Person, error) {
	return &models.Person{Base: models.Base{ID: uuid.New()}, Name: name, ProfilePath: profilePath}, nil
}
func (f *fakeCreditsRepo) FindPersonByID(uuid.UUID) (*models.Person, error) {
	return f.person, f.personErr
}
func (f *fakeCreditsRepo) GetPersonFilmography(uuid.UUID) ([]repository.PersonMovieCastRow, []repository.PersonMovieCrewRow, []repository.PersonTVShowCastRow, []repository.PersonTVShowCrewRow, error) {
	return nil, nil, nil, nil, nil
}
func (f *fakeCreditsRepo) SetMovieCredits(uuid.UUID, []models.MovieCast, []models.MovieCrew) error {
	return nil
}
func (f *fakeCreditsRepo) GetMovieCredits(uuid.UUID) ([]models.MovieCast, []models.MovieCrew, error) {
	return f.movieCast, f.movieCrew, nil
}
func (f *fakeCreditsRepo) SetTVShowCredits(uuid.UUID, []models.TVShowCast, []models.TVShowCrew) error {
	return nil
}
func (f *fakeCreditsRepo) GetTVShowCredits(uuid.UUID) ([]models.TVShowCast, []models.TVShowCrew, error) {
	return f.tvCast, f.tvCrew, nil
}

// --- Progress ---

type fakeProgressRepo struct {
	rows              []*models.WatchProgress
	inProgress        []models.WatchProgress
	completedEpisodes []models.WatchProgress
	active            []models.WatchProgress
	byUser            []models.WatchProgress
}

func (f *fakeProgressRepo) match(r *models.WatchProgress, userID, mediaType, mediaID string) bool {
	return r.UserID == userID && r.MediaType == mediaType && r.MediaID == mediaID
}
func (f *fakeProgressRepo) Upsert(p *models.WatchProgress) error {
	for i, r := range f.rows {
		if f.match(r, p.UserID, p.MediaType, p.MediaID) {
			f.rows[i] = p
			return nil
		}
	}
	f.rows = append(f.rows, p)
	return nil
}
func (f *fakeProgressRepo) Find(userID, mediaType, mediaID string) (*models.WatchProgress, error) {
	for _, r := range f.rows {
		if f.match(r, userID, mediaType, mediaID) {
			return r, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeProgressRepo) FindInProgress(string, int) ([]models.WatchProgress, error) {
	return f.inProgress, nil
}
func (f *fakeProgressRepo) FindAllByType(userID, mediaType string) ([]models.WatchProgress, error) {
	out := make([]models.WatchProgress, 0)
	for _, r := range f.rows {
		if r.UserID == userID && r.MediaType == mediaType {
			out = append(out, *r)
		}
	}
	return out, nil
}
func (f *fakeProgressRepo) FindAllActive(time.Time, int) ([]models.WatchProgress, error) {
	return f.active, nil
}
func (f *fakeProgressRepo) FindByUser(string, int) ([]models.WatchProgress, error) {
	return f.byUser, nil
}
func (f *fakeProgressRepo) FindCompletedEpisodes(string) ([]models.WatchProgress, error) {
	return f.completedEpisodes, nil
}
func (f *fakeProgressRepo) Delete(userID, mediaType, mediaID string) error {
	for i, r := range f.rows {
		if f.match(r, userID, mediaType, mediaID) {
			f.rows = append(f.rows[:i], f.rows[i+1:]...)
			return nil
		}
	}
	return nil // deleting a missing row is a no-op, mirroring the GORM repo
}

// --- DismissedNextUp ---

type fakeDismissedRepo struct {
	ids       []string
	deleteErr error
}

func (f *fakeDismissedRepo) Create(_, episodeID string) error {
	f.ids = append(f.ids, episodeID)
	return nil
}
func (f *fakeDismissedRepo) Delete(string, string) error { return f.deleteErr }
func (f *fakeDismissedRepo) ListEpisodeIDs(string) ([]string, error) {
	return f.ids, nil
}
