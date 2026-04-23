CREATE TABLE asset_logos (
    symbol       TEXT PRIMARY KEY REFERENCES assets(symbol) ON DELETE CASCADE,
    bytes        BLOB NOT NULL,
    content_type TEXT NOT NULL,
    fetched_at   TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
