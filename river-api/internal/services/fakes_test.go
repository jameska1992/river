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
