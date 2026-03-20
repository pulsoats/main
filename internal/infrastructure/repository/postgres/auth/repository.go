package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pulsoats/core/domain/derrors"
	"github.com/pulsoats/core/lib/errorsx"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type repo struct {
	qp postgres.QuerierProvider
}

func NewPostgresRepository(qp postgres.QuerierProvider) auth.Repository {
	return &repo{qp: qp}
}

func (r *repo) CreateInviteToken(ctx context.Context, token auth.InviteToken) error {
	const query = `
	INSERT INTO auth.invite_tokens (token_hash, created_by, expires_at)
	VALUES ($1, $2, $3);
	`

	q := r.qp.Get(ctx)

	_, err := q.Exec(ctx, query, token.TokenHash, token.CreatedBy, token.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create invite token: %w", derrors.ErrNotFound)
		}

		return fmt.Errorf("create invite token: %w: %v", errorsx.ErrInternal, err)
	}

	return nil
}

func (r *repo) InviteTokenByHash(ctx context.Context, tokenHash string) (auth.InviteToken, error) {
	const query = `
	SELECT id, token_hash, created_by, expires_at, used_at, created_at
	FROM auth.invite_tokens
	WHERE token_hash = $1;
	`

	q := r.qp.Get(ctx)
	var token auth.InviteToken
	err := q.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.TokenHash,
		&token.CreatedBy,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.InviteToken{}, fmt.Errorf("invite token by hash: %w", derrors.ErrNotFound)
		}
		return auth.InviteToken{}, fmt.Errorf("invite token by hash: %w: %v", errorsx.ErrInternal, err)
	}
	return token, nil
}

func (r *repo) MarkInviteTokenUsed(ctx context.Context, id int64) error {
	const query = `
	UPDATE auth.invite_tokens
	SET used_at = now()
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark invite token used: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark invite token used: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) CreateUser(ctx context.Context, user *auth.User) error {
	const query = `
	INSERT INTO auth.users (email, password_hash, role, status)
	VALUES ($1, $2, $3, 'pending')
	RETURNING id;
	`

	q := r.qp.Get(ctx)

	role := user.Role
	if role == "" {
		role = auth.RoleUser
	}

	err := q.QueryRow(ctx, query, user.Email, user.PasswordHash, role).Scan(&user.ID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create user: %w", derrors.ErrAlreadyExists)
		}
		return fmt.Errorf("create user: %w: %v", errorsx.ErrInternal, err)
	}

	return nil
}

func (r *repo) UserByID(ctx context.Context, id int64) (auth.User, error) {
	const query = `
	SELECT id, email, password_hash, role, email_verified_at, status, locked_until, created_at, updated_at
	FROM auth.users
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	u := auth.User{}

	err := q.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.EmailVerifiedAt,
		&u.Status,
		&u.LockedUntil,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, fmt.Errorf("user by id: %w", derrors.ErrNotFound)
		}
		return auth.User{}, fmt.Errorf("user by id: %w: %v", errorsx.ErrInternal, err)
	}

	return u, nil
}

func (r *repo) UserByEmail(ctx context.Context, email string) (auth.User, error) {
	const query = `
	SELECT id, email, password_hash, role, email_verified_at, status, locked_until, created_at, updated_at
	FROM auth.users
	WHERE email = $1;
	`

	q := r.qp.Get(ctx)
	u := auth.User{}

	err := q.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&u.Role,
		&u.EmailVerifiedAt,
		&u.Status,
		&u.LockedUntil,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, fmt.Errorf("user by email: %w", derrors.ErrNotFound)
		}
		return auth.User{}, fmt.Errorf("user by email", err)
	}

	return u, nil
}

func (r *repo) ChangePassword(ctx context.Context, userID int64, passwordHash string) error {
	const query = `
	UPDATE auth.users
	SET password_hash = $2
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("change password: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("change password: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) SetUserRole(ctx context.Context, userID int64, role auth.Role) error {
	const query = `
	UPDATE auth.users
	SET role = $2
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, userID, role)
	if err != nil {
		return fmt.Errorf("set user role: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set user role: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) CreatePasswordResetToken(ctx context.Context, token *auth.PasswordResetToken) error {
	const query = `
	INSERT INTO auth.password_reset_tokens (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	RETURNING id, created_at;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query, token.UserID, token.TokenHash, token.ExpiresAt).
		Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create password reset token: %w", derrors.ErrNotFound)
		}
		return fmt.Errorf("create password reset token: %w: %v", errorsx.ErrInternal, err)
	}

	return nil
}

func (r *repo) PasswordResetTokenByHash(ctx context.Context, tokenHash string) (auth.PasswordResetToken, error) {
	const query = `
	SELECT id, user_id, token_hash, expires_at, used_at, created_at
	FROM auth.password_reset_tokens
	WHERE token_hash = $1;
	`

	q := r.qp.Get(ctx)

	var token auth.PasswordResetToken
	err := q.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID,
		&token.UserID,
		&token.TokenHash,
		&token.ExpiresAt,
		&token.UsedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.PasswordResetToken{}, fmt.Errorf("password reset token by hash: %w", derrors.ErrNotFound)
		}
		return auth.PasswordResetToken{}, fmt.Errorf("password reset token by hash: %w: %v", errorsx.ErrInternal, err)
	}

	return token, nil
}

func (r *repo) MarkPasswordResetTokenUsed(ctx context.Context, id int64) error {
	const query = `
	UPDATE auth.password_reset_tokens
	SET used_at = now()
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark password reset token used: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark password reset token used: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) CreateSession(ctx context.Context, session *auth.Session) error {
	const query = `
	INSERT INTO auth.user_sessions (user_id, refresh_token_hash, user_agent, ip_address, expires_at)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(
		ctx,
		query,
		session.UserID,
		session.RefreshTokenHash,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
	).Scan(&session.ID)
	if err != nil {
		return fmt.Errorf("create session: %w: %v", errorsx.ErrInternal, err)
	}

	return nil
}

func (r *repo) SessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (auth.Session, error) {
	const query = `
	SELECT id, user_id, refresh_token_hash, user_agent, ip_address, expires_at, revoked_at, created_at
	FROM auth.user_sessions
	WHERE refresh_token_hash = $1;
	`

	q := r.qp.Get(ctx)
	s := auth.Session{}

	err := q.QueryRow(ctx, query, refreshTokenHash).Scan(
		&s.ID,
		&s.UserID,
		&s.RefreshTokenHash,
		&s.UserAgent,
		&s.IPAddress,
		&s.ExpiresAt,
		&s.RevokedAt,
		&s.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.Session{}, fmt.Errorf("session by refresh token hash: %w", derrors.ErrNotFound)
		}
		return auth.Session{}, fmt.Errorf("session by refresh token hash: %w: %v", errorsx.ErrInternal, err)
	}

	return s, nil
}

func (r *repo) RevokeSession(ctx context.Context, id int64) error {
	const query = `
	UPDATE auth.user_sessions
	SET revoked_at = now()
	WHERE id = $1
	  AND revoked_at IS NULL;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke session: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("revoke session: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) RevokeSessionsByUser(ctx context.Context, userID int64) error {
	const query = `
	UPDATE auth.user_sessions
	SET revoked_at = now()
	WHERE user_id = $1
	  AND revoked_at IS NULL;
	`

	q := r.qp.Get(ctx)
	_, err := q.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("revoke sessions by user: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}

func (r *repo) RevokeSessionsByUserExcept(ctx context.Context, userID int64, exceptedSessionID int64) error {
	const query = `
	UPDATE auth.user_sessions
	SET revoked_at = now()
	WHERE user_id = $1
	  AND id <> $2
	  AND revoked_at IS NULL;
	`

	q := r.qp.Get(ctx)
	_, err := q.Exec(ctx, query, userID, exceptedSessionID)
	if err != nil {
		return fmt.Errorf("revoke sessions by user except: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}

func (r *repo) CreateEmailVerificationToken(ctx context.Context, token *auth.EmailVerificationToken) error {
	const query = `
	INSERT INTO auth.email_verification_tokens (user_id, token_hash, expires_at)
	VALUES ($1, $2, $3)
	RETURNING id, created_at;
	`

	q := r.qp.Get(ctx)
	err := q.QueryRow(ctx, query, token.UserID, token.TokenHash, token.ExpiresAt).
		Scan(&token.ID, &token.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create email verification token: %w", derrors.ErrNotFound)
		}
		return fmt.Errorf("create email verification token: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}

func (r *repo) EmailVerificationTokenByHash(ctx context.Context, tokenHash string) (auth.EmailVerificationToken, error) {
	const query = `
	SELECT id, user_id, token_hash, expires_at, used_at, created_at
	FROM auth.email_verification_tokens
	WHERE token_hash = $1;
	`

	q := r.qp.Get(ctx)
	t := auth.EmailVerificationToken{}

	err := q.QueryRow(ctx, query, tokenHash).Scan(
		&t.ID,
		&t.UserID,
		&t.TokenHash,
		&t.ExpiresAt,
		&t.UsedAt,
		&t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.EmailVerificationToken{}, fmt.Errorf("email verification token by hash: %w", derrors.ErrNotFound)
		}
		return auth.EmailVerificationToken{}, fmt.Errorf("email verification token by hash: %w: %v", errorsx.ErrInternal, err)
	}

	return t, nil
}

func (r *repo) MarkEmailVerificationTokenUsed(ctx context.Context, id int64) error {
	const query = `
	UPDATE auth.email_verification_tokens
	SET used_at = now()
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark email verification token used: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark email verification token used: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) MarkUserEmailVerified(ctx context.Context, userID int64) error {
	const query = `
	UPDATE auth.users
	SET email_verified_at = now(),
	    status = 'active'
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("mark user email verified: %w: %v", errorsx.ErrInternal, err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark user email verified: %w", derrors.ErrNotFound)
	}
	return nil
}

func (r *repo) CreateLoginAttempt(ctx context.Context, attempt auth.LoginAttempt) error {
	const query = `
	INSERT INTO auth.login_attempts (user_id, email, ip_address, user_agent, success, reason)
	VALUES ($1, $2, $3, $4, $5, $6);
	`

	q := r.qp.Get(ctx)
	_, err := q.Exec(
		ctx,
		query,
		attempt.UserID,
		attempt.Email,
		attempt.IPAddress,
		attempt.UserAgent,
		attempt.Success,
		attempt.Reason,
	)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create login attempt: %w", derrors.ErrNotFound)
		}
		return fmt.Errorf("create login attempt: %w: %v", errorsx.ErrInternal, err)
	}
	return nil
}
