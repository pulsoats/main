package auth

import (
	"time"

	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/domain/auth"
)

type registerRequest struct {
	Email       string `json:"email" binding:"required,email"`
	Password    string `json:"password" binding:"required,min=8"`
	InviteToken string `json:"inviteToken" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type loginResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type inviteTokenResponse struct {
	ID        uuid.UUID  `json:"id"`
	CreatedBy uuid.UUID  `json:"createdBy"`
	ExpiresAt string     `json:"expiresAt"`
	UsedBy    *uuid.UUID `json:"usedBy"`
	UsedAt    *string    `json:"usedAt"`
	CreatedAt string     `json:"createdAt"`
}

func mapToInviteTokenResponse(token auth.InviteToken) inviteTokenResponse {
	var usedAtStr *string
	if token.UsedAt != nil {
		s := token.UsedAt.Format(time.RFC3339)
		usedAtStr = &s
	}
	return inviteTokenResponse{
		ID:        token.ID,
		CreatedBy: token.CreatedBy,
		ExpiresAt: token.ExpiresAt.Format(time.RFC3339),
		UsedBy:    token.UsedBy,
		UsedAt:    usedAtStr,
		CreatedAt: token.CreatedAt.Format(time.RFC3339),
	}
}

type createInviteTokenResponse struct {
	Token inviteTokenResponse `json:"token"`
	Link  string              `json:"link"`
}

func mapInviteTokensToSliceResponse(tokens []auth.InviteToken) []inviteTokenResponse {
	res := make([]inviteTokenResponse, 0, len(tokens))
	for _, t := range tokens {
		res = append(res, mapToInviteTokenResponse(t))
	}

	return res
}

type refreshTokenRequest struct {
	RefreshToken string `json:"refreshToken" binding:"required"`
}

type refreshTokenResponse struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

type changePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" binding:"required"`
	NewPassword     string `json:"newPassword" binding:"required,min=8"`
}

type passwordResetEmailRequest struct {
	Email string `json:"email" binding:"required,email"`
}

type resetPasswordRequest struct {
	ResetPasswordToken string `json:"resetPasswordToken" binding:"required"`
	NewPassword        string `json:"newPassword" binding:"required"`
}

type sessionResponse struct {
	ID        uuid.UUID `json:"id"`
	UserAgent *string   `json:"userAgent"`
	IPAddress *string   `json:"ipAddress"`
	CreatedAt string    `json:"createdAt"`
}

func mapToSessionResponse(session auth.Session) sessionResponse {
	return sessionResponse{
		ID:        session.ID,
		UserAgent: session.UserAgent,
		IPAddress: session.IPAddress,
		CreatedAt: session.CreatedAt.Format(time.RFC3339),
	}
}

func mapToSessionResponseSlice(sessions []auth.Session) []sessionResponse {
	res := make([]sessionResponse, 0, len(sessions))
	for _, s := range sessions {
		res = append(res, mapToSessionResponse(s))
	}
	return res
}
