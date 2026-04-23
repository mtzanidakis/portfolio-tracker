package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// AssetLogo is the cached image bytes for an asset's logo. Logos are
// fetched lazily from the provider URL stored on the asset row and then
// served from this table forever (the Service wipes cascaded rows when
// the asset itself is deleted).
type AssetLogo struct {
	Symbol      string
	Bytes       []byte
	ContentType string
	FetchedAt   time.Time
}

// GetAssetLogo returns the cached logo for symbol or ErrNotFound.
func (db *DB) GetAssetLogo(ctx context.Context, symbol string) (*AssetLogo, error) {
	var l AssetLogo
	err := db.QueryRowContext(ctx, `
        SELECT symbol, bytes, content_type, fetched_at
          FROM asset_logos WHERE symbol = ?`, symbol).
		Scan(&l.Symbol, &l.Bytes, &l.ContentType, &l.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan asset logo: %w", err)
	}
	return &l, nil
}

// PutAssetLogo inserts or replaces the cached logo for a symbol.
func (db *DB) PutAssetLogo(ctx context.Context, l *AssetLogo) error {
	_, err := db.ExecContext(ctx, `
        INSERT INTO asset_logos(symbol, bytes, content_type, fetched_at)
        VALUES (?, ?, ?, ?)
        ON CONFLICT(symbol) DO UPDATE SET
            bytes        = excluded.bytes,
            content_type = excluded.content_type,
            fetched_at   = excluded.fetched_at`,
		l.Symbol, l.Bytes, l.ContentType, l.FetchedAt)
	if err != nil {
		return fmt.Errorf("put asset logo: %w", err)
	}
	return nil
}
