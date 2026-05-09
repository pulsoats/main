CREATE TABLE IF NOT EXISTS services
(
    id           UUID        NOT NULL,
    kind         TEXT        NOT NULL,
    addr         TEXT        NOT NULL,
    name         TEXT        NOT NULL,
    exchange     TEXT        NOT NULL,
    account      TEXT        NOT NULL,
    version      TEXT        NOT NULL DEFAULT '',
    last_seen_at TIMESTAMPTZ,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    PRIMARY KEY (exchange, account)
);
