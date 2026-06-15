CREATE TABLE IF NOT EXISTS exchange_accounts
(
    id             UUID PRIMARY KEY,
    exchange       VARCHAR(10) NOT NULL,
    name           TEXT        NOT NULL,
    email          TEXT        NOT NULL,
    api_key_enc    BYTEA       NOT NULL,
    api_secret_enc BYTEA,
    passphrase_enc BYTEA,
    expires_at     TIMESTAMPTZ,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (exchange, name)
);

CREATE TABLE IF NOT EXISTS nodes
(
    id          UUID PRIMARY KEY,
    name        TEXT        NOT NULL,
    exchange    VARCHAR(10) NOT NULL,
    host        TEXT        NOT NULL,
    docker_port INTEGER     NOT NULL,
    region      TEXT        NOT NULL,
    max_workers SMALLINT    NOT NULL,
    dsn         TEXT        NOT NULL,
    status      TEXT        NOT NULL,
    last_error  TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (exchange, name)
);

CREATE TABLE IF NOT EXISTS workers
(
    id                  UUID PRIMARY KEY,
    node_id             UUID REFERENCES nodes (id) ON DELETE CASCADE,
    host                TEXT        NOT NULL,
    grpc_port           INTEGER     NOT NULL,
    container_id        TEXT        NOT NULL,
    exchange_account_id UUID REFERENCES exchange_accounts (id) ON DELETE CASCADE,
    status              TEXT        NOT NULL,
    last_error          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (exchange_account_id)
);