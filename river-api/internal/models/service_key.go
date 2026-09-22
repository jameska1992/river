package models

import "time"

// ServiceKey is a per-service API credential for the internal services
// (scan / trans / meta). It replaces the shared service-account password with
// an individually revocable, scoped key.
//
// Only the SHA-256 hash of the key is stored — the plaintext is shown once at
// mint time and never again. Keys are high-entropy random tokens (see
// services.ServiceKeyService), so a plain SHA-256 is sufficient (no salt/bcrypt
// needed — there's nothing to brute-force). KeyPrefix is the first few
// characters of the plaintext, kept for display so the admin can tell keys
// apart in the UI without exposing the secret.
//
// Scopes is a JSON-encoded []string (same convention as Genres/Paths) listing
// what the key may do; the auth middleware enforces them. Name is the service
// identity (e.g. "river-meta-movie") and is unique, which is what makes the
// env-seed path idempotent (create-if-absent by name).
type ServiceKey struct {
	Base
	Name       string     `gorm:"uniqueIndex;not null" json:"name"`
	KeyHash    string     `gorm:"uniqueIndex;not null" json:"-"`
	KeyPrefix  string     `json:"key_prefix"`
	Scopes     string     `gorm:"not null;default:'[]'" json:"scopes"`
	Revoked    bool       `gorm:"not null;default:false" json:"revoked"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}
