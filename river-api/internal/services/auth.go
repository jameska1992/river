package services

import (
	"errors"
	"fmt"
	"time"

	"river-api/internal/middleware"
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// RegistrationPolicy reports whether self-signup is currently allowed. It's an
// interface so the auth service doesn't depend on the settings service directly;
// SettingsService satisfies it via AllowRegistration().
type RegistrationPolicy interface {
	AllowRegistration() bool
}

type AuthService struct {
	users         repository.UserRepository
	refreshTokens repository.RefreshTokenRepository
	jwtSecret     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	streamExpiry  time.Duration
	// registration gates self-signup. Optional: when nil, registration is
	// always allowed (backwards-compatible default).
	registration RegistrationPolicy
}

// SetRegistrationPolicy wires the registration gate (called from main after the
// settings service is built). A nil policy leaves registration open.
func (s *AuthService) SetRegistrationPolicy(p RegistrationPolicy) {
	s.registration = p
}

// registrationEnabled is the raw admin toggle (nil policy → open).
func (s *AuthService) registrationEnabled() bool {
	return s.registration == nil || s.registration.AllowRegistration()
}

// RegistrationAllowed reports the effective self-signup state for a would-be
// registrant: the admin toggle, OR always true while no admin exists yet
// (bootstrap). Used by the public registration-status endpoint so the web app
// can show/hide the sign-up form. Fails open if the admin count can't be read.
func (s *AuthService) RegistrationAllowed() bool {
	adminCount, err := s.users.CountByRole(models.RoleAdmin)
	if err != nil {
		return true
	}
	return adminCount == 0 || s.registrationEnabled()
}

func NewAuthService(
	users repository.UserRepository,
	refreshTokens repository.RefreshTokenRepository,
	secret string,
	accessExpiry, refreshExpiry, streamExpiry time.Duration,
) *AuthService {
	return &AuthService{
		users:         users,
		refreshTokens: refreshTokens,
		jwtSecret:     secret,
		accessExpiry:  accessExpiry,
		refreshExpiry: refreshExpiry,
		streamExpiry:  streamExpiry,
	}
}

type TokenPair struct {
	AccessToken  string
	RefreshToken string
	// StreamToken is a longer-lived JWT restricted (server-side) to media
	// /stream and /download endpoints. The frontend embeds it in <video>
	// src URLs so playback survives longer than a single access-token TTL.
	StreamToken string
}

type LoginResult struct {
	TokenPair
	User models.User
}

func (s *AuthService) Register(username, email, password string) (*models.User, error) {
	// The first human to register becomes admin. Key this off "no admin
	// exists yet" rather than "no users at all", so a boot-seeded service
	// account (role=service) doesn't consume the admin bootstrap and leave
	// the first real user as a plain user.
	adminCount, err := s.users.CountByRole(models.RoleAdmin)
	if err != nil {
		return nil, err
	}

	// Gate self-signup when the admin has disabled it — but never block the
	// very first admin (adminCount == 0), so a fresh install can always be
	// bootstrapped and an operator can't lock themselves out.
	if adminCount > 0 && !s.registrationEnabled() {
		return nil, ErrRegistrationDisabled
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	role := models.RoleUser
	if adminCount == 0 {
		role = models.RoleAdmin
	}

	user := models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         role,
	}
	if err := s.users.Create(&user); err != nil {
		return nil, ErrConflict
	}
	return &user, nil
}

// SeedServiceUser idempotently ensures a least-privilege service account
// (role=service) exists for the internal services to authenticate with.
// Create-if-absent: an existing account (matched by username) is left
// untouched, so re-running on every boot / upgrade never rotates a changed
// password or destroys anything. A no-op when username or password is empty
// (service auth not configured — services fall back to their admin creds).
func (s *AuthService) SeedServiceUser(username, password string) error {
	if username == "" || password == "" {
		return nil
	}
	if _, err := s.users.FindByUsername(username); err == nil {
		return nil // already exists — leave it as-is
	} else if !errors.Is(err, ErrNotFound) {
		return err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash service password: %w", err)
	}
	// Synthetic unique email satisfies the not-null/unique email constraint
	// without colliding with a human account.
	user := models.User{
		Username:     username,
		Email:        username + "@service.river.local",
		PasswordHash: string(hash),
		Role:         models.RoleService,
	}
	if err := s.users.Create(&user); err != nil {
		return ErrConflict
	}
	return nil
}

func (s *AuthService) Login(username, password string) (*LoginResult, error) {
	user, err := s.users.FindByUsername(username)
	if err != nil {
		return nil, ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return nil, ErrUnauthorized
	}
	tokens, err := s.issueTokenPair(*user)
	if err != nil {
		return nil, err
	}
	return &LoginResult{TokenPair: *tokens, User: *user}, nil
}

func (s *AuthService) Refresh(rawToken string) (*TokenPair, error) {
	rt, err := s.refreshTokens.FindValid(rawToken, time.Now())
	if err != nil {
		return nil, ErrUnauthorized
	}

	user, err := s.users.FindByID(rt.UserID.String())
	if err != nil {
		return nil, ErrUnauthorized
	}

	if err := s.refreshTokens.Revoke(rt.ID); err != nil {
		return nil, err
	}

	return s.issueTokenPair(*user)
}

func (s *AuthService) Logout(rawToken string) {
	_ = s.refreshTokens.RevokeByToken(rawToken)
}

func (s *AuthService) GetUser(id string) (*models.User, error) {
	return s.users.FindByID(id)
}

func (s *AuthService) ListUsers() ([]models.User, error) {
	return s.users.List()
}

func (s *AuthService) AdminCreateUser(username, email, password, role string) (*models.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	user := models.User{
		Username:     username,
		Email:        email,
		PasswordHash: string(hash),
		Role:         models.Role(role),
	}
	if err := s.users.Create(&user); err != nil {
		return nil, ErrConflict
	}
	return &user, nil
}

func (s *AuthService) UpdateMe(id, email string) (*models.User, error) {
	user, err := s.users.FindByID(id)
	if err != nil {
		return nil, err
	}
	user.Email = email
	if err := s.users.Update(user); err != nil {
		return nil, ErrConflict
	}
	return user, nil
}

func (s *AuthService) ChangePassword(id, currentPassword, newPassword string) error {
	user, err := s.users.FindByID(id)
	if err != nil {
		return err
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrUnauthorized
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.users.UpdatePassword(id, string(hash))
}

func (s *AuthService) UpdateUser(id, username, email, role string) (*models.User, error) {
	user, err := s.users.FindByID(id)
	if err != nil {
		return nil, err
	}
	user.Username = username
	user.Email = email
	user.Role = models.Role(role)
	if err := s.users.Update(user); err != nil {
		return nil, ErrConflict
	}
	return user, nil
}

func (s *AuthService) SetPassword(id, newPassword string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	return s.users.UpdatePassword(id, string(hash))
}

func (s *AuthService) DeleteUser(id string) error {
	return s.users.Delete(id)
}

func (s *AuthService) issueTokenPair(user models.User) (*TokenPair, error) {
	now := time.Now()
	baseClaims := middleware.Claims{
		UserID:   user.ID.String(),
		Username: user.Username,
		Role:     string(user.Role),
	}

	accessClaims := baseClaims
	accessClaims.TokenType = middleware.TokenTypeAccess
	accessClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(s.accessExpiry)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, accessClaims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("sign access token: %w", err)
	}

	streamClaims := baseClaims
	streamClaims.TokenType = middleware.TokenTypeStream
	streamClaims.RegisteredClaims = jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(now.Add(s.streamExpiry)),
		IssuedAt:  jwt.NewNumericDate(now),
	}
	stream, err := jwt.NewWithClaims(jwt.SigningMethodHS256, streamClaims).SignedString([]byte(s.jwtSecret))
	if err != nil {
		return nil, fmt.Errorf("sign stream token: %w", err)
	}

	rt := models.RefreshToken{
		UserID:    user.ID,
		Token:     uuid.New().String(),
		ExpiresAt: now.Add(s.refreshExpiry),
	}
	if err := s.refreshTokens.Create(&rt); err != nil {
		return nil, fmt.Errorf("store refresh token: %w", err)
	}

	return &TokenPair{
		AccessToken:  access,
		RefreshToken: rt.Token,
		StreamToken:  stream,
	}, nil
}
