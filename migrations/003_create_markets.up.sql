CREATE TABLE IF NOT EXISTS markets
(
    id       BIGSERIAL PRIMARY KEY,
    exchange TEXT NOT NULL,
    category TEXT NOT NULL,
    symbol   TEXT NOT NULL,

    UNIQUE (exchange, category, symbol)
);

CREATE INDEX IF NOT EXISTS idx_markets_lookup
    ON markets (exchange, category, symbol);
