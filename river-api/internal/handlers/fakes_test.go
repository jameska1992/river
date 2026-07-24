package handlers

import (
	"errors"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/google/uuid"
)

// In-memory repository fakes for the handler tests. Handlers take concrete
// services, so these back a real service via its constructor — exercising
// the full handler -> service -> repo path over httptest.

type fakeUserRepo struct{ users []*models.User }

func (f *fakeUserRepo) Count() (int64, error) { return int64(len(f.users)), nil }
func (f *fakeUserRepo) Create(u *models.User) error {
	for _, e := range f.users {
		if e.Username == u.Username || e.Email == u.Email {
			return errors.New("duplicate") // AuthService maps this to ErrConflict
		}
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	f.users = append(f.users, u)
	return nil
}
func (f *fakeUserRepo) FindByUsername(username string) (*models.User, error) {
	for _, u := range f.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeUserRepo) FindByID(id string) (*models.User, error) {
	for _, u := range f.users {
		if u.ID.String() == id {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeUserRepo) List() ([]models.User, error) {
	out := make([]models.User, 0, len(f.users))
	for _, u := range f.users {
		out = append(out, *u)
	}
	return out, nil
}
func (f *fakeUserRepo) Update(u *models.User) error { return nil }
func (f *fakeUserRepo) UpdatePassword(id, hash string) error {
	for _, u := range f.users {
		if u.ID.String() == id {
			u.PasswordHash = hash
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (f *fakeUserRepo) Delete(id string) error { return nil }

type fakeRefreshRepo struct{ tokens []*models.RefreshToken }

func (f *fakeRefreshRepo) Create(rt *models.RefreshToken) error {
	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	f.tokens = append(f.tokens, rt)
	return nil
}
func (f *fakeRefreshRepo) FindValid(token string, now time.Time) (*models.RefreshToken, error) {
	for _, rt := range f.tokens {
		if rt.Token == token && !rt.Revoked && rt.ExpiresAt.After(now) {
			return rt, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeRefreshRepo) Revoke(id uuid.UUID) error {
	for _, rt := range f.tokens {
		if rt.ID == id {
			rt.Revoked = true
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (f *fakeRefreshRepo) RevokeByToken(token string) error {
	for _, rt := range f.tokens {
		if rt.Token == token {
			rt.Revoked = true
			return nil
		}
	}
	return apperrors.ErrNotFound
}

// fakeMovieRepo — stateful MovieRepository for the movie handler CRUD path
// and collection media lookups.
type fakeMovieRepo struct{ movies []*models.Movie }

func (f *fakeMovieRepo) FindByID(id string) (*models.Movie, error) {
	for _, m := range f.movies {
		if m.ID.String() == id {
			return m, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeMovieRepo) FindAll(string, int, int, string) ([]models.Movie, error) { return nil, nil }
func (f *fakeMovieRepo) Count(string) (int64, error)                              { return 0, nil }
func (f *fakeMovieRepo) FindRecent(int) ([]models.Movie, error)                   { return nil, nil }
func (f *fakeMovieRepo) FindUnidentified() ([]models.Movie, error)                { return nil, nil }
func (f *fakeMovieRepo) Create(m *models.Movie) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	f.movies = append(f.movies, m)
	return nil
}
func (f *fakeMovieRepo) Save(m *models.Movie) error { return nil }
func (f *fakeMovieRepo) Delete(id string) error {
	for i, m := range f.movies {
		if m.ID.String() == id {
			f.movies = append(f.movies[:i], f.movies[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

// fakeShowRepo — stateful TVShowRepository (Create + FindByID meaningful).
type fakeShowRepo struct{ shows []*models.TVShow }

func (f *fakeShowRepo) FindByID(id string) (*models.TVShow, error) {
	for _, s := range f.shows {
		if s.ID.String() == id {
			return s, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeShowRepo) FindAllUnpaged(string) ([]models.TVShow, error)            { return nil, nil }
func (f *fakeShowRepo) FindAll(string, int, int, string) ([]models.TVShow, error) { return nil, nil }
func (f *fakeShowRepo) Count(string) (int64, error)                              { return 0, nil }
func (f *fakeShowRepo) FindRecent(int) ([]models.TVShow, error)                  { return nil, nil }
func (f *fakeShowRepo) FindUnidentified() ([]models.TVShow, error)               { return nil, nil }
func (f *fakeShowRepo) Create(s *models.TVShow) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	f.shows = append(f.shows, s)
	return nil
}
func (f *fakeShowRepo) Save(*models.TVShow) error { return nil }
func (f *fakeShowRepo) Delete(id string) error {
	for i, s := range f.shows {
		if s.ID.String() == id {
			f.shows = append(f.shows[:i], f.shows[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

// fakeWatchlistRepo — in-memory WatchlistRepository.
type fakeWatchlistRepo struct{ items []*models.WatchlistItem }

func (f *fakeWatchlistRepo) Add(userID, mediaType, mediaID string) (*models.WatchlistItem, error) {
	for _, it := range f.items {
		if it.UserID == userID && it.MediaType == mediaType && it.MediaID == mediaID {
			return it, nil
		}
	}
	it := &models.WatchlistItem{Base: models.Base{ID: uuid.New()}, UserID: userID, MediaType: mediaType, MediaID: mediaID}
	f.items = append(f.items, it)
	return it, nil
}
func (f *fakeWatchlistRepo) Remove(userID, itemID string) error {
	for i, it := range f.items {
		if it.UserID == userID && it.ID.String() == itemID {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (f *fakeWatchlistRepo) List(userID string) ([]models.WatchlistItem, error) {
	out := make([]models.WatchlistItem, 0)
	for _, it := range f.items {
		if it.UserID == userID {
			out = append(out, *it)
		}
	}
	return out, nil
}

// fakeSearchRepo — SearchRepository returning seeded results (no filtering).
type fakeSearchRepo struct {
	movies []models.Movie
	shows  []models.TVShow
	books  []models.Audiobook
	people []models.Person
}

func (f *fakeSearchRepo) SearchMovies(q, g string, l int) ([]models.Movie, error) {
	return f.movies, nil
}
func (f *fakeSearchRepo) SearchTVShows(q, g string, l int) ([]models.TVShow, error) {
	return f.shows, nil
}
func (f *fakeSearchRepo) SearchAudiobooks(q, g string, l int) ([]models.Audiobook, error) {
	return f.books, nil
}
func (f *fakeSearchRepo) SearchPeople(q string, l int) ([]models.Person, error) {
	return f.people, nil
}

// fakeCleanupRepo — MediaCleanupRepository no-op for handler delete paths.
type fakeCleanupRepo struct{}

func (fakeCleanupRepo) PurgeMovie(string) error     { return nil }
func (fakeCleanupRepo) PurgeShow(string) error      { return nil }
func (fakeCleanupRepo) PurgeEpisode(string) error   { return nil }
func (fakeCleanupRepo) PurgeAudiobook(string) error { return nil }
func (fakeCleanupRepo) PurgeChapter(string) error   { return nil }

// fakeCollectionRepo — in-memory CollectionRepository.
type fakeCollectionRepo struct {
	cols  []*models.Collection
	items []*models.CollectionItem
}

func (f *fakeCollectionRepo) FindAll() ([]models.Collection, error) {
	out := make([]models.Collection, 0, len(f.cols))
	for _, c := range f.cols {
		out = append(out, *c)
	}
	return out, nil
}
func (f *fakeCollectionRepo) FindByID(id string) (*models.Collection, error) {
	for _, c := range f.cols {
		if c.ID.String() == id {
			return c, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeCollectionRepo) Create(c *models.Collection) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	f.cols = append(f.cols, c)
	return nil
}
func (f *fakeCollectionRepo) Save(c *models.Collection) error { return nil }
func (f *fakeCollectionRepo) Delete(id string) error {
	for i, c := range f.cols {
		if c.ID.String() == id {
			f.cols = append(f.cols[:i], f.cols[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (f *fakeCollectionRepo) FindItems(collectionID string) ([]models.CollectionItem, error) {
	out := make([]models.CollectionItem, 0)
	for _, it := range f.items {
		if it.CollectionID == collectionID {
			out = append(out, *it)
		}
	}
	return out, nil
}
func (f *fakeCollectionRepo) FindItem(id string) (*models.CollectionItem, error) {
	for _, it := range f.items {
		if it.ID.String() == id {
			return it, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeCollectionRepo) FindItemByMedia(collectionID, mediaType, mediaID string) (*models.CollectionItem, error) {
	for _, it := range f.items {
		if it.CollectionID == collectionID && it.MediaType == mediaType && it.MediaID == mediaID {
			return it, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (f *fakeCollectionRepo) AddItem(item *models.CollectionItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	f.items = append(f.items, item)
	return nil
}
func (f *fakeCollectionRepo) RemoveItem(id string) error {
	for i, it := range f.items {
		if it.ID.String() == id {
			f.items = append(f.items[:i], f.items[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}
