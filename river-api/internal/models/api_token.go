package models

import (
	"time"

	"github.com/google/uuid"
)

// APIToken is a long-lived, scoped, revocable credential a human admin mints for
// an external tool to call River's API. It parallels ServiceKey (internal
// services) but is user-owned and may carry an optional expiry.
//
// Only the SHA-256 hash of the token is stored — the plaintext is shown once at
// mint time. TokenPrefix is a short display prefix so tokens can be told apart
// in the UI. Scopes is a JSON-encoded []string.
type APIToken struct {
	Base
	UserID      uuid.UUID  `gorm:"type:varchar(36);not null;index" json:"user_id"`
	Name        string     `gorm:"not null" json:"name"`
	TokenHash   string     `gorm:"uniqueIndex;not null" json:"-"`
	TokenPrefix string     `json:"token_prefix"`
	Scopes      string     `gorm:"not null;default:'[]'" json:"scopes"`
	Revoked     bool       `gorm:"not null;default:false" json:"revoked"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}
