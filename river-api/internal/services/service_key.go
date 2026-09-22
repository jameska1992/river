package services

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"river-api/internal/apperrors"
	"river-api/internal/models"
	"river-api/internal/repository"
)

// ServiceKeyPrefix is the fixed marker every service key starts with. The auth
// middleware uses it to tell a service key apart from a JWT bearer token, so it
// can route validation without trying (and failing) to JWT-parse a key.
const ServiceKeyPrefix = "rvk_"

// Scope constants. Each gates a specific endpoint group in the router; a key
// only carries the scopes its service actually needs, so a compromised key
// can't reach beyond them (e.g. a transcoder key can't read the TMDB secret).
const (
	ScopeMediaWrite     = "media:write"     // create/update media records
	ScopeLogsWrite      = "logs:write"      // POST /logs
	ScopeJobsWrite      = "jobs:write"      // POST /failed-jobs
	ScopeSettingsTMDB   = "settings:tmdb"   // GET /settings/tmdb (secret)
	ScopeSettingsScan   = "settings:scan"   // GET /settings/scanning
	ScopeTVShowsResolve = "tvshows:resolve" // GET /admin/tvshows/resolve
)

// AllScopes is the canonical list, used to validate admin-supplied scopes.
var AllScopes = []string{
	ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite,
	ScopeSettingsTMDB, ScopeSettingsScan, ScopeTVShowsResolve,
}

// defaultServiceScopes maps a known service name to the scopes it needs. Used
// by the env-seed path (scopes derived server-side, so the deploy only carries
// name + key) and to pre-fill the admin mint form. Every internal service can
// write media, logs, and failed-jobs; only the differentiated reads are
// restricted.
var defaultServiceScopes = map[string][]string{
	"river-scan":        {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite, ScopeSettingsScan, ScopeTVShowsResolve},
	"river-video-trans": {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite},
	"river-audio-trans": {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite},
	"river-meta-movie":  {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite, ScopeSettingsTMDB},
	"river-meta-tv":     {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite, ScopeSettingsTMDB},
	"river-meta-book":   {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite},
	"river-meta-music":  {ScopeMediaWrite, ScopeLogsWrite, ScopeJobsWrite},
}

// DefaultScopesForService returns the recommended scope set for a known service
// name, or nil if the name isn't recognised (caller must supply scopes).
func DefaultScopesForService(name string) []string {
	return defaultServiceScopes[name]
}

// touchThrottle is how stale a key's last_used_at must be before we write a
// fresh timestamp — avoids a DB write on every single request.
const touchThrottle = time.Minute

type ServiceKeyService struct {
	repo repository.ServiceKeyRepository

	mu        sync.Mutex
	lastTouch map[string]time.Time // keyID → last last_used write
}

func NewServiceKeyService(repo repository.ServiceKeyRepository) *ServiceKeyService {
	return &ServiceKeyService{repo: repo, lastTouch: make(map[string]time.Time)}
}

// ServiceKeyView is the admin-facing, secret-free representation.
type ServiceKeyView struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     []string   `json:"scopes"`
	Revoked    bool       `json:"revoked"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func toView(k models.ServiceKey) ServiceKeyView {
	return ServiceKeyView{
		ID: k.ID.String(), Name: k.Name, KeyPrefix: k.KeyPrefix,
		Scopes: parseScopes(k.Scopes), Revoked: k.Revoked,
		LastUsedAt: k.LastUsedAt, CreatedAt: k.CreatedAt,
	}
}

// List returns all keys (secret-free) for the admin UI.
func (s *ServiceKeyService) List() ([]ServiceKeyView, error) {
	keys, err := s.repo.List()
	if err != nil {
		return nil, err
	}
	out := make([]ServiceKeyView, len(keys))
	for i, k := range keys {
		out[i] = toView(k)
	}
	return out, nil
}

// Mint creates a brand-new key with a random secret, returning the plaintext
// exactly once (it's only stored hashed). A name collision is a conflict — the
// admin should revoke or reuse the existing one.
func (s *ServiceKeyService) Mint(name string, scopes []string) (plaintext string, view ServiceKeyView, err error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ServiceKeyView{}, fmt.Errorf("%w: name is required", apperrors.ErrInvalidInput)
	}
	clean, err := validateScopes(scopes)
	if err != nil {
		return "", ServiceKeyView{}, err
	}
	if _, err := s.repo.FindByName(name); err == nil {
		return "", ServiceKeyView{}, fmt.Errorf("%w: a key named %q already exists", apperrors.ErrConflict, name)
	}

	plaintext, hash, prefix, err := generateKey()
	if err != nil {
		return "", ServiceKeyView{}, err
	}
	k := &models.ServiceKey{Name: name, KeyHash: hash, KeyPrefix: prefix, Scopes: encodeScopes(clean)}
	if err := s.repo.Create(k); err != nil {
		return "", ServiceKeyView{}, err
	}
	return plaintext, toView(*k), nil
}

// Seed provisions a key from an admin-supplied plaintext (the env-seed path),
// create-if-absent by name so it never clobbers a key already present (e.g. one
// an admin rotated via the UI). Scopes are derived from the service name when
// not supplied. A malformed key (missing the rvk_ prefix) is rejected so a
// misconfigured env is visible rather than silently unusable.
func (s *ServiceKeyService) Seed(name, plaintext string, scopes []string) (created bool, err error) {
	name = strings.TrimSpace(name)
	plaintext = strings.TrimSpace(plaintext)
	if name == "" || plaintext == "" {
		return false, nil // nothing to seed — skip quietly
	}
	if !strings.HasPrefix(plaintext, ServiceKeyPrefix) {
		return false, fmt.Errorf("%w: key for %q must start with %q", apperrors.ErrInvalidInput, name, ServiceKeyPrefix)
	}
	if len(scopes) == 0 {
		scopes = DefaultScopesForService(name)
	}
	clean, err := validateScopes(scopes)
	if err != nil {
		return false, err
	}
	k := &models.ServiceKey{
		Name:      name,
		KeyHash:   hashKey(plaintext),
		KeyPrefix: displayPrefix(plaintext),
		Scopes:    encodeScopes(clean),
	}
	return s.repo.CreateIfAbsent(k)
}

func (s *ServiceKeyService) Revoke(id string) error {
	return s.repo.Revoke(id)
}

// AuthenticateServiceKey validates a raw bearer token as a service key and
// returns the service name + scopes. Implements middleware.ServiceKeyAuthenticator.
// Runs on every key-authed request, so it's a single indexed hash lookup;
// last_used is refreshed at most once a minute per key.
func (s *ServiceKeyService) AuthenticateServiceKey(rawKey string) (name string, scopes []string, ok bool) {
	if !strings.HasPrefix(rawKey, ServiceKeyPrefix) {
		return "", nil, false
	}
	k, err := s.repo.FindByHash(hashKey(rawKey))
	if err != nil {
		return "", nil, false
	}
	// Constant-time confirm the stored hash matches (defence-in-depth; the DB
	// lookup already matched, but this avoids relying solely on SQL equality).
	if subtle.ConstantTimeCompare([]byte(k.KeyHash), []byte(hashKey(rawKey))) != 1 {
		return "", nil, false
	}
	s.touch(k.ID.String())
	return k.Name, parseScopes(k.Scopes), true
}

// touch refreshes last_used_at, throttled so we don't write on every request.
func (s *ServiceKeyService) touch(id string) {
	now := time.Now()
	s.mu.Lock()
	last, seen := s.lastTouch[id]
	if seen && now.Sub(last) < touchThrottle {
		s.mu.Unlock()
		return
	}
	s.lastTouch[id] = now
	s.mu.Unlock()
	// Best-effort; a failed timestamp write must never fail the request.
	_ = s.repo.TouchLastUsed(id, now)
}

// --- key + scope helpers ---

func generateKey() (plaintext, hash, prefix string, err error) {
	b := make([]byte, 24) // 192 bits of entropy
	if _, err = rand.Read(b); err != nil {
		return "", "", "", fmt.Errorf("generate key: %w", err)
	}
	plaintext = ServiceKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
	return plaintext, hashKey(plaintext), displayPrefix(plaintext), nil
}

func hashKey(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// displayPrefix returns a short, non-secret prefix for the UI listing.
func displayPrefix(plaintext string) string {
	const n = 12
	if len(plaintext) <= n {
		return plaintext
	}
	return plaintext[:n]
}

func validateScopes(scopes []string) ([]string, error) {
	seen := map[string]bool{}
	out := make([]string, 0, len(scopes))
	for _, sc := range scopes {
		sc = strings.TrimSpace(sc)
		if sc == "" {
			continue
		}
		if !isValidScope(sc) {
			return nil, fmt.Errorf("%w: unknown scope %q", apperrors.ErrInvalidInput, sc)
		}
		if !seen[sc] {
			seen[sc] = true
			out = append(out, sc)
		}
	}
	sort.Strings(out)
	return out, nil
}

func isValidScope(sc string) bool {
	for _, v := range AllScopes {
		if v == sc {
			return true
		}
	}
	return false
}

func encodeScopes(scopes []string) string {
	b, _ := json.Marshal(scopes)
	return string(b)
}

func parseScopes(s string) []string {
	var out []string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return []string{}
	}
	return out
}
