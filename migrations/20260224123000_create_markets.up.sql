CREATE TABLE IF NOT EXISTS exchanges
(
    id   BIGSERIAL PRIMARY KEY,
    code TEXT NOT NULL UNIQUE
);

CREATE TABLE IF NOT EXISTS markets
(
    id          BIGSERIAL PRIMARY KEY,
    exchange_id BIGINT  NOT NULL REFERENCES exchanges (id),
    category    TEXT    NOT NULL,
    symbol      TEXT    NOT NULL,
    launch_time TIMESTAMPTZ,
    is_active   BOOLEAN NOT NULL DEFAULT true,

    UNIQUE (exchange_id, category, symbol)
);

CREATE INDEX idx_markets_active
    ON markets (exchange_id, category, symbol)
    WHERE is_active = true;
