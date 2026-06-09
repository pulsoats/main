package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"log/slog"

	"github.com/alexedwards/argon2id"
	"github.com/google/uuid"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/domain/mailer"
	"github.com/pulsoats/main/internal/infrastructure/email/templates"
	"github.com/pulsoats/main/internal/ports"
)

const defaultAppName = "TradeBot"

type Application struct {
	repo           auth.Repository
	tx             domain.TxManager
	emailSender    mailer.Sender
	tokenSvc       ports.TokenService
	appFrontendURL string
	appName        string
	logger         *slog.Logger
}

type ApplicationConfig struct {
	Repository     auth.Repository
	EmailSender    mailer.Sender
	TokenService   ports.TokenService
	AppFrontendURL string
	AppName        string
	Logger         *slog.Logger
	TxManager      domain.TxManager
}

type ServiceConfig = ApplicationConfig

func NewApplication(cfg ApplicationConfig) (*Application, error) {
	const op = "auth app"
	if cfg.Repository == nil {
		return nil, fmt.Errorf("%s:  auth repository: %w", op, errorsx.ErrInvalidArgument)
	}
	if cfg.TxManager == nil {
		return nil, fmt.Errorf("%s: tx (transaction) manager: %w", op, errorsx.ErrInvalidArgument)
	}
	if cfg.EmailSender == nil {
		return nil, fmt.Errorf("%s: email sender: %w", op, errorsx.ErrInvalidArgument)
	}
	if cfg.TokenService == nil {
		return nil, fmt.Errorf("%s: token service: %w", op, errorsx.ErrInvalidArgument)
	}
	baseURL := strings.TrimSpace(cfg.AppFrontendURL)
	if baseURL == "" {
		return nil, fmt.Errorf("%s: app base url: %w", op, errorsx.ErrInvalidArgument)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	appName := strings.TrimSpace(cfg.AppName)
	if appName == "" {
		return nil, fmt.Errorf("%s: app name: %w", op, errorsx.ErrInvalidArgument)
	}

	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}

	return &Application{
		repo:           cfg.Repository,
		tx:             cfg.TxManager,
		emailSender:    cfg.EmailSender,
		tokenSvc:       cfg.TokenService,
		appFrontendURL: baseURL,
		appName:        appName,
		logger:         log,
	}, nil
}

func NewService(cfg ServiceConfig) (*Application, error) {
	return NewApplication(cfg)
}

func (a *Application) CreateInviteToken(ctx context.Context, userID uuid.UUID, role auth.UserRole) (auth.InviteToken, string, error) {
	const op = "create invite token"
	if role != auth.RoleAdmin {
		return auth.InviteToken{}, "", fmt.Errorf("%s: %w", op, errorsx.ErrForbidden)
	}

	tokenID, err := newUUID()
	if err != nil {
		return auth.InviteToken{}, "", fmt.Errorf("%s: %w", op, err)
	}

	rawToken, err := a.tokenSvc.GenerateToken()
	if err != nil {
		return auth.InviteToken{}, "", fmt.Errorf("%s: generate token: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	token := auth.InviteToken{
		ID:        tokenID,
		TokenHash: a.tokenSvc.HashToken(rawToken),
		CreatedBy: userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := a.repo.CreateInviteToken(ctx, token); err != nil {
		return auth.InviteToken{}, "", err
	}

	link := a.buildTokenLink("register", rawToken)

	return token, link, nil
}

func (a *Application) RevokeInviteToken(ctx context.Context, userID, tokenID uuid.UUID, role auth.UserRole) error {
	const op = "revoke invite token"
	if role != auth.RoleAdmin {
		return fmt.Errorf("%s: %w", op, errorsx.ErrForbidden)
	}

	if tokenID == uuid.Nil {
		return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
	}

	if err := a.repo.RevokeInviteToken(ctx, tokenID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *Application) InviteTokens(ctx context.Context, userID uuid.UUID, role auth.UserRole) ([]auth.InviteToken, error) {
	const op = "invite tokens"
	if role != auth.RoleAdmin {
		return nil, fmt.Errorf("%s: %w", op, errorsx.ErrForbidden)
	}

	tokens, err := a.repo.InviteTokens(ctx)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return tokens, nil
}

func (a *Application) Register(ctx context.Context, email, password, inviteToken string) error {
	const op = "register"
	if inviteToken == "" {
		return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
	}

	userID, idErr := newUUID()
	if idErr != nil {
		return fmt.Errorf("%s: %w", op, idErr)
	}

	email = strings.ToLower(strings.TrimSpace(email))
	inviteTokenHash := a.tokenSvc.HashToken(inviteToken)

	passwordHash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("%s: create password hash: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	rawVerificationToken, err := a.tokenSvc.GenerateToken()
	if err != nil {
		return fmt.Errorf("%s: generate verification token: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	verificationTokenHash := a.tokenSvc.HashToken(rawVerificationToken)

	user := auth.User{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
	}

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		invite, err := a.repo.InviteTokenByHash(txCtx, inviteTokenHash)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if invite.UsedAt != nil {
			return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
		}
		if invite.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
		}

		if err := a.repo.CreateUser(txCtx, &user); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		tokenID, err := newUUID()
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		verificationToken := auth.EmailVerificationToken{
			ID:        tokenID,
			UserID:    user.ID,
			TokenHash: verificationTokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		if err := a.repo.CreateEmailVerificationToken(txCtx, &verificationToken); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if err := a.repo.MarkInviteTokenUsed(txCtx, invite.ID, user.ID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if err = a.sendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *Application) VerifyEmailByToken(ctx context.Context, emailVerificationToken string) error {
	const op = "verify email by token"
	tokenHash := a.tokenSvc.HashToken(emailVerificationToken)

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		token, err := a.repo.EmailVerificationTokenByHash(txCtx, tokenHash)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if token.UsedAt != nil {
			return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
		}
		if token.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
		}

		if err := a.repo.MarkEmailVerificationTokenUsed(txCtx, token.ID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if err := a.repo.MarkUserEmailVerified(txCtx, token.UserID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) EnsureRoot(ctx context.Context, email, password string) error {
	const op = "ensure root"
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return fmt.Errorf("%s: credentials: %w", op, errorsx.ErrInvalidArgument)
	}

	userID, idErr := newUUID()
	if idErr != nil {
		return fmt.Errorf("%s: %w", op, idErr)
	}

	existing, err := a.repo.UserByEmail(ctx, email)
	if err == nil {
		if existing.Role != auth.RoleAdmin {
			if err := a.repo.SetUserRole(ctx, existing.ID, auth.RoleAdmin); err != nil {
				return fmt.Errorf("%s: set role: %w", op, err)
			}
		}
		return nil
	}
	if !errors.Is(err, errorsx.ErrNotFound) {
		return fmt.Errorf("%s: %w", op, err)
	}

	passwordHash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("%s: create password hash: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	rawVerificationToken, err := a.tokenSvc.GenerateToken()
	if err != nil {
		return fmt.Errorf("%s: generate verification token: %w", op, errors.Join(errorsx.ErrInternal, err))
	}
	verificationTokenHash := a.tokenSvc.HashToken(rawVerificationToken)

	user := auth.User{
		ID:           userID,
		Email:        email,
		PasswordHash: passwordHash,
		Role:         auth.RoleAdmin,
	}

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		if err := a.repo.CreateUser(txCtx, &user); err != nil {
			return fmt.Errorf("%s: create user: %w", op, err)
		}

		tokenID, err := newUUID()
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		token := auth.EmailVerificationToken{
			ID:        tokenID,
			UserID:    user.ID,
			TokenHash: verificationTokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := a.repo.CreateEmailVerificationToken(txCtx, &token); err != nil {
			return fmt.Errorf("%s: create verification token: %w", op, err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err = a.sendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *Application) Login(ctx context.Context, input auth.LoginInput) (resp auth.LoginResponse, err error) {
	const op = "login"
	input.Email = strings.ToLower(strings.TrimSpace(input.Email))

	attemptID, genErr := newUUID()
	if genErr != nil {
		return auth.LoginResponse{}, fmt.Errorf("%s: %w", op, genErr)
	}

	attempt := auth.LoginAttempt{
		ID:        attemptID,
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
		sessionID       uuid.UUID
	)

	sessionID, genErr = newUUID()
	if genErr != nil {
		return auth.LoginResponse{}, fmt.Errorf("%s: %w", op, genErr)
	}

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := a.repo.UserByEmail(txCtx, input.Email)
		if err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				reason := "invalid_credentials"
				attempt.Reason = &reason
				return errorsx.ErrUnauthorized
			}
			return fmt.Errorf("%s: %w", op, err)
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
			return fmt.Errorf("%s: compare password: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		if !matched {
			reason := "invalid_credentials"
			attempt.Reason = &reason
			return errorsx.ErrUnauthorized
		}

		rawRefreshToken, err = a.tokenSvc.GenerateToken()
		if err != nil {
			return fmt.Errorf("%s: generate refresh token: %w", op, errors.Join(errorsx.ErrInternal, err))
		}

		session := auth.Session{
			ID:               sessionID,
			UserID:           user.ID,
			RefreshTokenHash: a.tokenSvc.HashToken(rawRefreshToken),
			UserAgent:        input.UserAgent,
			IPAddress:        input.IPAddress,
			ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		}

		if err := a.repo.CreateSession(txCtx, &session); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		sessionID = session.ID

		return nil
	})
	if err != nil {
		return auth.LoginResponse{}, err
	}

	accessToken, err := a.tokenSvc.GenerateAccessToken(auth.AccessTokenClaims{
		UserID:    userID,
		SessionID: sessionID,
		Role:      userRole,
	})
	if err != nil {
		reason := "access_token_failed"
		attempt.Reason = &reason
		return auth.LoginResponse{}, fmt.Errorf("%s: generate access token: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	attempt.Success = true

	return auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, nil
}

func (a *Application) Logout(ctx context.Context, sessionID uuid.UUID) error {
	const op = "logout"
	if err := a.repo.RevokeSession(ctx, sessionID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *Application) ActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) ([]auth.Session, error) {
	const op = "active sessions"
	sessions, err := a.repo.ActiveSessionsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return sessions, nil
}

func (a *Application) LogoutAll(ctx context.Context, userID uuid.UUID, exceptedSessionID uuid.UUID) error {
	const op = "logout all"
	if err := a.repo.RevokeSessionsByUserExcept(ctx, userID, exceptedSessionID); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func (a *Application) UserByID(ctx context.Context, userID uuid.UUID) (auth.User, error) {
	const op = "user by id"
	u, err := a.repo.UserByID(ctx, userID)
	if err != nil {
		return auth.User{}, fmt.Errorf("%s: %w", op, err)
	}

	return u, nil
}

func (a *Application) RefreshToken(ctx context.Context, currentToken string) (auth.LoginResponse, error) {
	const op = "refresh token"
	tokenHash := a.tokenSvc.HashToken(currentToken)

	var (
		newRefreshToken string
		userID          uuid.UUID
		userRole        auth.UserRole
		newSessionID    uuid.UUID
		err             error
	)

	newSessionID, err = newUUID()
	if err != nil {
		return auth.LoginResponse{}, fmt.Errorf("%s: %w", op, err)
	}

	err = a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		session, err := a.repo.SessionByRefreshTokenHash(txCtx, tokenHash)
		if err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				return errorsx.ErrUnauthorized
			}
			return fmt.Errorf("%s: %w", op, err)
		}

		if session.RevokedAt != nil {
			return errorsx.ErrUnauthorized
		}
		if session.ExpiresAt.Before(time.Now()) {
			return errorsx.ErrUnauthorized
		}

		user, err := a.repo.UserByID(txCtx, session.UserID)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		userID = user.ID
		userRole = user.Role

		if err := a.repo.RevokeSession(txCtx, session.ID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		newRefreshToken, err = a.tokenSvc.GenerateToken()
		if err != nil {
			return fmt.Errorf("%s: generate token: %w", op, errors.Join(errorsx.ErrInternal, err))
		}

		newSession := auth.Session{
			ID:               newSessionID,
			UserID:           session.UserID,
			RefreshTokenHash: a.tokenSvc.HashToken(newRefreshToken),
			UserAgent:        session.UserAgent,
			IPAddress:        session.IPAddress,
			ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		}

		if err := a.repo.CreateSession(txCtx, &newSession); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}
		newSessionID = newSession.ID

		return nil
	})
	if err != nil {
		return auth.LoginResponse{}, err
	}

	accessToken, err := a.tokenSvc.GenerateAccessToken(auth.AccessTokenClaims{
		UserID:    userID,
		SessionID: newSessionID,
		Role:      userRole,
	})
	if err != nil {
		return auth.LoginResponse{}, fmt.Errorf("%s: generate access token: %w", op, errors.Join(errorsx.ErrInternal, err))
	}

	return auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (a *Application) ChangePassword(ctx context.Context, input auth.ChangePasswordInput) error {
	const op = "change password"
	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := a.repo.UserByID(txCtx, input.UserID)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		matched, err := argon2id.ComparePasswordAndHash(input.CurrentPassword, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("%s: compare password: %w", op, errors.Join(errorsx.ErrInternal, err))
		}
		if !matched {
			return fmt.Errorf("%s: %w", op, errorsx.ErrUnauthorized)
		}

		newPasswordHash, err := argon2id.CreateHash(input.NewPassword, argon2id.DefaultParams)
		if err != nil {
			return fmt.Errorf("%s: create password hash: %w", op, errors.Join(errorsx.ErrInternal, err))
		}

		if err := a.repo.ChangePassword(txCtx, input.UserID, newPasswordHash); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if err := a.repo.RevokeSessionsByUserExcept(txCtx, input.UserID, input.CurrentSessionID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) RequestPasswordReset(ctx context.Context, email string) error {
	const op = "request password reset"
	email = strings.TrimSpace(strings.ToLower(email))

	var rawToken string

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := a.repo.UserByEmail(txCtx, email)
		if err != nil {
			if errors.Is(err, errorsx.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("%s: %w", op, err)
		}

		rawToken, err = a.tokenSvc.GenerateToken()
		if err != nil {
			return fmt.Errorf("%s: generate token: %w", op, errors.Join(errorsx.ErrInternal, err))
		}

		tokenID, err := newUUID()
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		token := &auth.PasswordResetToken{
			ID:        tokenID,
			UserID:    user.ID,
			TokenHash: a.tokenSvc.HashToken(rawToken),
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}

		if err := a.repo.CreatePasswordResetToken(txCtx, token); err != nil {
			return fmt.Errorf("%s: %w", op, err)
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
		return fmt.Errorf("%s: %w", op, err)
	}

	return nil
}

func (a *Application) ResetPassword(ctx context.Context, resetPasswordToken string, newPassword string) error {
	const op = "reset password"
	tokenHash := a.tokenSvc.HashToken(resetPasswordToken)

	err := a.tx.WithinTx(ctx, func(txCtx context.Context) error {
		token, err := a.repo.PasswordResetTokenByHash(txCtx, tokenHash)
		if err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if token.UsedAt != nil {
			return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
		}
		if token.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("%s: %w", op, errorsx.ErrInvalidArgument)
		}

		newPasswordHash, err := argon2id.CreateHash(newPassword, argon2id.DefaultParams)
		if err != nil {
			return fmt.Errorf("%s: create password hash: %w", op, errors.Join(errorsx.ErrInternal, err))
		}

		if err := a.repo.ChangePassword(txCtx, token.UserID, newPasswordHash); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if err := a.repo.MarkPasswordResetTokenUsed(txCtx, token.ID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		if err := a.repo.RevokeSessionsByUser(txCtx, token.UserID); err != nil {
			return fmt.Errorf("%s: %w", op, err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (a *Application) sendVerificationEmail(ctx context.Context, to, token string) error {
	const op = "send verification email"
	link := a.buildTokenLink("verify-email", token)
	msg := templates.Verification(to, a.appName, link)
	if err := a.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	a.logger.Info("verification email sent", "to", to)
	return nil
}

func (a *Application) sendPasswordResetEmail(ctx context.Context, to, token string) error {
	const op = "send password reset email"
	link := a.buildTokenLink("reset-password", token)
	msg := templates.PasswordReset(to, a.appName, link)
	if err := a.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	a.logger.Info("password reset email sent", "to", to)
	return nil
}

func (a *Application) buildTokenLink(path, token string) string {
	escaped := url.QueryEscape(token)
	return fmt.Sprintf("%s/%s?token=%s", a.appFrontendURL, path, escaped)
}

func newUUID() (uuid.UUID, error) {
	const op = "generate uuid"
	id, err := uuid.NewV7()
	if err != nil {
		return uuid.Nil, fmt.Errorf("%s: %w", op, err)
	}
	return id, nil
}
