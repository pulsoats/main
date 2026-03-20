package auth

import "time"

type Session struct {
	ID               int64
	UserID           int64
	RefreshTokenHash string
	UserAgent        *string
	IPAddress        *string
	ExpiresAt        time.Time
	RevokedAt        *time.Time
	ReplacedByID     *int64
	CreatedAt        time.Time
}
