package api

import (
	"context"
	"errors"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/portfolio"
)

// buildLookups preloads assets + latest prices + latest FX rates into
// in-memory maps and wraps them in the portfolio package's lookup
// function types. Returns error only on unrecoverable DB failures.
func buildLookups(ctx context.Context, d *db.DB) (
	portfolio.PriceLookup,
	portfolio.FxLookup,
	portfolio.AssetCurrencyLookup,
	error,
) {
	assets, err := d.ListAssets(ctx)
	if err != nil {
		return nil, nil, nil, err
	}

	type assetInfo struct {
		Currency domain.Currency
		IsCash   bool
	}
	assetMap := make(map[string]assetInfo, len(assets))
	for _, a := range assets {
		assetMap[a.Symbol] = assetInfo{
			Currency: a.Currency,
			IsCash:   a.Type == domain.AssetCash,
		}
	}

	curLookup := func(sym string) (domain.Currency, bool) {
		info, ok := assetMap[sym]
		if !ok {
			return "", false
		}
		return info.Currency, true
	}

	priceLookup := func(sym string) (float64, bool) {
		if info, ok := assetMap[sym]; ok && info.IsCash {
			return 1.0, true
		}
		p, err := d.GetLatestPrice(ctx, sym)
		if err != nil {
			return 0, false
		}
		return p.Price, true
	}

	// Preload latest FX rates for all supported currencies.
	fxMap := map[domain.Currency]float64{domain.USD: 1.0}
	for _, c := range domain.AllCurrencies {
		if c == domain.USD {
			continue
		}
		r, err := d.GetLatestFxRate(ctx, c)
		if err != nil {
			if errors.Is(err, db.ErrNotFound) {
				continue
			}
			return nil, nil, nil, err
		}
		fxMap[c] = r.USDRate
	}
	fxLookup := func(c domain.Currency) (float64, bool) {
		r, ok := fxMap[c]
		return r, ok
	}

	return priceLookup, fxLookup, curLookup, nil
}
