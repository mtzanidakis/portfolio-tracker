package portfolio

import (
	"sort"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// PriceLookup returns the current price of a symbol in its native currency.
// Returns ok=false when the symbol has no known price.
type PriceLookup func(symbol string) (price float64, ok bool)

// FxLookup returns the "USD value of 1 unit of c" — i.e., 1 c = rate USD.
// For USD itself this is 1.0. Returns ok=false when no rate is known.
type FxLookup func(c domain.Currency) (rate float64, ok bool)

// AssetCurrencyLookup returns the native currency of the given asset symbol.
type AssetCurrencyLookup func(symbol string) (cur domain.Currency, ok bool)

// HoldingValue augments a Holding with current valuation and PnL in both
// the asset's native currency and the user's base currency.
type HoldingValue struct {
	Holding
	Currency     domain.Currency // native currency of the asset
	CurrentPrice float64         // in native currency
	ValueNative  float64
	ValueBase    float64
	PnLNative    float64
	PnLBase      float64
	PnLPctNative float64 // percentage, e.g. 12.5 == 12.5%
	PnLPctBase   float64
}

// ValueHoldings prices each holding in the user's base currency using the
// supplied lookup functions. Holdings for which price, currency, or FX is
// missing are silently omitted — the caller decides how to surface that.
//
// The FX conversion uses currency→USD→base (two lookups): this keeps the
// underlying rates table single-rooted (USD) while letting base_currency be
// any supported currency.
func ValueHoldings(
	holdings []Holding,
	prices PriceLookup,
	fx FxLookup,
	currencies AssetCurrencyLookup,
	base domain.Currency,
) []HoldingValue {
	out := make([]HoldingValue, 0, len(holdings))
	for _, h := range holdings {
		price, ok := prices(h.Symbol)
		if !ok {
			continue
		}
		cur, ok := currencies(h.Symbol)
		if !ok {
			continue
		}

		fxNativeToBase := 1.0
		if cur != base {
			nativeUSD, ok1 := fx(cur)
			baseUSD, ok2 := fx(base)
			if !ok1 || !ok2 || baseUSD == 0 {
				continue
			}
			fxNativeToBase = nativeUSD / baseUSD
		}

		valueNative := h.Qty * price
		valueBase := valueNative * fxNativeToBase
		pnlNative := valueNative - h.CostNative
		pnlBase := valueBase - h.CostBase

		var pctNative, pctBase float64
		if h.CostNative > 0 {
			pctNative = pnlNative / h.CostNative * 100
		}
		if h.CostBase > 0 {
			pctBase = pnlBase / h.CostBase * 100
		}

		out = append(out, HoldingValue{
			Holding:      h,
			Currency:     cur,
			CurrentPrice: price,
			ValueNative:  valueNative,
			ValueBase:    valueBase,
			PnLNative:    pnlNative,
			PnLBase:      pnlBase,
			PnLPctNative: pctNative,
			PnLPctBase:   pctBase,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Symbol < out[j].Symbol })
	return out
}

// TotalValueBase returns the sum of ValueBase across all priced holdings.
func TotalValueBase(values []HoldingValue) float64 {
	var sum float64
	for _, v := range values {
		sum += v.ValueBase
	}
	return sum
}

// TotalCostBase returns the sum of CostBase across all priced holdings.
func TotalCostBase(values []HoldingValue) float64 {
	var sum float64
	for _, v := range values {
		sum += v.CostBase
	}
	return sum
}
