CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE IF NOT EXISTS auth.users
(
    id                 BIGSERIAL PRIMARY KEY,
    email              TEXT        NOT NULL,
    password_hash      TEXT        NOT NULL,
    role               TEXT        NOT NULL DEFAULT 'user',
    status             TEXT        NOT NULL DEFAULT 'pending',
    failed_login_count INTEGER     NOT NULL DEFAULT 0,
    locked_until       TIMESTAMPTZ NULL,
    last_login_at      TIMESTAMPTZ NULL,
    email_verified_at  TIMESTAMPTZ NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT users_status_check
        CHECK (status IN ('pending', 'active', 'disabled', 'blocked'))
        CHECK (role IN ('user', 'admin'))
);

CREATE UNIQUE INDEX IF NOT EXISTS users_email_unique_idx
    ON auth.users (email);

CREATE INDEX IF NOT EXISTS users_status_idx
    ON auth.users (status);


CREATE TABLE IF NOT EXISTS auth.user_sessions
(
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT      NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    refresh_token_hash TEXT        NOT NULL UNIQUE,
    user_agent         TEXT        NULL,
    ip_address         INET        NULL,
    expires_at         TIMESTAMPTZ NOT NULL,
    revoked_at         TIMESTAMPTZ NULL,
    replaced_by_id     BIGINT      NULL REFERENCES auth.user_sessions (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT user_sessions_expires_after_created_check
        CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS user_sessions_user_id_idx
    ON auth.user_sessions (user_id);

CREATE INDEX IF NOT EXISTS user_sessions_expires_at_idx
    ON auth.user_sessions (expires_at);

CREATE INDEX IF NOT EXISTS user_sessions_revoked_at_idx
    ON auth.user_sessions (revoked_at);


CREATE TABLE IF NOT EXISTS auth.email_verification_tokens
(
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT email_verification_tokens_expires_after_created_check
        CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS email_verification_tokens_user_id_idx
    ON auth.email_verification_tokens (user_id);

CREATE INDEX IF NOT EXISTS email_verification_tokens_expires_at_idx
    ON auth.email_verification_tokens (expires_at);


CREATE TABLE IF NOT EXISTS auth.password_reset_tokens
(
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES auth.users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT password_reset_tokens_expires_after_created_check
        CHECK (expires_at > created_at)
);

CREATE INDEX IF NOT EXISTS password_reset_tokens_user_id_idx
    ON auth.password_reset_tokens (user_id);

CREATE INDEX IF NOT EXISTS password_reset_tokens_expires_at_idx
    ON auth.password_reset_tokens (expires_at);

CREATE TABLE IF NOT EXISTS auth.login_attempts
(
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NULL REFERENCES auth.users (id) ON DELETE SET NULL,
    email      TEXT        NOT NULL,
    ip_address INET        NULL,
    user_agent TEXT        NULL,
    success    BOOLEAN     NOT NULL,
    reason     TEXT        NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX login_attempts_email_idx
    ON auth.login_attempts (email);

CREATE INDEX login_attempts_created_at_idx
    ON auth.login_attempts (created_at);

CREATE TABLE IF NOT EXISTS auth.invite_tokens
(
    id         BIGSERIAL PRIMARY KEY,
    token_hash TEXT        NOT NULL UNIQUE,
    created_by BIGINT REFERENCES auth.users (id),
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    CONSTRAINT invite_tokens_expires_after_created_check
        CHECK (expires_at > created_at)
);

CREATE INDEX invite_tokens_token_hash_idx
    ON auth.invite_tokens (token_hash);
