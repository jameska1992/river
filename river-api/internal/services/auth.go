package services

import (
	"fmt"
	"time"

	"river-api/internal/middleware"
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type AuthService struct {
	users         repository.UserRepository
	refreshTokens repository.RefreshTokenRepository
	jwtSecret     string
	accessExpiry  time.Duration
	refreshExpiry time.Duration
	streamExpiry  time.Duration
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
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	count, err := s.users.Count()
	if err != nil {
		return nil, err
	}

	role := models.RoleUser
	if count == 0 {
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
