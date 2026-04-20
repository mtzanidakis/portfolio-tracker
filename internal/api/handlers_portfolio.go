package api

import (
	"net/http"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
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
		prices, fx, currencies, err := buildLookups(r.Context(), d)
		if err != nil {
			writeDBError(w, err)
			return
		}
		values := portfolio.ValueHoldings(holdings, prices, fx, currencies, u.BaseCurrency)
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
		resp := performanceResponse{
			Total:     total,
			Cost:      cost,
			PnL:       total - cost,
			AnyStale:  anyStale,
			PnLPct:    pct,
			Currency:  string(u.BaseCurrency),
			Timeframe: tf,
			Series:    []performancePoint{}, // populated once price history is available (Step 7+)
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
