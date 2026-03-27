package auth

import "github.com/google/uuid"

type ChangePasswordInput struct {
	UserID           uuid.UUID
	CurrentSessionID uuid.UUID
	CurrentPassword  string
	NewPassword      string
}
