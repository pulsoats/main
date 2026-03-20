package auth

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/main/internal/domain"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/domain/mailer"
	"github.com/pulsoats/main/internal/infrastructure/email/templates"
)

const defaultAppName = "TradeBot"

type service struct {
	repo        auth.Repository
	txManager   domain.TxManager
	jwtSecret   []byte
	emailSender mailer.Sender
	appBaseURL  string
	appName     string
}

type ServiceConfig struct {
	Repository  auth.Repository
	TxManager   domain.TxManager
	JWTSecret   []byte
	EmailSender mailer.Sender
	AppBaseURL  string
	AppName     string
}

func NewService(cfg ServiceConfig) (auth.Service, error) {
	if cfg.Repository == nil {
		return nil, fmt.Errorf("new auth service: %w: auth repository", derrors.ErrRequired)
	}
	if cfg.TxManager == nil {
		return nil, fmt.Errorf("new auth service: %w: tx manager", derrors.ErrRequired)
	}
	if len(cfg.JWTSecret) == 0 {
		return nil, fmt.Errorf("new auth service: %w: jwt secret", derrors.ErrRequired)
	}
	if cfg.EmailSender == nil {
		return nil, fmt.Errorf("new auth service: %w: email sender", derrors.ErrRequired)
	}
	baseURL := strings.TrimSpace(cfg.AppBaseURL)
	if baseURL == "" {
		return nil, fmt.Errorf("new auth service: %w: app base url", derrors.ErrRequired)
	}
	baseURL = strings.TrimRight(baseURL, "/")

	appName := strings.TrimSpace(cfg.AppName)
	if appName == "" {
		appName = defaultAppName
	}

	return &service{
		repo:        cfg.Repository,
		txManager:   cfg.TxManager,
		jwtSecret:   cfg.JWTSecret,
		emailSender: cfg.EmailSender,
		appBaseURL:  baseURL,
		appName:     appName,
	}, nil
}

func (s *service) InviteToken(ctx context.Context, userID int64) (string, error) {
	rawToken, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("create invite link: generate token: %w: %v", errorsx.ErrInternal, err)
	}

	token := auth.InviteToken{
		TokenHash: hashToken(rawToken),
		CreatedBy: userID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	if err := s.repo.CreateInviteToken(ctx, token); err != nil {
		return "", err
	}

	return rawToken, nil
}

func (s *service) Register(ctx context.Context, email, password, inviteToken string) error {
	if inviteToken == "" {
		return fmt.Errorf("register: %w", derrors.ErrRequired)
	}

	email = strings.ToLower(strings.TrimSpace(email))
	inviteTokenHash := hashToken(inviteToken)

	passwordHash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("register: create password hash: %w: %v", errorsx.ErrInternal, err)
	}

	rawVerificationToken, err := generateToken()
	if err != nil {
		return fmt.Errorf("register: generate verification token: %w: %v", errorsx.ErrInternal, err)
	}
	verificationTokenHash := hashToken(rawVerificationToken)

	user := auth.User{
		Email:        email,
		PasswordHash: passwordHash,
	}

	err = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		invite, err := s.repo.InviteTokenByHash(txCtx, inviteTokenHash)
		if err != nil {
			return fmt.Errorf("register: %w", err)
		}

		if invite.UsedAt != nil {
			return fmt.Errorf("register: %w", derrors.ErrInvalidArgument)
		}
		if invite.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("register: %w", derrors.ErrInvalidArgument)
		}

		if err := s.repo.CreateUser(txCtx, &user); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		verificationToken := auth.EmailVerificationToken{
			UserID:    user.ID,
			TokenHash: verificationTokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}

		if err := s.repo.CreateEmailVerificationToken(txCtx, &verificationToken); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		if err := s.repo.MarkInviteTokenUsed(txCtx, invite.ID); err != nil {
			return fmt.Errorf("register: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	if err = s.sendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		return fmt.Errorf("register: %w", err)
	}

	return nil
}

func (s *service) VerifyEmailByToken(ctx context.Context, emailVerificationToken string) error {
	tokenHash := hashToken(emailVerificationToken)

	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		token, err := s.repo.EmailVerificationTokenByHash(txCtx, tokenHash)
		if err != nil {
			return fmt.Errorf("verify email by token: %w", err)
		}

		if token.UsedAt != nil {
			return fmt.Errorf("verify email by token: %w", derrors.ErrInvalidArgument)
		}
		if token.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("verify email by token: %w", derrors.ErrInvalidArgument)
		}

		if err := s.repo.MarkEmailVerificationTokenUsed(txCtx, token.ID); err != nil {
			return fmt.Errorf("verify email by token: %w", err)
		}

		if err := s.repo.MarkUserEmailVerified(txCtx, token.UserID); err != nil {
			return fmt.Errorf("verify email by token: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *service) EnsureRoot(ctx context.Context, email, password string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	password = strings.TrimSpace(password)
	if email == "" || password == "" {
		return fmt.Errorf("ensure root: %w: email/password", derrors.ErrRequired)
	}

	existing, err := s.repo.UserByEmail(ctx, email)
	if err == nil {
		if existing.Role != auth.RoleAdmin {
			if err := s.repo.SetUserRole(ctx, existing.ID, auth.RoleAdmin); err != nil {
				return fmt.Errorf("ensure root: set role: %w", err)
			}
		}
		return nil
	}
	if !errors.Is(err, derrors.ErrNotFound) {
		return fmt.Errorf("ensure root: %w", err)
	}

	passwordHash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("ensure root: create password hash: %w: %v", errorsx.ErrInternal, err)
	}

	rawVerificationToken, err := generateToken()
	if err != nil {
		return fmt.Errorf("ensure root: generate verification token: %w: %v", errorsx.ErrInternal, err)
	}
	verificationTokenHash := hashToken(rawVerificationToken)

	user := auth.User{
		Email:        email,
		PasswordHash: passwordHash,
		Role:         auth.RoleAdmin,
	}

	err = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		if err := s.repo.CreateUser(txCtx, &user); err != nil {
			return fmt.Errorf("ensure root: create user: %w", err)
		}

		token := auth.EmailVerificationToken{
			UserID:    user.ID,
			TokenHash: verificationTokenHash,
			ExpiresAt: time.Now().Add(time.Hour),
		}
		if err := s.repo.CreateEmailVerificationToken(txCtx, &token); err != nil {
			return fmt.Errorf("ensure root: create verification token: %w", err)
		}
		return nil
	})
	if err != nil {
		return err
	}

	if err = s.sendVerificationEmail(ctx, email, rawVerificationToken); err != nil {
		return fmt.Errorf("ensure root: %w", err)
	}
	return nil
}

func (s *service) Login(ctx context.Context, input auth.LoginInput) (resp auth.LoginResponse, err error) {
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
		_ = s.repo.CreateLoginAttempt(ctx, attempt)
	}()

	var (
		userID          int64
		userRole        auth.Role
		rawRefreshToken string
	)

	err = s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.UserByEmail(txCtx, input.Email)
		if err != nil {
			if errors.Is(err, derrors.ErrNotFound) {
				reason := "invalid_credentials"
				attempt.Reason = &reason
				return derrors.ErrUnauthorized
			}
			return fmt.Errorf("login: %w", err)
		}

		userID = user.ID
		userRole = user.Role
		attempt.UserID = &user.ID

		if user.LockedUntil != nil && user.LockedUntil.After(time.Now()) {
			reason := "account_locked"
			attempt.Reason = &reason
			return derrors.ErrUnauthorized
		}
		if user.Status == "disabled" || user.Status == "locked" {
			reason := "account_disabled"
			attempt.Reason = &reason
			return derrors.ErrUnauthorized
		}
		if user.Status == "pending" {
			reason := "email_not_verified"
			attempt.Reason = &reason
			return derrors.ErrUnauthorized
		}

		matched, err := argon2id.ComparePasswordAndHash(input.Password, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("login: compare password: %w: %v", errorsx.ErrInternal, err)
		}
		if !matched {
			reason := "invalid_credentials"
			attempt.Reason = &reason
			return derrors.ErrUnauthorized
		}

		rawRefreshToken, err = generateToken()
		if err != nil {
			return fmt.Errorf("login: generate refresh token: %w: %v", errorsx.ErrInternal, err)
		}

		session := auth.Session{
			UserID:           user.ID,
			RefreshTokenHash: hashToken(rawRefreshToken),
			UserAgent:        input.UserAgent,
			IPAddress:        input.IPAddress,
			ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		}

		if err := s.repo.CreateSession(txCtx, &session); err != nil {
			return fmt.Errorf("login: %w", err)
		}

		return nil
	})
	if err != nil {
		return auth.LoginResponse{}, err
	}

	accessToken, err := generateAccessToken(AccessTokenConfig{
		UserID:  userID,
		Role:    string(userRole),
		Expires: 15 * time.Minute,
		Secret:  s.jwtSecret,
	})
	if err != nil {
		reason := "access_token_failed"
		attempt.Reason = &reason
		return auth.LoginResponse{}, fmt.Errorf("login: generate access token: %w: %v", errorsx.ErrInternal, err)
	}

	attempt.Success = true

	return auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: rawRefreshToken,
	}, nil
}

func (s *service) RefreshToken(ctx context.Context, currentToken string) (auth.LoginResponse, error) {
	tokenHash := hashToken(currentToken)

	var (
		newRefreshToken string
		userID          int64
		userRole        auth.Role
	)

	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		session, err := s.repo.SessionByRefreshTokenHash(txCtx, tokenHash)
		if err != nil {
			if errors.Is(err, derrors.ErrNotFound) {
				return derrors.ErrUnauthorized
			}
			return fmt.Errorf("refresh token: %w", err)
		}

		if session.RevokedAt != nil {
			return derrors.ErrUnauthorized
		}
		if session.ExpiresAt.Before(time.Now()) {
			return derrors.ErrUnauthorized
		}

		user, err := s.repo.UserByID(txCtx, session.UserID)
		if err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}

		userID = user.ID
		userRole = user.Role

		if err := s.repo.RevokeSession(txCtx, session.ID); err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}

		newRefreshToken, err = generateToken()
		if err != nil {
			return fmt.Errorf("refresh token: generate token: %w: %v", errorsx.ErrInternal, err)
		}

		newSession := auth.Session{
			UserID:           session.UserID,
			RefreshTokenHash: hashToken(newRefreshToken),
			UserAgent:        session.UserAgent,
			IPAddress:        session.IPAddress,
			ExpiresAt:        time.Now().Add(30 * 24 * time.Hour),
		}

		if err := s.repo.CreateSession(txCtx, &newSession); err != nil {
			return fmt.Errorf("refresh token: %w", err)
		}

		return nil
	})
	if err != nil {
		return auth.LoginResponse{}, err
	}

	accessToken, err := generateAccessToken(AccessTokenConfig{
		UserID:  userID,
		Role:    string(userRole),
		Expires: 15 * time.Minute,
		Secret:  s.jwtSecret,
	})
	if err != nil {
		return auth.LoginResponse{}, fmt.Errorf("refresh token: generate access token: %w: %w", errorsx.ErrInternal, err)
	}

	return auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
	}, nil
}

func (s *service) ChangePassword(ctx context.Context, input auth.ChangePasswordInput) error {
	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.UserByID(txCtx, input.UserID)
		if err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		matched, err := argon2id.ComparePasswordAndHash(input.CurrentPassword, user.PasswordHash)
		if err != nil {
			return fmt.Errorf("change password: compare password: %w: %v", errorsx.ErrInternal, err)
		}
		if !matched {
			return fmt.Errorf("change password: %w", derrors.ErrUnauthorized)
		}

		newPasswordHash, err := argon2id.CreateHash(input.NewPassword, argon2id.DefaultParams)
		if err != nil {
			return fmt.Errorf("change password: create password hash: %w: %v", errorsx.ErrInternal, err)
		}

		if err := s.repo.ChangePassword(txCtx, input.UserID, newPasswordHash); err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		if err := s.repo.RevokeSessionsByUserExcept(txCtx, input.UserID, input.CurrentSessionID); err != nil {
			return fmt.Errorf("change password: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *service) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))

	var rawToken string

	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		user, err := s.repo.UserByEmail(txCtx, email)
		if err != nil {
			if errors.Is(err, derrors.ErrNotFound) {
				return nil
			}
			return fmt.Errorf("request password reset: %w", err)
		}

		rawToken, err = generateToken()
		if err != nil {
			return fmt.Errorf("request password reset: generate token: %w: %v", errorsx.ErrInternal, err)
		}

		token := &auth.PasswordResetToken{
			UserID:    user.ID,
			TokenHash: hashToken(rawToken),
			ExpiresAt: time.Now().Add(15 * time.Minute),
		}

		if err := s.repo.CreatePasswordResetToken(txCtx, token); err != nil {
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

	if err = s.sendPasswordResetEmail(ctx, email, rawToken); err != nil {
		return fmt.Errorf("request password reset: %w", err)
	}

	return nil
}

func (s *service) ResetPassword(ctx context.Context, resetPasswordToken string, newPassword string) error {
	tokenHash := hashToken(resetPasswordToken)

	err := s.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		token, err := s.repo.PasswordResetTokenByHash(txCtx, tokenHash)
		if err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		if token.UsedAt != nil {
			return fmt.Errorf("reset password: %w", derrors.ErrInvalidArgument)
		}
		if token.ExpiresAt.Before(time.Now()) {
			return fmt.Errorf("reset password: %w", derrors.ErrInvalidArgument)
		}

		newPasswordHash, err := argon2id.CreateHash(newPassword, argon2id.DefaultParams)
		if err != nil {
			return fmt.Errorf("reset password: create password hash: %w: %v", errorsx.ErrInternal, err)
		}

		if err := s.repo.ChangePassword(txCtx, token.UserID, newPasswordHash); err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		if err := s.repo.MarkPasswordResetTokenUsed(txCtx, token.ID); err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		if err := s.repo.RevokeSessionsByUser(txCtx, token.UserID); err != nil {
			return fmt.Errorf("reset password: %w", err)
		}

		return nil
	})
	if err != nil {
		return err
	}

	return nil
}

func (s *service) sendVerificationEmail(ctx context.Context, to, token string) error {
	link := s.buildTokenLink("verify", token)
	msg := templates.Verification(to, s.appName, link)
	if err := s.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send verification email: %w", err)
	}
	return nil
}

func (s *service) sendPasswordResetEmail(ctx context.Context, to, token string) error {
	link := s.buildTokenLink("reset-password", token)
	msg := templates.PasswordReset(to, s.appName, link)
	if err := s.emailSender.Send(ctx, msg); err != nil {
		return fmt.Errorf("send password reset email: %w", err)
	}
	return nil
}

func (s *service) buildTokenLink(path, token string) string {
	escaped := url.QueryEscape(token)
	return fmt.Sprintf("%s/%s?token=%s", s.appBaseURL, path, escaped)
}
