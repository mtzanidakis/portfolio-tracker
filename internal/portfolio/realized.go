package portfolio

import (
	"sort"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// RealizedPnL is the lifetime realized profit or loss across all
// partially- or fully-closed positions, expressed in the user's base
// currency. It mirrors the average-cost method used by Holdings:
// every sell realises (proceeds − proportional cost), where proceeds
// are (qty×price − fee) converted at tx.FxToBase and proportional
// cost is the running (cost_base / qty) × qty_sold.
//
// Deposit / withdraw / interest on cash assets don't produce realized
// PnL — they are cash movements, not a gain on an investment. The
// user elected earlier to treat interest as a deposit for accounting
// purposes, so we stay consistent here.
func RealizedPnL(txs []*domain.Transaction) (float64, error) {
	sorted := make([]*domain.Transaction, len(txs))
	copy(sorted, txs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].OccurredAt.Equal(sorted[j].OccurredAt) {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	type state struct{ qty, costBase float64 }
	by := map[string]*state{}
	var realized float64

	for _, tx := range sorted {
		h := by[tx.AssetSymbol]
		if h == nil {
			h = &state{}
			by[tx.AssetSymbol] = h
		}
		switch {
		case tx.Side.IncreasesQty():
			addNative := tx.Qty*tx.Price + tx.Fee
			h.qty += tx.Qty
			h.costBase += addNative * tx.FxToBase
		case tx.Side == domain.SideSell:
			avg := 0.0
			if h.qty > 0 {
				avg = h.costBase / h.qty
			}
			proceeds := (tx.Qty*tx.Price - tx.Fee) * tx.FxToBase
			costRemoved := avg * tx.Qty
			realized += proceeds - costRemoved
			h.qty -= tx.Qty
			h.costBase -= costRemoved
			if h.qty < epsilon {
				h.qty = 0
				h.costBase = 0
			}
		case tx.Side == domain.SideWithdraw:
			avg := 0.0
			if h.qty > 0 {
				avg = h.costBase / h.qty
			}
			h.qty -= tx.Qty
			h.costBase -= avg * tx.Qty
			if h.qty < epsilon {
				h.qty = 0
				h.costBase = 0
			}
		}
	}
	return realized, nil
}
