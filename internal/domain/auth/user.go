package auth

import "time"

type Role string

const (
	RoleUser  Role = "user"
	RoleAdmin Role = "admin"
)

type User struct {
	ID              int64
	Email           string
	PasswordHash    string
	Role            Role
	Status          string
	EmailVerifiedAt *time.Time
	LockedUntil     *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}
