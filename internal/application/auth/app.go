package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/core/lib/logx"
	"github.com/pulsoats/main/internal/domain"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/domain/mailer"
	"github.com/pulsoats/main/internal/infrastructure/email/templates"
)

const defaultAppName = "TradeBot"

type Application struct {
	repo           auth.Repository
	tx             domain.TxManager
	emailSender    mailer.Sender
	tokenSvc       tokenService
	appFrontendURL string
	appName        string
	logger         logx.Logger
}

type ApplicationConfig struct {
	Repository     auth.Repository
	EmailSender    mailer.Sender
	TokenService   tokenService
	appFrontendURL string
	AppName        string
	Logger         logx.Logger
	TxManager      domain.TxManager
}

func NewApplication(cfg ApplicationConfig) (*Application, error) {
	if cfg.Repository == nil {
		return nil, fmt.Errorf("auth app:  auth repository: %w", errorsx.ErrInvalidArgument)
	}
	if cfg.TxManager == nil {
		return nil, fmt.Errorf("auth app: tx (transaction) manager: %w", errorsx.ErrInvalidArgument)
	}
	if cfg.EmailSender == nil {
		return nil, fmt.Errorf("auth app: email sender: %w", errorsx.ErrInvalidArgument)
	}
	baseURL := strings.TrimSpace(cfg.appFrontendURL)
	if baseURL == "" {
		return nil, fmt.Errorf("auth app: app base url: %w", errorsx.ErrInvalidArgument)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	appName := strings.TrimSpace(cfg.AppName)
	if appName == "" {
		return nil, fmt.Errorf("auth app: app name: %w", errorsx.ErrInvalidArgument)
	}

	return &Application{
		repo:           cfg.Repository,
		tx:             cfg.TxManager,
		emailSender:    cfg.EmailSender,
		appFrontendURL: baseURL,
		appName:        appName,
		logger:         cfg.Logger,
	}, nil
}

func (a *Application) CreateInviteToken(ctx context.Context, userID uuid.UUID) (auth.InviteToken, error) {
	rawToken, err := a.tokenSvc.GenerateToken()
	if err != nil {
		return auth.InviteToken{}, fmt.Errorf("create invite token: generate token: %w", errors.Join(errorsx.ErrInternal, err))
	}

	token := auth.InviteToken{
		TokenHash: a.tokenSvc.HashToken(rawToken),
		CreatedBy: userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := a.repo.CreateInviteToken(ctx, token); err != nil {
		return auth.InviteToken{}, err
	}

	link := a.buildTokenLink("register", rawToken)
	token.Link = link
	
	return token, nil
}

func (a *Application) Register(ctx context.Context, email, password, inviteToken string) error {
	if inviteToken == "" {
		return fmt.Errorf("register: %w", errorsx.ErrInvalidArgument)
	}

	email = strings.ToLower(strings.TrimSpace(email))
	inviteTokenHash := a.tokenSvc.HashToken(inviteToken)

	passwordHash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("register: create password hash: %w", errors.Join(errorsx.ErrInternal, err))
	}

	rawVerificationToken, err := a.tokenSvc.GenerateToken()
	if err != nil {
		return fmt.Errorf("register: generate verification token: %w", errors.Join(errorsx.ErrInternal, err))
	}
	verificationTokenHash := a.tokenSvc.HashToken(rawVerificationToken)

	user := auth.User{
		Email:        email,
		PasswordHash: passwordHash,
	}

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		invite, err := a.repo.InviteTokenByHash(txCtx, inviteTokenHash)
		if err != nil {
			return fmt.Errorf("register: %w", err)
		}

		if invite.UsedAt != nil {
			return fmt.Errorf("register: %w", errorsx.ErrInvalidArgument)
		}
		if invite.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("register: %w", errorsx.ErrInvalidArgument)
		}

		if err := a.repo.CreateUser(txCtx, &user); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		verificationToken := auth.EmailVerificationToken{
			UserID:    user.ID,
			TokenHash: verificationTokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		if err := a.repo.CreateEmailVerificationToken(txCtx, &verificationToken); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		if err := a.repo.MarkInviteTokenUsed(txCtx, invite.ID, user.ID); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if err = a.sendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	return nil
}

func (a *Application) VerifyEmailByToken(ctx context.Context, emailVerificationToken string) error {
	tokenHash := a.tokenSvc.HashToken(emailVerificationToken)

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		token, err := a.repo.EmailVerificationTokenByHash(txCtx, tokenHash)
		if err != nil {
			return fmt.Errorf("verify email by token: %w", err)
		}

		if token.UsedAt != nil {
			return fmt.Errorf("verify email by token: %w", errorsx.ErrInvalidArgument)
		}
		if token.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("verify email by token: %w", errorsx.ErrInvalidArgument)
		}

		if err := a.repo.MarkEmailVerificationTokenUsed(txCtx, token.ID); err != nil {
			return fmt.Errorf("verify email by token: %w", err)
		}

		if err := a.repo.MarkUserEmailVerified(txCtx, token.UserID); err != nil {
			return fmt.Errorf("verify email by token: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) EnsureRoot(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return fmt.Errorf("ensure root: credentials: %w", errorsx.ErrInvalidArgument)
	}

	existing, err := a.repo.UserByEmail(ctx, email)
	if err == nil {
		if existing.Role != auth.RoleAdmin {
			if err := a.repo.SetUserRole(ctx, existing.ID, auth.RoleAdmin); err != nil {
				return fmt.Errorf("ensure root: set role: %w", err)
			}
		}
		return nil
	}
	if !errors.Is(err, errorsx.ErrNotFound) {
		return fmt.Errorf("ensure root: %w", err)
	}

	passwordHash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("ensure root: create password hash: %w", errors.Join(errorsx.ErrInternal, err))
	}

	rawVerificationToken, err := a.tokenSvc.GenerateToken()
	if err != nil {
		return fmt.Errorf("ensure root: generate verification token: %w", errors.Join(errorsx.ErrInternal, err))
	}
	verificationTokenHash := a.tokenSvc.HashToken(rawVerificationToken)

	user := auth.User{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         auth.RoleAdmin,
	}

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := a.repo.CreateUser(txCtx, &user); err != nil {
			return fmt.Errorf("ensure root: create user: %w", err)
		}

		token := auth.EmailVerificationToken{
			UserID:    user.ID,
			TokenHash: verificationTokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := a.repo.CreateEmailVerificationToken(txCtx, &token); err != nil {
			return fmt.Errorf("ensure root: create verification token: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err = a.sendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		return fmt.Errorf("ensure root: %w", err)
	}
	return nil
}

func (a *Application) Login(ctx context.Context, input auth.LoginInput) (resp auth.LoginResponse, err error) {
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	attempt := auth.LoginAttempt{
		Email:     input.Email,
		IPAddress: input.IPAddress,
		UserAgent: input.UserAgent,
		Success:   false,
	}

	defer func() {
		if err != nil && attempt.Reason == nil {
			reason := "login_failed"
			attempt.Reason = &reason
		}
		_ = a.repo.CreateLoginAttempt(ctx, attempt)
	}()

	var (
		userID          uuid.UUID
		userRole        auth.UserRole
		rawRefreshToken string
	)

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := a.repo.UserByEmail(txCtx, input.Email)
		if err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				reason := "invalid_credentials"
				attempt.Reason = &reason
				return errorsx.ErrUnauthorized
			}
			return fmt.Errorf("login: %w", err)
		}

		userID = user.ID
		userRole = user.Role
		attempt.UserID = &user.ID

		if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
			reason := "account_locked"
			attempt.Reason = &reason
			return errorsx.ErrUnauthorized
		}
		if user.Status == "disabled" || user.Status == "locked" {
			reason := "account_disabled"
			attempt.Reason = &reason
			return errorsx.ErrUnauthorized
		}
		if user.Status == "pending" {
			reason := "email_not_verified"
			attempt.Reason = &reason
			return errorsx.ErrUnauthorized
		}

		matched, err := argon2id.ComparePasswordAndHash(input.Password, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("login: compare password: %w", errors.Join(errorsx.ErrInternal, err))
		}
		if !matched {
			reason := "invalid_credentials"
			attempt.Reason = &reason
			return errorsx.ErrUnauthorized
		}

		rawRefreshToken, err = a.tokenSvc.GenerateToken()
		if err != nil {
			return fmt.Errorf("login: generate refresh token: %w", errors.Join(errorsx.ErrInternal, err))
		}

		session := auth.Session{
			UserID:           user.ID,
			RefreshTokenHash: a.tokenSvc.HashToken(rawRefreshToken),
			UserAgent:        input.UserAgent,
			IPAddress:        input.IPAddress,
			ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		}

		if err := a.repo.CreateSession(txCtx, &session); err != nil {
			return fmt.Errorf("login: %w", err)
		}

		return nil
	})
	if err != nil {
		return auth.LoginResponse{}, err
	}

	accessToken, err := a.tokenSvc.GenerateAccessToken(userID, userRole)
	if err != nil {
		reason := "access_token_failed"
		attempt.Reason = &reason
		return auth.LoginResponse{}, fmt.Errorf("login: generate access token: %w", errors.Join(errorsx.ErrInternal, err))
	}

	attempt.Success = true

	return auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, nil
}

func (a *Application) Logout(ctx context.Context, sessionID uuid.UUID) error {
	if err := a.repo.RevokeSession(ctx, sessionID); err != nil {
		return fmt.Errorf("logout: %w", err)
	}

	return nil
}

func (a *Application) ListActiveSessions(ctx context.Context, userID uuid.UUID) ([]auth.Session, error) {
	sessions, err := a.repo.ListActiveSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions: %w", err)
	}
	return sessions, nil
}

func (a *Application) LogoutAll(ctx context.Context, userID uuid.UUID, exceptedSessionID uuid.UUID) error {
	if err := a.repo.RevokeSessionsByUserExcept(ctx, userID, exceptedSessionID); err != nil {
		return fmt.Errorf("logout all: %w", err)
	}
	return nil
}

func (a *Application) RefreshToken(ctx context.Context, currentToken string) (auth.LoginResponse, error) {
	tokenHash := a.tokenSvc.HashToken(currentToken)

	var (
		newRefreshToken string
		userID          uuid.UUID
		userRole        auth.UserRole
	)

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		session, err := a.repo.SessionByRefreshTokenHash(txCtx, tokenHash)
		if err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				return errorsx.ErrUnauthorized
			}
			return fmt.Errorf("refresh token: %w", err)
		}

		if session.RevokedAt != nil {
			return errorsx.ErrUnauthorized
		}
		if session.ExpiresAt.Before(time.Now()) {
			return errorsx.ErrUnauthorized
		}

		user, err := a.repo.UserByID(txCtx, session.UserID)
		if err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}

		userID = user.ID
		userRole = user.Role

		if err := a.repo.RevokeSession(txCtx, session.ID); err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}

		newRefreshToken, err = a.tokenSvc.GenerateToken()
		if err != nil {
			return fmt.Errorf("refresh token: generate token: %w", errors.Join(errorsx.ErrInternal, err))
		}

		newSession := auth.Session{
			UserID:           session.UserID,
			RefreshTokenHash: a.tokenSvc.HashToken(newRefreshToken),
			UserAgent:        session.UserAgent,
			IPAddress:        session.IPAddress,
			ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		}

		if err := a.repo.CreateSession(txCtx, &newSession); err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}

		return nil
	})
	if err != nil {
		return auth.LoginResponse{}, err
	}

	accessToken, err := a.tokenSvc.GenerateAccessToken(userID, userRole)
	if err != nil {
		return auth.LoginResponse{}, fmt.Errorf("refresh token: generate access token: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (a *Application) ChangePassword(ctx context.Context, input auth.ChangePasswordInput) error {
	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := a.repo.UserByID(txCtx, input.UserID)
		if err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		matched, err := argon2id.ComparePasswordAndHash(input.CurrentPassword, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("change password: compare password: %w", errors.Join(errorsx.ErrInternal, err))
		}
		if !matched {
			return fmt.Errorf("change password: %w", errorsx.ErrUnauthorized)
		}

		newPasswordHash, err := argon2id.CreateHash(input.NewPassword, argon2id.DefaultParams)
		if err != nil {
			return fmt.Errorf("change password: create password hash: %w", errors.Join(errorsx.ErrInternal, err))
		}

		if err := a.repo.ChangePassword(txCtx, input.UserID, newPasswordHash); err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		if err := a.repo.RevokeSessionsByUserExcept(txCtx, input.UserID, input.CurrentSessionID); err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	var rawToken string

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := a.repo.UserByEmail(txCtx, email)
		if err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("request password reset: %w", err)
		}

		rawToken, err = a.tokenSvc.GenerateToken()
		if err != nil {
			return fmt.Errorf("request password reset: generate token: %w", errors.Join(errorsx.ErrInternal, err))
		}

		token := &auth.PasswordResetToken{
			UserID:    user.ID,
			TokenHash: a.tokenSvc.HashToken(rawToken),
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}

		if err := a.repo.CreatePasswordResetToken(txCtx, token); err != nil {
			return fmt.Errorf("request password reset: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if rawToken == "" {
		return nil
	}

	if err = a.sendPasswordResetEmail(ctx, email, rawToken); err != nil {
		return fmt.Errorf("request password reset: %w", err)
	}

	return nil
}

func (a *Application) ResetPassword(ctx context.Context, resetPasswordToken string, newPassword string) error {
	tokenHash := a.tokenSvc.HashToken(resetPasswordToken)

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		token, err := a.repo.PasswordResetTokenByHash(txCtx, tokenHash)
		if err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		if token.UsedAt != nil {
			return fmt.Errorf("reset password: %w", errorsx.ErrInvalidArgument)
		}
		if token.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("reset password: %w", errorsx.ErrInvalidArgument)
		}

		newPasswordHash, err := argon2id.CreateHash(newPassword, argon2id.DefaultParams)
		if err != nil {
			return fmt.Errorf("reset password: create password hash: %w", errors.Join(errorsx.ErrInternal, err))
		}

		if err := a.repo.ChangePassword(txCtx, token.UserID, newPasswordHash); err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		if err := a.repo.MarkPasswordResetTokenUsed(txCtx, token.ID); err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		if err := a.repo.RevokeSessionsByUser(txCtx, token.UserID); err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) sendVerificationEmail(ctx context.Context, to, token string) error {
	link := a.buildTokenLink("verify-email", token)
	msg := templates.Verification(to, a.appName, link)
	if err := a.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	if a.logger != nil {
		a.logger.Info("verification email sent", "to", to)
	}
	return nil
}

func (a *Application) sendPasswordResetEmail(ctx context.Context, to, token string) error {
	link := a.buildTokenLink("reset-password", token)
	msg := templates.PasswordReset(to, a.appName, link)
	if err := a.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	if a.logger != nil {
		a.logger.Info("password reset email sent", "to", to)
	}
	return nil
}

func (a *Application) buildTokenLink(path, token string) string {
	escaped := url.QueryEscape(token)
	return fmt.Sprintf("%s/%s?token=%s", a.appFrontendURL, path, escaped)
}
