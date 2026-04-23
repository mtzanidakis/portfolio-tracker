CREATE TABLE transactions (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id       INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id    INTEGER NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    asset_symbol  TEXT    NOT NULL REFERENCES assets(symbol) ON DELETE CASCADE,
    side          TEXT    NOT NULL CHECK (side IN ('buy','sell','deposit','withdraw','interest')),
    qty           REAL    NOT NULL CHECK (qty > 0),
    price         REAL    NOT NULL CHECK (price >= 0),
    fee           REAL    NOT NULL DEFAULT 0 CHECK (fee >= 0),
    fx_to_base    REAL    NOT NULL CHECK (fx_to_base > 0),
    occurred_at   TIMESTAMP NOT NULL,
    note          TEXT    NOT NULL DEFAULT '',
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_tx_user_date ON transactions(user_id, occurred_at);
CREATE INDEX idx_tx_account   ON transactions(account_id);
CREATE INDEX idx_tx_asset     ON transactions(asset_symbol);
