package auth

import (
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
)

type UserRole string

const (
	RoleUser  UserRole = "user"
	RoleAdmin UserRole = "admin"
)

func ParseUserRole(s string) (UserRole, error) {
	switch UserRole(s) {
	case RoleAdmin, RoleUser:
		return UserRole(s), nil
	default:
		return "", fmt.Errorf("parse role: %w", errorsx.ErrInvalidArgument)
	}
}

type UserStatus string

const (
	StatusPending  = "pending"
	StatusActive   = "active"
	StatusDisabled = "disabled"
)

func ParseStatus(s string) (UserStatus, error) {
	switch UserStatus(s) {
	case StatusPending, StatusActive, StatusDisabled:
		return UserStatus(s), nil
	default:
		return "", fmt.Errorf("parse status: %w", errorsx.ErrInvalidArgument)
	}
}

type User struct {
	ID              uuid.UUID
	Email           string
	PasswordHash    string
	Role            UserRole
	Status          UserStatus
	EmailVerifiedAt *time.Time
	LockedUntil     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
