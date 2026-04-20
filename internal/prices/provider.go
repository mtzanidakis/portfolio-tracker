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
type HistoryProvider interface {
	FetchHistory(ctx context.Context, externalID string) ([]HistoricalSnapshot, error)
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
