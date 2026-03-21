package auth

import (
	"context"
)

// Repository provides methods from database for auth purposes
type Repository interface {
	CreateInviteToken(ctx context.Context, token InviteToken) error
	InviteTokenByHash(ctx context.Context, tokenHash string) (InviteToken, error)
	MarkInviteTokenUsed(ctx context.Context, id int64) error

	CreateUser(ctx context.Context, user *User) error
	UserByID(ctx context.Context, id int64) (User, error)
	UserByEmail(ctx context.Context, email string) (User, error)

	ChangePassword(ctx context.Context, userID int64, passwordHash string) error
	SetUserRole(ctx context.Context, userID int64, role Role) error
	CreatePasswordResetToken(ctx context.Context, token *PasswordResetToken) error
	PasswordResetTokenByHash(ctx context.Context, tokenHash string) (PasswordResetToken, error)
	MarkPasswordResetTokenUsed(ctx context.Context, id int64) error

	CreateEmailVerificationToken(ctx context.Context, token *EmailVerificationToken) error
	EmailVerificationTokenByHash(ctx context.Context, tokenHash string) (EmailVerificationToken, error)
	MarkEmailVerificationTokenUsed(ctx context.Context, id int64) error
	MarkUserEmailVerified(ctx context.Context, userID int64) error

	CreateSession(ctx context.Context, session *Session) error
	SessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (Session, error)
	RevokeSession(ctx context.Context, id int64) error
	RevokeSessionsByUser(ctx context.Context, userID int64) error
	RevokeSessionsByUserExcept(ctx context.Context, userID int64, exceptedSessionID int64) error

	CreateLoginAttempt(ctx context.Context, attempt LoginAttempt) error
}

type Service interface {
	InviteToken(ctx context.Context, userID int64) (string, error)

	Register(ctx context.Context, email, password string, inviteToken string) error
	VerifyEmailByToken(ctx context.Context, emailVerificationToken string) error

	Login(ctx context.Context, input LoginInput) (resp LoginResponse, err error)
	Logout(ctx context.Context, sessionID int64) error
	LogoutAll(ctx context.Context, userID int64, exceptedSessionID int64) error

	RefreshToken(ctx context.Context, currentToken string) (LoginResponse, error)

	ChangePassword(ctx context.Context, input ChangePasswordInput) error
	RequestPasswordReset(ctx context.Context, email string) error
	ResetPassword(ctx context.Context, resetPasswordToken string, newPassword string) error
	EnsureRoot(ctx context.Context, email, password string) error
}
