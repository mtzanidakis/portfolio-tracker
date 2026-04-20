package api

import (
	"net/http"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
)

// NewRouter returns the v1 API mux. /api/v1/version is public; every
// other route requires a Bearer token. The returned *http.ServeMux can
// be extended by the caller (e.g., to mount a static-file handler at "/").
func NewRouter(d *db.DB) *http.ServeMux {
	mw := &auth.Middleware{DB: d}
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /api/v1/version", versionHandler)

	// Protected — bind each route through the auth middleware.
	protect := func(h http.HandlerFunc) http.Handler {
		return mw.Handler(h)
	}

	mux.Handle("GET /api/v1/me", protect(meHandler(d)))
	mux.Handle("PATCH /api/v1/me", protect(updateMeHandler(d)))

	mux.Handle("GET /api/v1/accounts", protect(listAccountsHandler(d)))
	mux.Handle("POST /api/v1/accounts", protect(createAccountHandler(d)))
	mux.Handle("GET /api/v1/accounts/{id}", protect(getAccountHandler(d)))
	mux.Handle("PATCH /api/v1/accounts/{id}", protect(updateAccountHandler(d)))
	mux.Handle("DELETE /api/v1/accounts/{id}", protect(deleteAccountHandler(d)))

	mux.Handle("GET /api/v1/assets", protect(listAssetsHandler(d)))
	mux.Handle("POST /api/v1/assets", protect(upsertAssetHandler(d)))
	mux.Handle("GET /api/v1/assets/{symbol}", protect(getAssetHandler(d)))
	mux.Handle("DELETE /api/v1/assets/{symbol}", protect(deleteAssetHandler(d)))

	mux.Handle("GET /api/v1/transactions", protect(listTransactionsHandler(d)))
	mux.Handle("POST /api/v1/transactions", protect(createTransactionHandler(d)))
	mux.Handle("GET /api/v1/transactions/{id}", protect(getTransactionHandler(d)))
	mux.Handle("PATCH /api/v1/transactions/{id}", protect(updateTransactionHandler(d)))
	mux.Handle("DELETE /api/v1/transactions/{id}", protect(deleteTransactionHandler(d)))

	mux.Handle("GET /api/v1/holdings", protect(holdingsHandler(d)))
	mux.Handle("GET /api/v1/allocations", protect(allocationsHandler(d)))
	mux.Handle("GET /api/v1/performance", protect(performanceHandler(d)))

	return mux
}
