// Package portfolio provides pure computations over portfolio data:
// deriving current holdings from transactions (average cost method) and
// valuing them in the user's base currency.
//
// The package does no I/O; it accepts already-fetched transactions and
// lookup functions for current prices and FX rates.
package portfolio

import (
	"fmt"
	"sort"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// epsilon guards against float drift when zeroing out fully-sold positions.
const epsilon = 1e-9

// Holding is a derived position in a single asset, aggregated across all
// accounts for one user. Cost basis is tracked separately in the asset's
// native currency and in the user's base currency (using the fx_to_base
// locked at each buy transaction).
type Holding struct {
	Symbol        string
	Qty           float64
	CostNative    float64 // total cost basis in native currency
	CostBase      float64 // total cost basis in user's base currency
	AvgCostNative float64 // per-unit, native
	AvgCostBase   float64 // per-unit, base
}

// Holdings computes the current position in each asset from the given
// transactions using the average cost method. Transactions are processed in
// chronological order of OccurredAt (ties broken by ID). Returns an error
// when a sell would produce a negative quantity (selling more than owned).
//
// Fully closed positions (quantity goes to zero) are omitted from the result.
func Holdings(txs []*domain.Transaction) ([]Holding, error) {
	sorted := make([]*domain.Transaction, len(txs))
	copy(sorted, txs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].OccurredAt.Equal(sorted[j].OccurredAt) {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	byMap := make(map[string]*Holding)
	for _, tx := range sorted {
		h, ok := byMap[tx.AssetSymbol]
		if !ok {
			h = &Holding{Symbol: tx.AssetSymbol}
			byMap[tx.AssetSymbol] = h
		}
		switch {
		case tx.Side.IncreasesQty():
			// buy / deposit / interest — all add qty and extend cost
			// basis by the per-unit price plus whatever fee was paid.
			addNative := tx.Qty*tx.Price + tx.Fee
			h.Qty += tx.Qty
			h.CostNative += addNative
			h.CostBase += addNative * tx.FxToBase
		case tx.Side == domain.SideSell || tx.Side == domain.SideWithdraw:
			if tx.Qty > h.Qty+epsilon {
				return nil, fmt.Errorf(
					"%s of %g %s exceeds current holding of %g",
					tx.Side, tx.Qty, tx.AssetSymbol, h.Qty,
				)
			}
			// Reduce cost basis at current average cost.
			avgNative := 0.0
			avgBase := 0.0
			if h.Qty > 0 {
				avgNative = h.CostNative / h.Qty
				avgBase = h.CostBase / h.Qty
			}
			h.Qty -= tx.Qty
			h.CostNative -= avgNative * tx.Qty
			h.CostBase -= avgBase * tx.Qty
			if h.Qty < epsilon {
				// Clamp to exact zero to avoid float residue.
				h.Qty = 0
				h.CostNative = 0
				h.CostBase = 0
			}
		default:
			return nil, fmt.Errorf("unknown transaction side %q", tx.Side)
		}
		if h.Qty > 0 {
			h.AvgCostNative = h.CostNative / h.Qty
			h.AvgCostBase = h.CostBase / h.Qty
		} else {
			h.AvgCostNative = 0
			h.AvgCostBase = 0
		}
	}

	// Sort output by symbol for determinism.
	keys := make([]string, 0, len(byMap))
	for k := range byMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	out := make([]Holding, 0, len(keys))
	for _, k := range keys {
		h := byMap[k]
		if h.Qty > 0 {
			out = append(out, *h)
		}
	}
	return out, nil
}
