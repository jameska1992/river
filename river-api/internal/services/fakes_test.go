package services

import (
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"

	"github.com/google/uuid"
)

// In-memory repository fakes used across the service unit tests. They
// implement the repository interfaces without a database so services can
// be exercised in isolation.

type memUserRepo struct {
	users     []*models.User
	createErr error
}

func (m *memUserRepo) Count() (int64, error) { return int64(len(m.users)), nil }

func (m *memUserRepo) Create(u *models.User) error {
	if m.createErr != nil {
		return m.createErr
	}
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	m.users = append(m.users, u)
	return nil
}

func (m *memUserRepo) FindByUsername(username string) (*models.User, error) {
	for _, u := range m.users {
		if u.Username == username {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memUserRepo) FindByID(id string) (*models.User, error) {
	for _, u := range m.users {
		if u.ID.String() == id {
			return u, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memUserRepo) List() ([]models.User, error) {
	out := make([]models.User, 0, len(m.users))
	for _, u := range m.users {
		out = append(out, *u)
	}
	return out, nil
}

func (m *memUserRepo) Update(u *models.User) error {
	for i, e := range m.users {
		if e.ID == u.ID {
			m.users[i] = u
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func (m *memUserRepo) UpdatePassword(id, hash string) error {
	for _, u := range m.users {
		if u.ID.String() == id {
			u.PasswordHash = hash
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func (m *memUserRepo) Delete(id string) error {
	for i, u := range m.users {
		if u.ID.String() == id {
			m.users = append(m.users[:i], m.users[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

type memRefreshRepo struct {
	tokens []*models.RefreshToken
}

func (m *memRefreshRepo) Create(rt *models.RefreshToken) error {
	if rt.ID == uuid.Nil {
		rt.ID = uuid.New()
	}
	m.tokens = append(m.tokens, rt)
	return nil
}

func (m *memRefreshRepo) FindValid(token string, now time.Time) (*models.RefreshToken, error) {
	for _, rt := range m.tokens {
		if rt.Token == token && !rt.Revoked && rt.ExpiresAt.After(now) {
			return rt, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memRefreshRepo) Revoke(id uuid.UUID) error {
	for _, rt := range m.tokens {
		if rt.ID == id {
			rt.Revoked = true
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func (m *memRefreshRepo) RevokeByToken(token string) error {
	for _, rt := range m.tokens {
		if rt.Token == token {
			rt.Revoked = true
			return nil
		}
	}
	return apperrors.ErrNotFound
}

type memLibraryRepo struct {
	libs []*models.Library
}

func (m *memLibraryRepo) FindAll() ([]models.Library, error) {
	out := make([]models.Library, 0, len(m.libs))
	for _, l := range m.libs {
		out = append(out, *l)
	}
	return out, nil
}

func (m *memLibraryRepo) FindByID(id string) (*models.Library, error) {
	for _, l := range m.libs {
		if l.ID.String() == id {
			return l, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memLibraryRepo) Create(lib *models.Library) error {
	if lib.ID == uuid.Nil {
		lib.ID = uuid.New()
	}
	m.libs = append(m.libs, lib)
	return nil
}

func (m *memLibraryRepo) Save(lib *models.Library) error {
	for i, l := range m.libs {
		if l.ID == lib.ID {
			m.libs[i] = lib
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func (m *memLibraryRepo) Delete(id string) error {
	for i, l := range m.libs {
		if l.ID.String() == id {
			m.libs = append(m.libs[:i], m.libs[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

type memProgressRepo struct {
	rows []*models.WatchProgress
}

func (m *memProgressRepo) match(r *models.WatchProgress, userID, mediaType, mediaID string) bool {
	return r.UserID == userID && r.MediaType == mediaType && r.MediaID == mediaID
}

func (m *memProgressRepo) Upsert(p *models.WatchProgress) error {
	for i, r := range m.rows {
		if m.match(r, p.UserID, p.MediaType, p.MediaID) {
			m.rows[i] = p
			return nil
		}
	}
	m.rows = append(m.rows, p)
	return nil
}

func (m *memProgressRepo) Find(userID, mediaType, mediaID string) (*models.WatchProgress, error) {
	for _, r := range m.rows {
		if m.match(r, userID, mediaType, mediaID) {
			return r, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

// Delete mirrors the GORM repo: deleting a missing row is a no-op, not an error.
func (m *memProgressRepo) Delete(userID, mediaType, mediaID string) error {
	for i, r := range m.rows {
		if m.match(r, userID, mediaType, mediaID) {
			m.rows = append(m.rows[:i], m.rows[i+1:]...)
			return nil
		}
	}
	return nil
}

func (m *memProgressRepo) FindAllByType(userID, mediaType string) ([]models.WatchProgress, error) {
	out := make([]models.WatchProgress, 0)
	for _, r := range m.rows {
		if r.UserID == userID && r.MediaType == mediaType {
			out = append(out, *r)
		}
	}
	return out, nil
}

func (m *memProgressRepo) FindInProgress(userID string, limit int) ([]models.WatchProgress, error) {
	return nil, nil
}
func (m *memProgressRepo) FindAllActive(since time.Time, limit int) ([]models.WatchProgress, error) {
	return nil, nil
}
func (m *memProgressRepo) FindByUser(userID string, limit int) ([]models.WatchProgress, error) {
	return nil, nil
}
func (m *memProgressRepo) FindCompletedEpisodes(userID string) ([]models.WatchProgress, error) {
	return nil, nil
}

type memEpisodeRepo struct {
	episodes []*models.Episode
}

func (m *memEpisodeRepo) FindByShowID(showID string) ([]models.Episode, error) {
	out := make([]models.Episode, 0)
	for _, e := range m.episodes {
		if e.TVShowID.String() == showID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (m *memEpisodeRepo) FindByID(id string) (*models.Episode, error) {
	for _, e := range m.episodes {
		if e.ID.String() == id {
			return e, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memEpisodeRepo) FindBySeasonID(seasonID string) ([]models.Episode, error) {
	out := make([]models.Episode, 0)
	for _, e := range m.episodes {
		if e.SeasonID.String() == seasonID {
			out = append(out, *e)
		}
	}
	return out, nil
}

func (m *memEpisodeRepo) FindBySeasonAndNumber(seasonID string, number int, isSpecial bool) (*models.Episode, error) {
	for _, e := range m.episodes {
		if e.SeasonID.String() == seasonID && e.Number == number {
			return e, nil
		}
	}
	return nil, apperrors.ErrNotFound
}

func (m *memEpisodeRepo) Create(e *models.Episode) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	m.episodes = append(m.episodes, e)
	return nil
}

func (m *memEpisodeRepo) Save(e *models.Episode) error {
	for i, ex := range m.episodes {
		if ex.ID == e.ID {
			m.episodes[i] = e
			return nil
		}
	}
	return apperrors.ErrNotFound
}

func (m *memEpisodeRepo) Delete(id string) error {
	for i, e := range m.episodes {
		if e.ID.String() == id {
			m.episodes = append(m.episodes[:i], m.episodes[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

// Media repository fakes. Only FindByID carries behaviour (seeded via the
// slice); the remaining interface methods are stubbed since the services
// under test here only look media up by ID.

type memMovieRepo struct{ movies []*models.Movie }

func (m *memMovieRepo) FindByID(id string) (*models.Movie, error) {
	for _, x := range m.movies {
		if x.ID.String() == id {
			return x, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memMovieRepo) FindAll(string, int, int, string) ([]models.Movie, error) { return nil, nil }
func (m *memMovieRepo) Count(string) (int64, error)                              { return 0, nil }
func (m *memMovieRepo) FindRecent(int) ([]models.Movie, error)                   { return nil, nil }
func (m *memMovieRepo) FindUnidentified() ([]models.Movie, error)                { return nil, nil }
func (m *memMovieRepo) Create(x *models.Movie) error {
	if x.ID == uuid.Nil {
		x.ID = uuid.New()
	}
	m.movies = append(m.movies, x)
	return nil
}
func (m *memMovieRepo) Save(x *models.Movie) error {
	for i, e := range m.movies {
		if e.ID == x.ID {
			m.movies[i] = x
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memMovieRepo) Delete(id string) error {
	for i, e := range m.movies {
		if e.ID.String() == id {
			m.movies = append(m.movies[:i], m.movies[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

type memShowRepo struct{ shows []*models.TVShow }

func (m *memShowRepo) FindByID(id string) (*models.TVShow, error) {
	for _, x := range m.shows {
		if x.ID.String() == id {
			return x, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memShowRepo) FindAllUnpaged(string) ([]models.TVShow, error)            { return nil, nil }
func (m *memShowRepo) FindAll(string, int, int, string) ([]models.TVShow, error) { return nil, nil }
func (m *memShowRepo) Count(string) (int64, error)                              { return 0, nil }
func (m *memShowRepo) FindRecent(int) ([]models.TVShow, error)                  { return nil, nil }
func (m *memShowRepo) FindUnidentified() ([]models.TVShow, error)               { return nil, nil }
func (m *memShowRepo) Create(x *models.TVShow) error {
	if x.ID == uuid.Nil {
		x.ID = uuid.New()
	}
	m.shows = append(m.shows, x)
	return nil
}
func (m *memShowRepo) Save(x *models.TVShow) error {
	for i, e := range m.shows {
		if e.ID == x.ID {
			m.shows[i] = x
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memShowRepo) Delete(id string) error {
	for i, e := range m.shows {
		if e.ID.String() == id {
			m.shows = append(m.shows[:i], m.shows[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

// memCleanupRepo fakes MediaCleanupRepository. Per-type error hooks let a
// test drive the "purge fails → row is not deleted" path; purged records
// the ids that were successfully purged.
type memCleanupRepo struct {
	movieErr, showErr, episodeErr error
	purged                        []string
}

func (m *memCleanupRepo) PurgeMovie(id string) error {
	if m.movieErr != nil {
		return m.movieErr
	}
	m.purged = append(m.purged, "movie:"+id)
	return nil
}
func (m *memCleanupRepo) PurgeShow(id string) error {
	if m.showErr != nil {
		return m.showErr
	}
	m.purged = append(m.purged, "show:"+id)
	return nil
}
func (m *memCleanupRepo) PurgeEpisode(id string) error {
	if m.episodeErr != nil {
		return m.episodeErr
	}
	m.purged = append(m.purged, "episode:"+id)
	return nil
}
func (m *memCleanupRepo) PurgeAudiobook(id string) error { return nil }
func (m *memCleanupRepo) PurgeChapter(id string) error   { return nil }

type memAudiobookRepo struct{ books []*models.Audiobook }

func (m *memAudiobookRepo) FindByID(id string) (*models.Audiobook, error) {
	for _, x := range m.books {
		if x.ID.String() == id {
			return x, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memAudiobookRepo) FindAll(string, int, int, string) ([]models.Audiobook, error) {
	return nil, nil
}
func (m *memAudiobookRepo) Count(string) (int64, error)    { return 0, nil }
func (m *memAudiobookRepo) Create(*models.Audiobook) error { return nil }
func (m *memAudiobookRepo) Save(*models.Audiobook) error   { return nil }
func (m *memAudiobookRepo) Delete(string) error            { return nil }

type memCollectionRepo struct {
	cols  []*models.Collection
	items []*models.CollectionItem
}

func (m *memCollectionRepo) FindAll() ([]models.Collection, error) {
	out := make([]models.Collection, 0, len(m.cols))
	for _, c := range m.cols {
		out = append(out, *c)
	}
	return out, nil
}
func (m *memCollectionRepo) FindByID(id string) (*models.Collection, error) {
	for _, c := range m.cols {
		if c.ID.String() == id {
			return c, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memCollectionRepo) Create(c *models.Collection) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	m.cols = append(m.cols, c)
	return nil
}
func (m *memCollectionRepo) Save(c *models.Collection) error { return nil }
func (m *memCollectionRepo) Delete(id string) error {
	for i, c := range m.cols {
		if c.ID.String() == id {
			m.cols = append(m.cols[:i], m.cols[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memCollectionRepo) FindItems(collectionID string) ([]models.CollectionItem, error) {
	out := make([]models.CollectionItem, 0)
	for _, it := range m.items {
		if it.CollectionID == collectionID {
			out = append(out, *it)
		}
	}
	return out, nil
}
func (m *memCollectionRepo) FindItem(id string) (*models.CollectionItem, error) {
	for _, it := range m.items {
		if it.ID.String() == id {
			return it, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memCollectionRepo) FindItemByMedia(collectionID, mediaType, mediaID string) (*models.CollectionItem, error) {
	for _, it := range m.items {
		if it.CollectionID == collectionID && it.MediaType == mediaType && it.MediaID == mediaID {
			return it, nil
		}
	}
	return nil, apperrors.ErrNotFound
}
func (m *memCollectionRepo) AddItem(item *models.CollectionItem) error {
	if item.ID == uuid.Nil {
		item.ID = uuid.New()
	}
	m.items = append(m.items, item)
	return nil
}
func (m *memCollectionRepo) RemoveItem(id string) error {
	for i, it := range m.items {
		if it.ID.String() == id {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}

type memWatchlistRepo struct {
	items []*models.WatchlistItem
}

func (m *memWatchlistRepo) Add(userID, mediaType, mediaID string) (*models.WatchlistItem, error) {
	for _, it := range m.items {
		if it.UserID == userID && it.MediaType == mediaType && it.MediaID == mediaID {
			return it, nil // already present — idempotent, matches the GORM repo
		}
	}
	it := &models.WatchlistItem{Base: models.Base{ID: uuid.New()}, UserID: userID, MediaType: mediaType, MediaID: mediaID}
	m.items = append(m.items, it)
	return it, nil
}
func (m *memWatchlistRepo) Remove(userID, itemID string) error {
	for i, it := range m.items {
		if it.UserID == userID && it.ID.String() == itemID {
			m.items = append(m.items[:i], m.items[i+1:]...)
			return nil
		}
	}
	return apperrors.ErrNotFound
}
func (m *memWatchlistRepo) List(userID string) ([]models.WatchlistItem, error) {
	out := make([]models.WatchlistItem, 0)
	for _, it := range m.items {
		if it.UserID == userID {
			out = append(out, *it)
		}
	}
	return out, nil
}
