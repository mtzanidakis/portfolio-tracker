package api

import (
	"context"
	"net/http"
	"sort"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/portfolio"
)

func holdingsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		txs, err := d.ListTransactions(r.Context(), db.TxFilter{UserID: u.ID})
		if err != nil {
			writeDBError(w, err)
			return
		}
		holdings, err := portfolio.Holdings(txs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		prices, fx, currencies, err := buildLookups(r.Context(), d)
		if err != nil {
			writeDBError(w, err)
			return
		}
		values := portfolio.ValueHoldings(holdings, prices, fx, currencies, u.BaseCurrency)
		writeJSON(w, http.StatusOK, values)
	}
}

type allocationEntry struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Sub      string  `json:"sub,omitempty"`
	Value    float64 `json:"value"`
	Fraction float64 `json:"fraction"` // 0..1
}

func allocationsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		group := r.URL.Query().Get("group")
		if group == "" {
			group = "asset"
		}

		txs, err := d.ListTransactions(r.Context(), db.TxFilter{UserID: u.ID})
		if err != nil {
			writeDBError(w, err)
			return
		}
		holdings, err := portfolio.Holdings(txs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		prices, fx, currencies, err := buildLookups(r.Context(), d)
		if err != nil {
			writeDBError(w, err)
			return
		}
		values := portfolio.ValueHoldings(holdings, prices, fx, currencies, u.BaseCurrency)
		total := portfolio.TotalValueBase(values)

		// For "by-asset" the group key is the symbol; for "type" it's the
		// asset's type (from DB). "account" groupings require replaying
		// transactions per account — deferred (returns empty for now).
		buckets := make(map[string]*allocationEntry)
		switch group {
		case "asset":
			for _, v := range values {
				buckets[v.Symbol] = &allocationEntry{
					Key:   v.Symbol,
					Label: v.Symbol,
					Value: v.ValueBase,
				}
			}
		case "type":
			assets, _ := d.ListAssets(r.Context())
			typeOf := make(map[string]string, len(assets))
			for _, a := range assets {
				typeOf[a.Symbol] = string(a.Type)
			}
			for _, v := range values {
				t := typeOf[v.Symbol]
				if t == "" {
					t = "other"
				}
				b, ok := buckets[t]
				if !ok {
					b = &allocationEntry{Key: t, Label: t}
					buckets[t] = b
				}
				b.Value += v.ValueBase
			}
		case "account":
			// Per-account allocation computed by replaying transactions
			// scoped to each account. Intentionally left empty for MVP;
			// the UI falls back to by-asset.
		default:
			writeError(w, http.StatusBadRequest, "invalid group")
			return
		}

		out := make([]allocationEntry, 0, len(buckets))
		for _, b := range buckets {
			if total > 0 {
				b.Fraction = b.Value / total
			}
			out = append(out, *b)
		}
		writeJSON(w, http.StatusOK, out)
	}
}

type performancePoint struct {
	At    string  `json:"at"`
	Value float64 `json:"value"`
}

type performanceResponse struct {
	Total     float64            `json:"total"`
	Cost      float64            `json:"cost"`
	PnL       float64            `json:"pnl"`
	PnLPct    float64            `json:"pnl_pct"`
	Currency  string             `json:"currency"`
	Timeframe string             `json:"timeframe"`
	Series    []performancePoint `json:"series"`
	AnyStale  bool               `json:"any_stale"`
}

// timeframeDays maps the client's timeframe string to a day count.
// "ALL" returns 0 — callers should use the earliest tx date instead.
func timeframeDays(tf string) int {
	switch tf {
	case "1D":
		return 1
	case "1W":
		return 7
	case "1M":
		return 30
	case "3M":
		return 90
	case "6M":
		return 180
	case "1Y":
		return 365
	case "ALL":
		return 0
	default:
		return 180
	}
}

func performanceHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		tf := r.URL.Query().Get("tf")
		if tf == "" {
			tf = "6M"
		}

		txs, err := d.ListTransactions(r.Context(), db.TxFilter{UserID: u.ID})
		if err != nil {
			writeDBError(w, err)
			return
		}
		holdings, err := portfolio.Holdings(txs)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		priceFn, fxFn, curFn, err := buildLookups(r.Context(), d)
		if err != nil {
			writeDBError(w, err)
			return
		}
		values := portfolio.ValueHoldings(holdings, priceFn, fxFn, curFn, u.BaseCurrency)
		total := portfolio.TotalValueBase(values)
		cost := portfolio.TotalCostBase(values)
		anyStale := false
		for _, v := range values {
			if v.PriceStale {
				anyStale = true
				break
			}
		}

		var pct float64
		if cost > 0 {
			pct = (total - cost) / cost * 100
		}

		series := buildSeries(r.Context(), d, txs, tf, curFn, u.BaseCurrency)

		resp := performanceResponse{
			Total:     total,
			Cost:      cost,
			PnL:       total - cost,
			AnyStale:  anyStale,
			PnLPct:    pct,
			Currency:  string(u.BaseCurrency),
			Timeframe: tf,
			Series:    series,
		}
		writeJSON(w, http.StatusOK, resp)
	}
}

// buildSeries produces the daily performance series for the requested
// timeframe. Prices and FX are preloaded for the date range into
// in-memory maps; lookups fall back to the nearest earlier entry.
func buildSeries(
	ctx context.Context,
	d *db.DB,
	txs []*domain.Transaction,
	tf string,
	curFn portfolio.AssetCurrencyLookup,
	base domain.Currency,
) []performancePoint {
	if len(txs) == 0 {
		return []performancePoint{}
	}

	to := time.Now().UTC()
	days := timeframeDays(tf)
	var from time.Time
	if days == 0 {
		earliest := txs[0].OccurredAt
		for _, tx := range txs {
			if tx.OccurredAt.Before(earliest) {
				earliest = tx.OccurredAt
			}
		}
		from = earliest
	} else {
		from = to.AddDate(0, 0, -days)
	}
	// Never start before the first tx — the chart is flat at zero before that.
	earliest := txs[0].OccurredAt
	for _, tx := range txs {
		if tx.OccurredAt.Before(earliest) {
			earliest = tx.OccurredAt
		}
	}
	if from.Before(earliest) {
		from = earliest
	}

	// Preload per-symbol snapshots for the whole range, plus one
	// earlier row as a fallback so the first in-range day always
	// resolves to a price.
	preload := from.AddDate(0, 0, -30)
	symbols := map[string]struct{}{}
	for _, tx := range txs {
		symbols[tx.AssetSymbol] = struct{}{}
	}
	snapByAsset := make(map[string][]db.PriceSnapshot, len(symbols))
	for s := range symbols {
		snaps, err := d.ListPriceSnapshots(ctx, s, preload, to)
		if err == nil {
			snapByAsset[s] = snaps
		}
	}
	priceAt := func(symbol string, at time.Time) (float64, bool) {
		snaps := snapByAsset[symbol]
		if len(snaps) == 0 {
			return 0, false
		}
		// Latest snapshot on or before `at`.
		i := sort.Search(len(snaps), func(i int) bool { return snaps[i].At.After(at) })
		if i == 0 {
			return 0, false
		}
		return snaps[i-1].Price, true
	}

	// Preload FX history for every supported currency.
	fxByCur := make(map[domain.Currency][]db.FxRate, len(domain.AllCurrencies))
	for _, c := range domain.AllCurrencies {
		if c == domain.USD {
			continue
		}
		rates, err := d.ListFxRates(ctx, c, preload, to)
		if err == nil {
			fxByCur[c] = rates
		}
	}
	fxAt := func(c domain.Currency, at time.Time) (float64, bool) {
		if c == domain.USD {
			return 1.0, true
		}
		rates := fxByCur[c]
		if len(rates) == 0 {
			return 0, false
		}
		i := sort.Search(len(rates), func(i int) bool { return rates[i].At.After(at) })
		if i == 0 {
			return 0, false
		}
		return rates[i-1].USDRate, true
	}

	raw := portfolio.SeriesFromTransactions(txs, from, to, priceAt, fxAt, curFn, base)
	out := make([]performancePoint, 0, len(raw))
	for _, p := range raw {
		out = append(out, performancePoint{
			At:    p.At.Format(time.RFC3339),
			Value: p.Value,
		})
	}
	return out
}
