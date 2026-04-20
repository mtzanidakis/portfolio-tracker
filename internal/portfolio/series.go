package portfolio

import (
	"sort"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// SeriesPoint is a single (day, value) data point in a performance
// time series. Value is in the user's base currency.
type SeriesPoint struct {
	At    time.Time
	Value float64
}

// PriceAtFn returns the asset's price (in its native currency) on the
// given day, or ok=false if no snapshot is known for that day.
type PriceAtFn func(symbol string, at time.Time) (price float64, ok bool)

// FxAtFn returns "1 currency = X USD" on the given day, falling back
// to the nearest known earlier rate. Returns ok=false when no rate at
// all is known.
type FxAtFn func(currency domain.Currency, at time.Time) (rate float64, ok bool)

// SeriesFromTransactions walks day by day from `from` through `to`
// (inclusive), replays transactions, looks up historical prices and FX
// rates, and produces the portfolio's total value in `base` for each
// day.
//
// A day with a missing price for a held asset contributes the asset at
// its most recently known price (the caller's PriceAtFn is expected to
// implement that fallback); a day with missing FX is skipped for the
// affected holding.
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

	qty := map[string]float64{}
	txIdx := 0

	for i := range days {
		day := fromD.AddDate(0, 0, i)

		// Apply every tx that happened on or before this day.
		for txIdx < len(sorted) && !truncateDay(sorted[txIdx].OccurredAt).After(day) {
			tx := sorted[txIdx]
			switch tx.Side {
			case domain.SideBuy:
				qty[tx.AssetSymbol] += tx.Qty
			case domain.SideSell:
				qty[tx.AssetSymbol] -= tx.Qty
			}
			txIdx++
		}

		baseUSD, baseOK := fxAt(base, day)
		var total float64
		for sym, q := range qty {
			if q <= 0 {
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
			total += q * price * rate
		}
		out = append(out, SeriesPoint{At: day, Value: total})
	}
	return out
}

func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}
