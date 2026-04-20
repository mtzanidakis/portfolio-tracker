package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PriceSnapshot is a single (symbol, date, price) quote.
type PriceSnapshot struct {
	Symbol string
	At     time.Time
	Price  float64
}

// LatestPrice is the most recently fetched price for an asset.
type LatestPrice struct {
	Symbol    string
	Price     float64
	FetchedAt time.Time
}

// InsertPriceSnapshot upserts a snapshot keyed by (symbol, at).
func (db *DB) InsertPriceSnapshot(ctx context.Context, s PriceSnapshot) error {
	_, err := db.ExecContext(ctx, `
        INSERT INTO price_snapshots(asset_symbol, at, price) VALUES (?, ?, ?)
        ON CONFLICT(asset_symbol, at) DO UPDATE SET price = excluded.price`,
		s.Symbol, s.At, s.Price)
	if err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	return nil
}

// ListPriceSnapshots returns snapshots for a symbol in the given inclusive
// date range, ordered ascending by `at`.
func (db *DB) ListPriceSnapshots(ctx context.Context, symbol string, from, to time.Time) ([]PriceSnapshot, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT asset_symbol, at, price
          FROM price_snapshots
         WHERE asset_symbol = ? AND at >= ? AND at <= ?
         ORDER BY at ASC`, symbol, from, to)
	if err != nil {
		return nil, fmt.Errorf("query snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []PriceSnapshot
	for rows.Next() {
		var s PriceSnapshot
		if err := rows.Scan(&s.Symbol, &s.At, &s.Price); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// SetLatestPrice upserts the "current" price for an asset.
func (db *DB) SetLatestPrice(ctx context.Context, p LatestPrice) error {
	_, err := db.ExecContext(ctx, `
        INSERT INTO prices_latest(asset_symbol, price, fetched_at)
        VALUES (?, ?, ?)
        ON CONFLICT(asset_symbol) DO UPDATE SET
            price      = excluded.price,
            fetched_at = excluded.fetched_at`,
		p.Symbol, p.Price, p.FetchedAt)
	if err != nil {
		return fmt.Errorf("upsert latest price: %w", err)
	}
	return nil
}

// GetLatestPrice returns the most recent stored price for symbol.
func (db *DB) GetLatestPrice(ctx context.Context, symbol string) (LatestPrice, error) {
	var p LatestPrice
	err := db.QueryRowContext(ctx, `
        SELECT asset_symbol, price, fetched_at
          FROM prices_latest WHERE asset_symbol = ?`, symbol).
		Scan(&p.Symbol, &p.Price, &p.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LatestPrice{}, ErrNotFound
	}
	if err != nil {
		return LatestPrice{}, fmt.Errorf("scan latest price: %w", err)
	}
	return p, nil
}
