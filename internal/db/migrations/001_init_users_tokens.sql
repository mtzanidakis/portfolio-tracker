CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT    NOT NULL UNIQUE,
    name          TEXT    NOT NULL,
    password_hash TEXT    NOT NULL DEFAULT '',  -- argon2id; '' = no browser login
    base_currency TEXT    NOT NULL
        CHECK (base_currency IN ('USD','EUR','GBP','JPY','CHF','CAD','AUD')),
    created_at    TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name         TEXT    NOT NULL,
    hash         TEXT    NOT NULL UNIQUE,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP,
    revoked_at   TIMESTAMP
);

CREATE INDEX idx_tokens_user ON tokens(user_id);
CREATE INDEX idx_tokens_hash ON tokens(hash);

-- Browser sessions. session_id is a 32-byte random value, b64url-encoded.
CREATE TABLE sessions (
    id           TEXT    PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at   TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP
);

CREATE INDEX idx_sessions_user    ON sessions(user_id);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);
