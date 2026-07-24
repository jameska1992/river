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
