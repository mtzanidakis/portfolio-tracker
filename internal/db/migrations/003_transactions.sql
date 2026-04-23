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

-- Full-text index over the three textual fields a user might search
-- on from the Activities page. unicode61 is tokenizer-aware for every
-- script; remove_diacritics=2 folds "Άπλ" ≡ "απλ" ≡ "ΑΠΛ" so the
-- Greek search bar just works. The name column is denormalised from
-- the assets table and kept in sync by triggers below.
CREATE VIRTUAL TABLE tx_fts USING fts5(
    symbol, name, note,
    tokenize = 'unicode61 remove_diacritics 2'
);

-- Keep tx_fts in lock-step with transactions + assets.
CREATE TRIGGER tx_fts_ai AFTER INSERT ON transactions BEGIN
    INSERT INTO tx_fts(rowid, symbol, name, note)
    VALUES (
        NEW.id,
        NEW.asset_symbol,
        COALESCE((SELECT name FROM assets WHERE symbol = NEW.asset_symbol), ''),
        NEW.note
    );
END;

CREATE TRIGGER tx_fts_au AFTER UPDATE ON transactions BEGIN
    UPDATE tx_fts SET
        symbol = NEW.asset_symbol,
        name   = COALESCE((SELECT name FROM assets WHERE symbol = NEW.asset_symbol), ''),
        note   = NEW.note
    WHERE rowid = NEW.id;
END;

CREATE TRIGGER tx_fts_ad AFTER DELETE ON transactions BEGIN
    DELETE FROM tx_fts WHERE rowid = OLD.id;
END;

CREATE TRIGGER assets_name_au AFTER UPDATE OF name ON assets BEGIN
    UPDATE tx_fts SET name = NEW.name WHERE symbol = NEW.symbol;
END;
