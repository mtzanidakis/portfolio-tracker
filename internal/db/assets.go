package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// UpsertAsset inserts or replaces an asset row (keyed by symbol).
func (db *DB) UpsertAsset(ctx context.Context, a *domain.Asset) error {
	if !a.Type.Valid() {
		return fmt.Errorf("invalid asset type %q", a.Type)
	}
	if !a.Currency.Valid() {
		return fmt.Errorf("invalid currency %q", a.Currency)
	}
	_, err := db.ExecContext(ctx, `
        INSERT INTO assets(symbol, name, type, currency, provider, provider_id, logo_url)
        VALUES (?, ?, ?, ?, ?, ?, ?)
        ON CONFLICT(symbol) DO UPDATE SET
            name        = excluded.name,
            type        = excluded.type,
            currency    = excluded.currency,
            provider    = excluded.provider,
            provider_id = excluded.provider_id,
            logo_url    = excluded.logo_url`,
		a.Symbol, a.Name, string(a.Type), string(a.Currency),
		a.Provider, a.ProviderID, a.LogoURL,
	)
	if err != nil {
		return fmt.Errorf("upsert asset: %w", err)
	}
	return nil
}

// GetAsset returns the asset for the given symbol or ErrNotFound.
func (db *DB) GetAsset(ctx context.Context, symbol string) (*domain.Asset, error) {
	var (
		a        domain.Asset
		typ, cur string
	)
	err := db.QueryRowContext(ctx, `
        SELECT symbol, name, type, currency, provider, provider_id, logo_url
          FROM assets WHERE symbol = ?`, symbol).
		Scan(&a.Symbol, &a.Name, &typ, &cur, &a.Provider, &a.ProviderID, &a.LogoURL)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan asset: %w", err)
	}
	a.Type = domain.AssetType(typ)
	a.Currency = domain.Currency(cur)
	return &a, nil
}

// ListAssets returns every asset, ordered by symbol.
func (db *DB) ListAssets(ctx context.Context) ([]*domain.Asset, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT symbol, name, type, currency, provider, provider_id, logo_url
          FROM assets ORDER BY symbol`)
	if err != nil {
		return nil, fmt.Errorf("query assets: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*domain.Asset
	for rows.Next() {
		var (
			a        domain.Asset
			typ, cur string
		)
		if err := rows.Scan(&a.Symbol, &a.Name, &typ, &cur,
			&a.Provider, &a.ProviderID, &a.LogoURL); err != nil {
			return nil, fmt.Errorf("scan asset: %w", err)
		}
		a.Type = domain.AssetType(typ)
		a.Currency = domain.Currency(cur)
		out = append(out, &a)
	}
	return out, rows.Err()
}

// DeleteAsset removes an asset. Fails if transactions reference it.
func (db *DB) DeleteAsset(ctx context.Context, symbol string) error {
	res, err := db.ExecContext(ctx, `DELETE FROM assets WHERE symbol = ?`, symbol)
	if err != nil {
		return fmt.Errorf("delete asset: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
