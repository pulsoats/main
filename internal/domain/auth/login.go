package auth

import "time"

type LoginAttempt struct {
	ID        int64
	UserID    *int64
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
