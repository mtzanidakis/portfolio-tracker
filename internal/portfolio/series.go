package portfolio

import (
	"sort"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// SeriesPoint is a single (day, value, cost) data point in a performance
// time series. Value and Cost are both in the user's base currency;
// Cost is the cumulative basis of currently-open positions (locked at
// each buy's fx_to_base, reduced proportionally on sells — the same
// average-cost method used by Holdings).
type SeriesPoint struct {
	At    time.Time
	Value float64
	Cost  float64
}

// PriceAtFn returns the asset's price (in its native currency) on the
// given day, or ok=false if no snapshot is known for that day.
type PriceAtFn func(symbol string, at time.Time) (price float64, ok bool)

// FxAtFn returns "1 currency = X USD" on the given day, falling back
// to the nearest known earlier rate. Returns ok=false when no rate at
// all is known.
type FxAtFn func(currency domain.Currency, at time.Time) (rate float64, ok bool)

// holdingState is the running per-symbol state maintained while
// replaying transactions. Only cost-in-base is tracked since the series
// is emitted in base currency.
type holdingState struct {
	qty      float64
	costBase float64
}

// SeriesFromTransactions walks day by day from `from` through `to`
// (inclusive), replays transactions, looks up historical prices and FX
// rates, and produces the portfolio's total value and cost basis in
// `base` for each day.
//
// A day with a missing price for a held asset contributes the asset at
// its most recently known price (the caller's PriceAtFn is expected to
// implement that fallback); a day with missing FX is skipped for the
// affected holding. Cost is always known (it was locked at tx time) and
// is emitted regardless of price/FX availability.
func SeriesFromTransactions(
	txs []*domain.Transaction,
	from, to time.Time,
	priceAt PriceAtFn,
	fxAt FxAtFn,
	assetCur AssetCurrencyLookup,
	base domain.Currency,
) []SeriesPoint {
	if to.Before(from) {
		return nil
	}
	sorted := append([]*domain.Transaction(nil), txs...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].OccurredAt.Equal(sorted[j].OccurredAt) {
			return sorted[i].ID < sorted[j].ID
		}
		return sorted[i].OccurredAt.Before(sorted[j].OccurredAt)
	})

	fromD := truncateDay(from)
	toD := truncateDay(to)
	days := int(toD.Sub(fromD).Hours()/24) + 1
	out := make([]SeriesPoint, 0, days)

	state := map[string]*holdingState{}
	txIdx := 0

	for i := range days {
		day := fromD.AddDate(0, 0, i)

		// Apply every tx that happened on or before this day, updating
		// both qty and cost basis so the end-of-day snapshot is correct.
		// Track which symbols transacted today so we can anchor their
		// value to cost — the user paid tx.Price (not Yahoo's EOD
		// close), so valuing them at the snapshot here would report a
		// phantom PnL on day zero.
		txToday := map[string]bool{}
		for txIdx < len(sorted) && !truncateDay(sorted[txIdx].OccurredAt).After(day) {
			tx := sorted[txIdx]
			if truncateDay(tx.OccurredAt).Equal(day) {
				txToday[tx.AssetSymbol] = true
			}
			h := state[tx.AssetSymbol]
			if h == nil {
				h = &holdingState{}
				state[tx.AssetSymbol] = h
			}
			switch {
			case tx.Side.IncreasesQty():
				// buy / deposit / interest
				addNative := tx.Qty*tx.Price + tx.Fee
				h.qty += tx.Qty
				h.costBase += addNative * tx.FxToBase
			case tx.Side == domain.SideSell || tx.Side == domain.SideWithdraw:
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
			txIdx++
		}

		baseUSD, baseOK := fxAt(base, day)
		var totalValue, totalCost float64
		for sym, h := range state {
			if h.qty <= 0 {
				continue
			}
			totalCost += h.costBase

			if txToday[sym] {
				// Anchor value == cost on tx days so deposits register
				// at book value rather than at the day's closing price.
				totalValue += h.costBase
				continue
			}

			price, ok := priceAt(sym, day)
			if !ok {
				continue
			}
			cur, ok := assetCur(sym)
			if !ok {
				continue
			}
			rate := 1.0
			if cur != base {
				srcUSD, ok1 := fxAt(cur, day)
				if !ok1 || !baseOK || baseUSD == 0 {
					continue
				}
				rate = srcUSD / baseUSD
			}
			totalValue += h.qty * price * rate
		}
		out = append(out, SeriesPoint{At: day, Value: totalValue, Cost: totalCost})
	}
	return out
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
