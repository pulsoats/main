package auth

import "time"

type InviteToken struct {
	ID        int64
	TokenHash string
	CreatedBy int64
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}
