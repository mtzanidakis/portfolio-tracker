CREATE TABLE accounts (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name       TEXT    NOT NULL,
    type       TEXT    NOT NULL,
    short      TEXT    NOT NULL,
    color      TEXT    NOT NULL,
    currency   TEXT    NOT NULL
        CHECK (currency IN ('USD','EUR','GBP','JPY','CHF','CAD','AUD')),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_accounts_user ON accounts(user_id);

CREATE TABLE assets (
    symbol      TEXT PRIMARY KEY,
    name        TEXT NOT NULL,
    type        TEXT NOT NULL
        CHECK (type IN ('stock','etf','crypto','cash')),
    currency    TEXT NOT NULL
        CHECK (currency IN ('USD','EUR','GBP','JPY','CHF','CAD','AUD')),
    provider    TEXT NOT NULL DEFAULT '',
    provider_id TEXT NOT NULL DEFAULT '',
    logo_url    TEXT NOT NULL DEFAULT ''
);
