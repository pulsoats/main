package auth

import (
	"time"

	"github.com/google/uuid"
)

type EmailVerificationToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type PasswordResetToken struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	TokenHash string
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

type AccessTokenClaims struct {
	UserID    uuid.UUID
	SessionID uuid.UUID
	Role      UserRole
}
