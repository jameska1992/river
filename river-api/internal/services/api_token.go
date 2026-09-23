package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"

	"github.com/google/uuid"
)

// APITokenPrefix marks a bearer token as a user API token (vs a JWT or an
// rvk_ service key). The auth middleware routes on it.
const APITokenPrefix = "rvat_"

// APITokenService mints and authenticates long-lived user API tokens for
// external tools. v1 tokens are read-only: an authenticated token principal has
// role "user", so it can read the API but the existing role checks block writes
// and the admin surface. Scopes are stored for forward compatibility.
type APITokenService struct {
	repo repository.APITokenRepository

	mu        sync.Mutex
	lastTouch map[string]time.Time
}

func NewAPITokenService(repo repository.APITokenRepository) *APITokenService {
	return &APITokenService{repo: repo, lastTouch: make(map[string]time.Time)}
}

type APITokenView struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      []string   `json:"scopes"`
	Revoked     bool       `json:"revoked"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func toAPITokenView(t models.APIToken) APITokenView {
	return APITokenView{
		ID: t.ID.String(), Name: t.Name, TokenPrefix: t.TokenPrefix,
		Scopes: parseScopes(t.Scopes), Revoked: t.Revoked, LastUsedAt: t.LastUsedAt,
		ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	}
}

func (s *APITokenService) List() ([]APITokenView, error) {
	toks, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]APITokenView, len(toks))
	for i, t := range toks {
		out[i] = toAPITokenView(t)
	}
	return out, nil
}

// Mint creates a token owned by userID, returning the plaintext once. expiresAt
// is optional (nil = never expires until revoked).
func (s *APITokenService) Mint(userID uuid.UUID, name string, scopes []string, expiresAt *time.Time) (plaintext string, view APITokenView, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", APITokenView{}, fmt.Errorf("%w: name is required", apperrors.ErrInvalidInput)
	}
	// v1 tokens are read-only; default the scope set to ["read"].
	if len(scopes) == 0 {
		scopes = []string{"read"}
	}
	plaintext, hash, prefix, err := generateAPIToken()
	if err != nil {
		return "", APITokenView{}, err
	}
	t := &models.APIToken{
		UserID: userID, Name: name, TokenHash: hash, TokenPrefix: prefix,
		Scopes: encodeScopes(scopes), ExpiresAt: expiresAt,
	}
	if err := s.repo.Create(t); err != nil {
		return "", APITokenView{}, err
	}
	return plaintext, toAPITokenView(*t), nil
}

func (s *APITokenService) Revoke(id string) error {
	return s.repo.Revoke(id)
}

// AuthenticateAPIToken validates a raw bearer token and returns the owning user
// id + scopes. Implements middleware.APITokenAuthenticator. Rejects expired or
// revoked tokens. last_used is refreshed at most once a minute.
func (s *APITokenService) AuthenticateAPIToken(raw string) (userID string, scopes []string, ok bool) {
	if !strings.HasPrefix(raw, APITokenPrefix) {
		return "", nil, false
	}
	t, err := s.repo.FindByHash(hashAPIToken(raw))
	if err != nil {
		return "", nil, false
	}
	if subtle.ConstantTimeCompare([]byte(t.TokenHash), []byte(hashAPIToken(raw))) != 1 {
		return "", nil, false
	}
	if t.ExpiresAt != nil && time.Now().After(*t.ExpiresAt) {
		return "", nil, false
	}
	s.touch(t.ID.String())
	return t.UserID.String(), parseScopes(t.Scopes), true
}

func (s *APITokenService) touch(id string) {
	now := time.Now()
	s.mu.Lock()
	last, seen := s.lastTouch[id]
	if seen && now.Sub(last) < touchThrottle {
		s.mu.Unlock()
		return
	}
	s.lastTouch[id] = now
	s.mu.Unlock()
	_ = s.repo.TouchLastUsed(id, now)
}

func generateAPIToken() (plaintext, hash, prefix string, err error) {
	b := make([]byte, 24)
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate token: %w", err)
	}
	plaintext = APITokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	return plaintext, hashAPIToken(plaintext), displayPrefix(plaintext), nil
}

func hashAPIToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}
