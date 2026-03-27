package auth

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	ID               uuid.UUID
	UserID           uuid.UUID
	RefreshTokenHash string
	UserAgent        *string
	IPAddress        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	ReplacedByID     *uuid.UUID
	CreatedAt        time.Time
}
