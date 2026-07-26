package services

import (
	"errors"
	"testing"

	"river-api/internal/models"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

func seededUser(username, role string) *models.User {
	return &models.User{
		Base:     models.Base{ID: uuid.New()},
		Username: username,
		Email:    username + "@example.com",
		Role:     models.Role(role),
	}
}

func TestAuthService_GetUser(t *testing.T) {
	u := seededUser("alice", "admin")
	svc := newTestAuthService(&memUserRepo{users: []*models.User{u}}, &memRefreshRepo{})

	got, err := svc.GetUser(u.ID.String())
	require.NoError(t, err)
	assert.Equal(t, "alice", got.Username)

	_, err = svc.GetUser(uuid.NewString())
	assert.Error(t, err, "unknown id should error")
}

func TestAuthService_ListUsers(t *testing.T) {
	svc := newTestAuthService(&memUserRepo{users: []*models.User{
		seededUser("alice", "admin"), seededUser("bob", "user"),
	}}, &memRefreshRepo{})

	users, err := svc.ListUsers()
	require.NoError(t, err)
	assert.Len(t, users, 2)
}

func TestAuthService_AdminCreateUser(t *testing.T) {
	users := &memUserRepo{}
	svc := newTestAuthService(users, &memRefreshRepo{})

	u, err := svc.AdminCreateUser("carol", "carol@example.com", "s3cret", "user")
	require.NoError(t, err)
	assert.Equal(t, "carol", u.Username)
	assert.Equal(t, models.Role("user"), u.Role)
	// The stored hash must verify against the supplied password (not stored plaintext).
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("s3cret")))
	assert.Len(t, users.users, 1)
}

func TestAuthService_AdminCreateUser_ConflictOnCreateError(t *testing.T) {
	users := &memUserRepo{createErr: errors.New("unique violation")}
	svc := newTestAuthService(users, &memRefreshRepo{})

	_, err := svc.AdminCreateUser("dupe", "dupe@example.com", "pw", "user")
	assert.ErrorIs(t, err, ErrConflict)
}

func TestAuthService_UpdateMe(t *testing.T) {
	u := seededUser("erin", "user")
	users := &memUserRepo{users: []*models.User{u}}
	svc := newTestAuthService(users, &memRefreshRepo{})

	got, err := svc.UpdateMe(u.ID.String(), "new@example.com")
	require.NoError(t, err)
	assert.Equal(t, "new@example.com", got.Email)

	_, err = svc.UpdateMe(uuid.NewString(), "x@example.com")
	assert.Error(t, err, "unknown id should surface not-found")
}

func TestAuthService_UpdateMe_ConflictOnUpdateError(t *testing.T) {
	u := seededUser("frank", "user")
	users := &memUserRepo{users: []*models.User{u}, updateErr: errors.New("email taken")}
	svc := newTestAuthService(users, &memRefreshRepo{})

	_, err := svc.UpdateMe(u.ID.String(), "taken@example.com")
	assert.ErrorIs(t, err, ErrConflict)
}

func TestAuthService_UpdateUser(t *testing.T) {
	u := seededUser("grace", "user")
	users := &memUserRepo{users: []*models.User{u}}
	svc := newTestAuthService(users, &memRefreshRepo{})

	got, err := svc.UpdateUser(u.ID.String(), "grace2", "grace2@example.com", "admin")
	require.NoError(t, err)
	assert.Equal(t, "grace2", got.Username)
	assert.Equal(t, "grace2@example.com", got.Email)
	assert.Equal(t, models.Role("admin"), got.Role)

	_, err = svc.UpdateUser(uuid.NewString(), "x", "x@example.com", "user")
	assert.Error(t, err)
}

func TestAuthService_UpdateUser_ConflictOnUpdateError(t *testing.T) {
	u := seededUser("heidi", "user")
	users := &memUserRepo{users: []*models.User{u}, updateErr: errors.New("username taken")}
	svc := newTestAuthService(users, &memRefreshRepo{})

	_, err := svc.UpdateUser(u.ID.String(), "clash", "clash@example.com", "user")
	assert.ErrorIs(t, err, ErrConflict)
}

func TestAuthService_SetPassword(t *testing.T) {
	u := seededUser("ivan", "user")
	users := &memUserRepo{users: []*models.User{u}}
	svc := newTestAuthService(users, &memRefreshRepo{})

	require.NoError(t, svc.SetPassword(u.ID.String(), "brand-new-pw"))
	// UpdatePassword mutates the stored user's hash — verify it matches.
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("brand-new-pw")))

	// Unknown id: the fake's UpdatePassword returns not-found.
	assert.Error(t, svc.SetPassword(uuid.NewString(), "pw"))
}

func TestAuthService_DeleteUser(t *testing.T) {
	u := seededUser("judy", "user")
	users := &memUserRepo{users: []*models.User{u}}
	svc := newTestAuthService(users, &memRefreshRepo{})

	require.NoError(t, svc.DeleteUser(u.ID.String()))
	assert.Empty(t, users.users)

	assert.Error(t, svc.DeleteUser(uuid.NewString()), "deleting a missing user errors")
}
