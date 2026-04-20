package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// FxRate is a dated currency → USD rate (1 currency = usd_rate USD at `at`).
type FxRate struct {
	Currency domain.Currency
	At       time.Time
	USDRate  float64
}

// LatestFxRate is the most recently fetched rate for a currency.
type LatestFxRate struct {
	Currency  domain.Currency
	USDRate   float64
	FetchedAt time.Time
}

// InsertFxRate upserts a historical FX quote.
func (db *DB) InsertFxRate(ctx context.Context, r FxRate) error {
	if !r.Currency.Valid() {
		return fmt.Errorf("invalid currency %q", r.Currency)
	}
	_, err := db.ExecContext(ctx, `
        INSERT INTO fx_rates(currency, at, usd_rate) VALUES (?, ?, ?)
        ON CONFLICT(currency, at) DO UPDATE SET usd_rate = excluded.usd_rate`,
		string(r.Currency), r.At, r.USDRate)
	if err != nil {
		return fmt.Errorf("insert fx: %w", err)
	}
	return nil
}

// GetFxRateAt returns the FX rate for `currency` at or before `at` (nearest
// earlier date). Returns ErrNotFound when no rate exists.
func (db *DB) GetFxRateAt(ctx context.Context, currency domain.Currency, at time.Time) (FxRate, error) {
	var (
		r       FxRate
		curStr  string
		atStamp time.Time
	)
	err := db.QueryRowContext(ctx, `
        SELECT currency, at, usd_rate
          FROM fx_rates
         WHERE currency = ? AND at <= ?
         ORDER BY at DESC LIMIT 1`,
		string(currency), at).Scan(&curStr, &atStamp, &r.USDRate)
	if errors.Is(err, sql.ErrNoRows) {
		return FxRate{}, ErrNotFound
	}
	if err != nil {
		return FxRate{}, fmt.Errorf("scan fx: %w", err)
	}
	r.Currency = domain.Currency(curStr)
	r.At = atStamp
	return r, nil
}

// ListFxRates returns every stored rate for currency in the inclusive
// date range, ordered ascending by `at`. Useful for building time
// series without issuing one query per day.
func (db *DB) ListFxRates(ctx context.Context, currency domain.Currency, from, to time.Time) ([]FxRate, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT currency, at, usd_rate
          FROM fx_rates
         WHERE currency = ? AND at >= ? AND at <= ?
         ORDER BY at ASC`, string(currency), from, to)
	if err != nil {
		return nil, fmt.Errorf("query fx: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []FxRate
	for rows.Next() {
		var (
			r       FxRate
			curStr  string
			atStamp time.Time
		)
		if err := rows.Scan(&curStr, &atStamp, &r.USDRate); err != nil {
			return nil, fmt.Errorf("scan fx: %w", err)
		}
		r.Currency = domain.Currency(curStr)
		r.At = atStamp
		out = append(out, r)
	}
	return out, rows.Err()
}

// SetLatestFxRate upserts the most-current FX quote for a currency.
func (db *DB) SetLatestFxRate(ctx context.Context, r LatestFxRate) error {
	if !r.Currency.Valid() {
		return fmt.Errorf("invalid currency %q", r.Currency)
	}
	_, err := db.ExecContext(ctx, `
        INSERT INTO fx_latest(currency, usd_rate, fetched_at)
        VALUES (?, ?, ?)
        ON CONFLICT(currency) DO UPDATE SET
            usd_rate   = excluded.usd_rate,
            fetched_at = excluded.fetched_at`,
		string(r.Currency), r.USDRate, r.FetchedAt)
	if err != nil {
		return fmt.Errorf("upsert latest fx: %w", err)
	}
	return nil
}

// GetLatestFxRate returns the current FX quote or ErrNotFound.
func (db *DB) GetLatestFxRate(ctx context.Context, currency domain.Currency) (LatestFxRate, error) {
	var (
		r      LatestFxRate
		curStr string
	)
	err := db.QueryRowContext(ctx, `
        SELECT currency, usd_rate, fetched_at
          FROM fx_latest WHERE currency = ?`, string(currency)).
		Scan(&curStr, &r.USDRate, &r.FetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return LatestFxRate{}, ErrNotFound
	}
	if err != nil {
		return LatestFxRate{}, fmt.Errorf("scan latest fx: %w", err)
	}
	r.Currency = domain.Currency(curStr)
	return r, nil
}
