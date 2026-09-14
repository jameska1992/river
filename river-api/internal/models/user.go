package models

import (
	"time"

	"github.com/google/uuid"
)

type Role string

const (
	RoleAdmin Role = "admin"
	RoleUser  Role = "user"
	// RoleService is a non-human, least-privilege role for the internal
	// services (scan / trans / meta). It can create and update media records
	// but not touch users, settings writes, deletes, or other admin surface.
	// Provisioned only by boot-time seeding, never assignable via the API.
	RoleService Role = "service"
)

type User struct {
	Base
	Username     string `gorm:"uniqueIndex;not null" json:"username"`
	Email        string `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash string `gorm:"not null" json:"-"`
	Role         Role   `gorm:"default:user" json:"role"`
}

type RefreshToken struct {
	Base
	UserID    uuid.UUID `gorm:"type:varchar(36);not null;index" json:"user_id"`
	Token     string    `gorm:"uniqueIndex;not null" json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	Revoked   bool      `gorm:"default:false" json:"revoked"`
}
