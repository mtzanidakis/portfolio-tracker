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
//
// PriceStale is true when either the current price or the FX rate was
// unavailable. In that case ValueBase falls back to CostBase (so the
// holding still contributes to totals and appears in allocations) and
// the PnL fields are zero.
type HoldingValue struct {
	Holding
	Currency     domain.Currency // native currency of the asset
	CurrentPrice float64         // in native currency; 0 when missing
	ValueNative  float64
	ValueBase    float64
	PnLNative    float64
	PnLBase      float64
	PnLPctNative float64
	PnLPctBase   float64
	PriceStale   bool
}

// ValueHoldings prices each holding in the user's base currency. When a
// price or FX rate is missing, the holding is still returned with
// ValueBase == CostBase (zero PnL) and PriceStale=true so the UI can
// surface the condition. Holdings whose asset currency is entirely
// unknown use the user's base currency as a neutral fallback.
func ValueHoldings(
	holdings []Holding,
	prices PriceLookup,
	fx FxLookup,
	currencies AssetCurrencyLookup,
	base domain.Currency,
) []HoldingValue {
	out := make([]HoldingValue, 0, len(holdings))
	for _, h := range holdings {
		cur, hasCur := currencies(h.Symbol)
		if !hasCur {
			cur = base
		}

		price, hasPrice := prices(h.Symbol)

		fxNativeToBase := 1.0
		hasFx := true
		if cur != base {
			nativeUSD, ok1 := fx(cur)
			baseUSD, ok2 := fx(base)
			if !ok1 || !ok2 || baseUSD == 0 {
				hasFx = false
			} else {
				fxNativeToBase = nativeUSD / baseUSD
			}
		}

		stale := !hasPrice || !hasFx
		var valueNative, valueBase, pnlNative, pnlBase, pctNative, pctBase float64

		switch {
		case !stale:
			valueNative = h.Qty * price
			valueBase = valueNative * fxNativeToBase
			pnlNative = valueNative - h.CostNative
			pnlBase = valueBase - h.CostBase
			if h.CostNative > 0 {
				pctNative = pnlNative / h.CostNative * 100
			}
			if h.CostBase > 0 {
				pctBase = pnlBase / h.CostBase * 100
			}
		default:
			// Missing price or FX: fall back to cost basis. PnL = 0.
			valueNative = h.CostNative
			valueBase = h.CostBase
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
			PriceStale:   stale,
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
