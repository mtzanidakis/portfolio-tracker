// Package prices provides price and FX-rate providers plus a refresh
// service that writes the latest quotes into the database.
//
// Providers are thin HTTP adapters around Yahoo Finance (stocks/ETFs),
// CoinGecko (crypto), and Frankfurter (FX). Providers return data; the
// Service orchestrates and persists.
package prices

import (
	"context"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// PriceQuote is a single instrument quote as returned by a PriceProvider.
// The Symbol field echoes the *external* identifier the caller supplied
// (e.g., CoinGecko coin ID, or Yahoo ticker) — the Service maps it back
// to the tracker's asset symbol.
type PriceQuote struct {
	Symbol    string
	Price     float64
	Currency  domain.Currency
	FetchedAt time.Time
}

// PriceProvider fetches the current price of one or more instruments.
type PriceProvider interface {
	// Name identifies the provider (matches assets.provider in DB).
	Name() string
	// Fetch returns quotes for the given external IDs.
	Fetch(ctx context.Context, externalIDs []string) ([]PriceQuote, error)
}

// FxProvider fetches FX rates expressed as "1 currency = rate USD".
type FxProvider interface {
	Name() string
	// Fetch returns a map keyed by currency with rate in USD.
	// USD itself may be included with rate 1.0.
	Fetch(ctx context.Context, currencies []domain.Currency) (map[domain.Currency]float64, error)
}

// FxHistoryProvider fetches an historical FX rate for a specific pair.
// at may be zero to indicate "latest".
type FxHistoryProvider interface {
	FetchRate(ctx context.Context, from, to domain.Currency, at time.Time) (float64, error)
}

// HistoricalSnapshot is a dated price point (typically a daily close).
// Price is denominated in Currency — usually the asset's native
// currency for Yahoo, USD for CoinGecko.
type HistoricalSnapshot struct {
	Symbol   string
	At       time.Time
	Price    float64
	Currency domain.Currency
}

// HistoryProvider can fetch daily price history for a single external
// identifier (ticker / coin id). Implemented by Yahoo + CoinGecko.
//
// `from` is the earliest date the caller wants covered. Providers may
// return more than that (rounded up to their supported range step —
// Yahoo only offers 1y/2y/5y/10y/max, for instance) and must return
// what exists when the instrument hasn't traded that far back. Zero
// `from` means "provider default", typically ~1 year.
type HistoryProvider interface {
	FetchHistory(ctx context.Context, externalID string, from time.Time) ([]HistoricalSnapshot, error)
}

// HistoricalFxRate is a dated FX quote expressed as "1 Currency = X USD".
type HistoricalFxRate struct {
	Currency domain.Currency
	At       time.Time
	USDRate  float64
}

// FxRangeProvider fetches historical FX rates for a range of dates.
type FxRangeProvider interface {
	FetchRange(ctx context.Context, currencies []domain.Currency, from, to time.Time) ([]HistoricalFxRate, error)
}

// SymbolInfo is provider-resolved metadata about a ticker / coin id.
// Fields are best-effort: providers may leave some blank (e.g., Yahoo
// doesn't classify cash; CoinGecko always returns USD + crypto). LogoURL
// points at an external logo (Clearbit for stocks, CoinGecko CDN for
// crypto); empty when nothing was resolvable.
type SymbolInfo struct {
	Symbol     string
	Name       string
	Currency   domain.Currency
	AssetType  domain.AssetType
	ProviderID string
	LogoURL    string
}

// SymbolLookup resolves a user-typed symbol to provider metadata so the
// "add asset" form can auto-fill name / currency / type.
type SymbolLookup interface {
	LookupSymbol(ctx context.Context, symbol string) (*SymbolInfo, error)
}
