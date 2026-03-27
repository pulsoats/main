package auth

import (
	"context"

	"github.com/google/uuid"
)

// Repository provides methods from database for auth purposes
type Repository interface {
	CreateInviteToken(ctx context.Context, token InviteToken) error
	InviteTokenByHash(ctx context.Context, tokenHash string) (InviteToken, error)
	ListInviteTokens(ctx context.Context) ([]InviteToken, error)
	MarkInviteTokenUsed(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	RevokeInviteToken(ctx context.Context, id uuid.UUID) error

	CreateUser(ctx context.Context, user *User) error
	UserByID(ctx context.Context, id uuid.UUID) (User, error)
	UserByEmail(ctx context.Context, email string) (User, error)

	ChangePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error
	SetUserRole(ctx context.Context, userID uuid.UUID, role UserRole) error
	CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error
	PasswordResetTokenByHash(ctx context.Context, tokenHash string) (PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error

	CreateEmailVerificationToken(ctx context.Context, token *EmailVerificationToken) error
	EmailVerificationTokenByHash(ctx context.Context, tokenHash string) (EmailVerificationToken, error)
	MarkEmailVerificationTokenUsed(ctx context.Context, id uuid.UUID) error
	MarkUserEmailVerified(ctx context.Context, userID uuid.UUID) error

	CreateSession(ctx context.Context, session *Session) error
	SessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (Session, error)
	ListActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) ([]Session, error)
	RevokeSession(ctx context.Context, id uuid.UUID) error
	RevokeSessionsByUser(ctx context.Context, userID uuid.UUID) error
	RevokeSessionsByUserExcept(ctx context.Context, userID uuid.UUID, exceptedSessionID uuid.UUID) error

	CreateLoginAttempt(ctx context.Context, attempt LoginAttempt) error
}
