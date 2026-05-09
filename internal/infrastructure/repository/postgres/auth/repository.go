package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pulsoats/core/errorsx"
	"github.com/pulsoats/main/internal/domain/auth"
	"github.com/pulsoats/main/internal/infrastructure/repository/postgres"
)

type Repository struct {
	qp postgres.QuerierProvider
}

func NewPostgresRepository(qp postgres.QuerierProvider) *Repository {
	return &Repository{qp: qp}
}

func (r *Repository) CreateInviteToken(ctx context.Context, token auth.InviteToken) error {
	const query = `
	INSERT INTO auth.invite_tokens (id, token_hash, created_by, expires_at)
	VALUES ($1, $2, $3, $4);
	`

	q := r.qp.Get(ctx)

	_, err := q.Exec(ctx, query, token.ID, token.TokenHash, token.CreatedBy, token.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError

		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create invite token: %w", errorsx.ErrNotFound)
		}

		return fmt.Errorf("create invite token: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *Repository) InviteTokenByHash(ctx context.Context, tokenHash string) (auth.InviteToken, error) {
	const query = `
	SELECT id, token_hash, created_by, expires_at, used_by, used_at, created_at
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
		&token.UsedBy,
		&token.UsedAt,
		&token.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.InviteToken{}, fmt.Errorf("invite token by hash: %w", errorsx.ErrNotFound)
		}
		return auth.InviteToken{}, fmt.Errorf("invite token by hash: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return token, nil
}

func (r *Repository) ListInviteTokens(ctx context.Context) ([]auth.InviteToken, error) {
	const query = `
	SELECT id, token_hash, created_by, expires_at, used_by, used_at, created_at
	FROM auth.invite_tokens
	ORDER BY created_at DESC;
	`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list invite tokens: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	tokens, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (auth.InviteToken, error) {
		var token auth.InviteToken
		err := row.Scan(
			&token.ID,
			&token.TokenHash,
			&token.CreatedBy,
			&token.ExpiresAt,
			&token.UsedBy,
			&token.UsedAt,
			&token.CreatedAt,
		)
		if err != nil {
			return auth.InviteToken{}, fmt.Errorf("list invite tokens: %w", errors.Join(errorsx.ErrInternal, err))
		}
		return token, nil
	})
	if err != nil {
		return nil, err
	}

	return tokens, nil
}

func (r *Repository) MarkInviteTokenUsed(ctx context.Context, id uuid.UUID, userID uuid.UUID) error {
	const query = `
	UPDATE auth.invite_tokens
	SET used_by = $2,
	    used_at = now()
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, id, userID)
	if err != nil {
		return fmt.Errorf("mark invite token used: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark invite token used: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) RevokeInviteToken(ctx context.Context, id uuid.UUID) error {
	const query = `
	DELETE FROM auth.invite_tokens
	WHERE id = $1 AND used_by IS NULL AND used_at IS NULL;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke invite token: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("revoke invite token: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) CreateUser(ctx context.Context, user *auth.User) error {
	const query = `
	INSERT INTO auth.users (id, email, password_hash, role)
	VALUES ($1, $2, $3, $4);
	`

	q := r.qp.Get(ctx)

	role := user.Role
	if role == "" {
		role = auth.RoleUser
	}

	_, err := q.Exec(ctx, query, user.ID, user.Email, user.PasswordHash, role)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return fmt.Errorf("create user: %w", errorsx.ErrAlreadyExists)
		}
		return fmt.Errorf("create user: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *Repository) UserByID(ctx context.Context, id uuid.UUID) (auth.User, error) {
	const query = `
	SELECT id, email, password_hash, role, email_verified_at, status, locked_until, created_at, updated_at
	FROM auth.users
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	u := auth.User{}

	var rawRole string
	err := q.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&rawRole,
		&u.EmailVerifiedAt,
		&u.Status,
		&u.LockedUntil,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, fmt.Errorf("user by id: %w", errorsx.ErrNotFound)
		}
		return auth.User{}, fmt.Errorf("user by id: %w", errors.Join(errorsx.ErrInternal, err))
	}

	role, err := auth.ParseUserRole(rawRole)
	if err != nil {
		return auth.User{}, fmt.Errorf("user by id: %w", err)
	}
	u.Role = role

	return u, nil
}

func (r *Repository) UserByEmail(ctx context.Context, email string) (auth.User, error) {
	const query = `
	SELECT id, email, password_hash, role, email_verified_at, status, locked_until, created_at, updated_at
	FROM auth.users
	WHERE email = $1;
	`

	q := r.qp.Get(ctx)
	u := auth.User{}

	var rawRole string
	err := q.QueryRow(ctx, query, email).Scan(
		&u.ID,
		&u.Email,
		&u.PasswordHash,
		&rawRole,
		&u.EmailVerifiedAt,
		&u.Status,
		&u.LockedUntil,
		&u.CreatedAt,
		&u.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return auth.User{}, fmt.Errorf("user by email: %w", errorsx.ErrNotFound)
		}
		return auth.User{}, fmt.Errorf("user by email: %w", errors.Join(errorsx.ErrInternal, err))
	}

	role, err := auth.ParseUserRole(rawRole)
	if err != nil {
		return auth.User{}, fmt.Errorf("user by email: %w", err)
	}
	u.Role = role

	return u, nil
}

func (r *Repository) ChangePassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	const query = `
	UPDATE auth.users
	SET password_hash = $2
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)

	tag, err := q.Exec(ctx, query, userID, passwordHash)
	if err != nil {
		return fmt.Errorf("change password: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("change password: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) SetUserRole(ctx context.Context, userID uuid.UUID, role auth.UserRole) error {
	const query = `
	UPDATE auth.users
	SET role = $2
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, userID, role)
	if err != nil {
		return fmt.Errorf("set user role: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("set user role: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) CreatePasswordResetToken(ctx context.Context, token *auth.PasswordResetToken) error {
	const query = `
	INSERT INTO auth.password_reset_tokens (id, user_id, token_hash, expires_at)
	VALUES ($1, $2, $3, $4)
	RETURNING created_at;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt).
		Scan(&token.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create password reset token: %w", errorsx.ErrNotFound)
		}
		return fmt.Errorf("create password reset token: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *Repository) PasswordResetTokenByHash(ctx context.Context, tokenHash string) (auth.PasswordResetToken, error) {
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
			return auth.PasswordResetToken{}, fmt.Errorf("password reset token by hash: %w", errorsx.ErrNotFound)
		}
		return auth.PasswordResetToken{}, fmt.Errorf("password reset token by hash: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return token, nil
}

func (r *Repository) MarkPasswordResetTokenUsed(ctx context.Context, id uuid.UUID) error {
	const query = `
	UPDATE auth.password_reset_tokens
	SET used_at = now()
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark password reset token used: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark password reset token used: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) CreateSession(ctx context.Context, session *auth.Session) error {
	const query = `
	INSERT INTO auth.user_sessions (id, user_id, refresh_token_hash, user_agent, ip_address, expires_at)
	VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING created_at;
	`

	q := r.qp.Get(ctx)

	err := q.QueryRow(
		ctx,
		query,
		session.ID,
		session.UserID,
		session.RefreshTokenHash,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
	).Scan(&session.CreatedAt)
	if err != nil {
		return fmt.Errorf("create session: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return nil
}

func (r *Repository) SessionByRefreshTokenHash(ctx context.Context, refreshTokenHash string) (auth.Session, error) {
	const query = `
	SELECT id, user_id, refresh_token_hash, user_agent, ip_address::text, expires_at, revoked_at, created_at
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
			return auth.Session{}, fmt.Errorf("session by refresh token hash: %w", errorsx.ErrNotFound)
		}
		return auth.Session{}, fmt.Errorf("session by refresh token hash: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return s, nil
}

func (r *Repository) ListActiveSessionsByUserID(ctx context.Context, userID uuid.UUID) ([]auth.Session, error) {
	const query = `
	SELECT id, user_id, refresh_token_hash, user_agent, ip_address::text, expires_at, revoked_at, created_at
	FROM auth.user_sessions
	WHERE user_id = $1
	AND revoked_at IS NULL
	AND expires_at > NOW();
	`

	q := r.qp.Get(ctx)

	rows, err := q.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list active sessions by user id: %w", errors.Join(errorsx.ErrInternal, err))
	}
	defer rows.Close()

	sessions, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (auth.Session, error) {
		var s auth.Session
		if err := r.Scan(
			&s.ID,
			&s.UserID,
			&s.RefreshTokenHash,
			&s.UserAgent,
			&s.IPAddress,
			&s.ExpiresAt,
			&s.RevokedAt,
			&s.CreatedAt,
		); err != nil {
			return auth.Session{}, fmt.Errorf("list active sessions by user id: %w", errors.Join(errorsx.ErrInternal, err))
		}

		return s, nil
	})
	if err != nil {
		return nil, err
	}
	return sessions, nil
}

func (r *Repository) RevokeSession(ctx context.Context, id uuid.UUID) error {
	const query = `
	UPDATE auth.user_sessions
	SET revoked_at = now()
	WHERE id = $1
	  AND revoked_at IS NULL;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("revoke session: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("revoke session: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) RevokeSessionsByUser(ctx context.Context, userID uuid.UUID) error {
	const query = `
	UPDATE auth.user_sessions
	SET revoked_at = now()
	WHERE user_id = $1
	  AND revoked_at IS NULL;
	`

	q := r.qp.Get(ctx)
	_, err := q.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("revoke sessions by user: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (r *Repository) RevokeSessionsByUserExcept(ctx context.Context, userID uuid.UUID, exceptedSessionID uuid.UUID) error {
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
		return fmt.Errorf("revoke sessions by user except: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (r *Repository) CreateEmailVerificationToken(ctx context.Context, token *auth.EmailVerificationToken) error {
	const query = `
	INSERT INTO auth.email_verification_tokens (id, user_id, token_hash, expires_at)
	VALUES ($1, $2, $3, $4)
	RETURNING created_at;
	`

	q := r.qp.Get(ctx)
	err := q.QueryRow(ctx, query, token.ID, token.UserID, token.TokenHash, token.ExpiresAt).
		Scan(&token.CreatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return fmt.Errorf("create email verification token: %w", errorsx.ErrNotFound)
		}
		return fmt.Errorf("create email verification token: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}

func (r *Repository) EmailVerificationTokenByHash(ctx context.Context, tokenHash string) (auth.EmailVerificationToken, error) {
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
			return auth.EmailVerificationToken{}, fmt.Errorf("email verification token by hash: %w", errorsx.ErrNotFound)
		}
		return auth.EmailVerificationToken{}, fmt.Errorf("email verification token by hash: %w", errors.Join(errorsx.ErrInternal, err))
	}

	return t, nil
}

func (r *Repository) MarkEmailVerificationTokenUsed(ctx context.Context, id uuid.UUID) error {
	const query = `
	UPDATE auth.email_verification_tokens
	SET used_at = now()
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("mark email verification token used: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark email verification token used: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) MarkUserEmailVerified(ctx context.Context, userID uuid.UUID) error {
	const query = `
	UPDATE auth.users
	SET email_verified_at = NOW(),
	    status = 'active'
	WHERE id = $1;
	`

	q := r.qp.Get(ctx)
	tag, err := q.Exec(ctx, query, userID)
	if err != nil {
		return fmt.Errorf("mark user email verified: %w", errors.Join(errorsx.ErrInternal, err))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark user email verified: %w", errorsx.ErrNotFound)
	}
	return nil
}

func (r *Repository) CreateLoginAttempt(ctx context.Context, attempt auth.LoginAttempt) error {
	const query = `
	INSERT INTO auth.login_attempts (id, user_id, email, ip_address, user_agent, success, reason)
	VALUES ($1, $2, $3, $4, $5, $6, $7);
	`

	q := r.qp.Get(ctx)
	_, err := q.Exec(
		ctx,
		query,
		attempt.ID,
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
			return fmt.Errorf("create login attempt: %w", errorsx.ErrNotFound)
		}
		return fmt.Errorf("create login attempt: %w", errors.Join(errorsx.ErrInternal, err))
	}
	return nil
}
