package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/prices"
)

type fxRateResponse struct {
	From string  `json:"from"`
	To   string  `json:"to"`
	At   string  `json:"at,omitempty"` // empty means "latest"
	Rate float64 `json:"rate"`
}

// fxRateHandler returns the FX rate "1 from = rate to" at the given
// date (query param `at`, YYYY-MM-DD). Omit `at` for the latest rate.
// Used by the frontend to auto-fill fx_to_base on transaction entry.
func fxRateHandler(provider prices.FxHistoryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		from := domain.Currency(strings.ToUpper(q.Get("from")))
		to := domain.Currency(strings.ToUpper(q.Get("to")))
		if !from.Valid() || !to.Valid() {
			writeError(w, http.StatusBadRequest, "invalid from/to currency")
			return
		}

		var at time.Time
		if s := q.Get("at"); s != "" {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				writeError(w, http.StatusBadRequest, "at must be YYYY-MM-DD")
				return
			}
			at = t
		}

		rate, err := provider.FetchRate(r.Context(), from, to, at)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		resp := fxRateResponse{From: string(from), To: string(to), Rate: rate}
		if !at.IsZero() {
			resp.At = at.Format("2006-01-02")
		}
		writeJSON(w, http.StatusOK, resp)
	}
}
