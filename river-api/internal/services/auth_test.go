package services

import (
	"errors"
	"testing"
	"time"

	"river-api/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func newTestAuthService(users *memUserRepo, refresh *memRefreshRepo) *AuthService {
	return NewAuthService(users, refresh, "test-secret", 15*time.Minute, 7*24*time.Hour, 8*time.Hour)
}

func TestRegister_FirstUserBecomesAdmin(t *testing.T) {
	users := &memUserRepo{}
	svc := newTestAuthService(users, &memRefreshRepo{})

	first, err := svc.Register("admin", "admin@x.com", "pw")
	require.NoError(t, err)
	assert.Equal(t, models.RoleAdmin, first.Role, "first registered user should be admin")

	second, err := svc.Register("bob", "bob@x.com", "pw")
	require.NoError(t, err)
	assert.Equal(t, models.RoleUser, second.Role, "subsequent users should be plain users")
}

func TestRegister_HashesPassword(t *testing.T) {
	svc := newTestAuthService(&memUserRepo{}, &memRefreshRepo{})

	u, err := svc.Register("admin", "admin@x.com", "secret")
	require.NoError(t, err)
	assert.NotEqual(t, "secret", u.PasswordHash, "password must not be stored in plaintext")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("secret")))
}

func TestRegister_ConflictOnCreateError(t *testing.T) {
	users := &memUserRepo{createErr: errors.New("unique violation")}
	svc := newTestAuthService(users, &memRefreshRepo{})

	_, err := svc.Register("admin", "admin@x.com", "pw")
	assert.ErrorIs(t, err, ErrConflict)
}

func TestLogin(t *testing.T) {
	users := &memUserRepo{}
	svc := newTestAuthService(users, &memRefreshRepo{})
	_, err := svc.Register("admin", "admin@x.com", "pw")
	require.NoError(t, err)

	t.Run("unknown username is unauthorized", func(t *testing.T) {
		_, err := svc.Login("nobody", "pw")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("wrong password is unauthorized", func(t *testing.T) {
		_, err := svc.Login("admin", "wrong")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("valid credentials issue a full token triple", func(t *testing.T) {
		res, err := svc.Login("admin", "pw")
		require.NoError(t, err)
		assert.NotEmpty(t, res.AccessToken)
		assert.NotEmpty(t, res.RefreshToken)
		assert.NotEmpty(t, res.StreamToken)
		assert.Equal(t, "admin", res.User.Username)
	})
}

func TestRefresh_RotatesAndRevokesOldToken(t *testing.T) {
	users := &memUserRepo{}
	svc := newTestAuthService(users, &memRefreshRepo{})
	_, err := svc.Register("admin", "admin@x.com", "pw")
	require.NoError(t, err)
	login, err := svc.Login("admin", "pw")
	require.NoError(t, err)

	rotated, err := svc.Refresh(login.RefreshToken)
	require.NoError(t, err)
	assert.NotEmpty(t, rotated.RefreshToken)
	assert.NotEqual(t, login.RefreshToken, rotated.RefreshToken, "refresh should rotate the token")

	// The original token is revoked on use — replaying it must fail.
	_, err = svc.Refresh(login.RefreshToken)
	assert.ErrorIs(t, err, ErrUnauthorized)

	// The rotated token is still valid.
	_, err = svc.Refresh(rotated.RefreshToken)
	assert.NoError(t, err)
}

func TestRefresh_InvalidTokenIsUnauthorized(t *testing.T) {
	svc := newTestAuthService(&memUserRepo{}, &memRefreshRepo{})
	_, err := svc.Refresh("does-not-exist")
	assert.ErrorIs(t, err, ErrUnauthorized)
}

func TestLogout_RevokesToken(t *testing.T) {
	users := &memUserRepo{}
	svc := newTestAuthService(users, &memRefreshRepo{})
	_, err := svc.Register("admin", "admin@x.com", "pw")
	require.NoError(t, err)
	login, err := svc.Login("admin", "pw")
	require.NoError(t, err)

	svc.Logout(login.RefreshToken)

	_, err = svc.Refresh(login.RefreshToken)
	assert.ErrorIs(t, err, ErrUnauthorized, "a logged-out token can no longer be refreshed")
}

func TestChangePassword(t *testing.T) {
	users := &memUserRepo{}
	svc := newTestAuthService(users, &memRefreshRepo{})
	u, err := svc.Register("admin", "admin@x.com", "oldpass")
	require.NoError(t, err)

	t.Run("wrong current password is rejected", func(t *testing.T) {
		err := svc.ChangePassword(u.ID.String(), "wrong", "newpass")
		assert.ErrorIs(t, err, ErrUnauthorized)
	})

	t.Run("correct current password updates the hash", func(t *testing.T) {
		err := svc.ChangePassword(u.ID.String(), "oldpass", "newpass")
		require.NoError(t, err)

		_, err = svc.Login("admin", "oldpass")
		assert.ErrorIs(t, err, ErrUnauthorized, "old password should no longer work")
		_, err = svc.Login("admin", "newpass")
		assert.NoError(t, err, "new password should work")
	})
}
