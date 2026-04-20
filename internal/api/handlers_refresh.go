package api

import (
	"context"
	"net/http"
)

// PriceRefresher is the subset of prices.Service that the API layer
// needs in order to expose an on-demand refresh endpoint. Declared as
// an interface so tests can inject a fake without importing the real
// service.
type PriceRefresher interface {
	RefreshAll(ctx context.Context) error
}

// refreshPricesHandler triggers an immediate price + FX refresh.
// prices.Service.RefreshAll swallows per-provider errors and logs them,
// so this endpoint always responds 200 once the refresh completes.
func refreshPricesHandler(r PriceRefresher) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if err := r.RefreshAll(req.Context()); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
