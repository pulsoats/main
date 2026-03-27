package auth

import (
	"time"

	"github.com/google/uuid"
)

type LoginAttempt struct {
	ID        uuid.UUID
	UserID    *uuid.UUID
	Email     string
	IPAddress *string
	UserAgent *string
	Success   bool
	Reason    *string
	CreatedAt time.Time
}

type LoginInput struct {
	Email     string
	Password  string
	IPAddress *string
	UserAgent *string
}

type LoginResponse struct {
	AccessToken  string
	RefreshToken string
}
