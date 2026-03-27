package auth

import (
	"time"

	"github.com/google/uuid"
)

type InviteToken struct {
	ID        uuid.UUID
	TokenHash string
	CreatedBy uuid.UUID
	ExpiresAt time.Time
	UsedBy    *uuid.UUID
	UsedAt    *time.Time
	CreatedAt time.Time
}
