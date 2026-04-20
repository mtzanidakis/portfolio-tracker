CREATE TABLE price_snapshots (
    asset_symbol TEXT NOT NULL REFERENCES assets(symbol),
    at           TIMESTAMP NOT NULL,
    price        REAL NOT NULL,
    PRIMARY KEY (asset_symbol, at)
);

CREATE INDEX idx_snap_date ON price_snapshots(at);

CREATE TABLE prices_latest (
    asset_symbol TEXT PRIMARY KEY REFERENCES assets(symbol),
    price        REAL NOT NULL,
    fetched_at   TIMESTAMP NOT NULL
);

-- fx_rates stores daily FX quotes expressed in USD.
-- For base_currency = USD: usd_rate = 1.0 (identity).
-- For any other currency C: 1 C = usd_rate USD at the given `at` date.
CREATE TABLE fx_rates (
    currency TEXT NOT NULL
        CHECK (currency IN ('USD','EUR','GBP','JPY','CHF','CAD','AUD')),
    at       TIMESTAMP NOT NULL,
    usd_rate REAL NOT NULL CHECK (usd_rate > 0),
    PRIMARY KEY (currency, at)
);

CREATE INDEX idx_fx_date ON fx_rates(at);

CREATE TABLE fx_latest (
    currency   TEXT PRIMARY KEY
        CHECK (currency IN ('USD','EUR','GBP','JPY','CHF','CAD','AUD')),
    usd_rate   REAL NOT NULL CHECK (usd_rate > 0),
    fetched_at TIMESTAMP NOT NULL
);
