package auth

import (
	"context"

	"github.com/google/uuid"
	"github.com/pulsoats/main/internal/domain/auth"
)

type app interface {
	CreateInviteToken(ctx context.Context, userID uuid.UUID, role auth.UserRole) (auth.InviteToken, string, error)
	RevokeInviteToken(ctx context.Context, userID uuid.UUID, tokenID uuid.UUID, role auth.UserRole) error
	ListInviteTokens(ctx context.Context, userID uuid.UUID, role auth.UserRole) ([]auth.InviteToken, error)

	Register(ctx context.Context, email, password string, inviteToken string) error
	VerifyEmailByToken(ctx context.Context, emailVerificationToken string) error

	Login(ctx context.Context, input auth.LoginInput) (resp auth.LoginResponse, err error)
	Logout(ctx context.Context, sessionID uuid.UUID) error
	LogoutAll(ctx context.Context, userID uuid.UUID, exceptedSessionID uuid.UUID) error

	UserByID(ctx context.Context, id uuid.UUID) (auth.User, error)

	ListActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) ([]auth.Session, error)

	RefreshToken(ctx context.Context, currentToken string) (auth.LoginResponse, error)

	ChangePassword(ctx context.Context, input auth.ChangePasswordInput) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, resetPasswordToken string, newPassword string) error
	EnsureRoot(ctx context.Context, email, password string) error
}
