package api

import (
	"net/http"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/prices"
)

// DefaultSessionLifetime is used when NewRouter is called with a
// non-positive lifetime.
const DefaultSessionLifetime = 30 * 24 * time.Hour

// NewRouter returns the v1 API mux. /api/v1/version and /api/v1/login
// are public; every other route requires either a Bearer API token or
// a valid pt_session cookie (with X-CSRF-Token header on mutations).
//
// An optional FxHistoryProvider overrides the default Frankfurter
// backing for GET /api/v1/fx/rate — useful in tests.
//
// The returned *http.ServeMux can be extended by the caller (e.g., to
// mount a static-file handler at "/").
func NewRouter(d *db.DB, sessionLifetime time.Duration, fxHistory ...prices.FxHistoryProvider) *http.ServeMux {
	if sessionLifetime <= 0 {
		sessionLifetime = DefaultSessionLifetime
	}
	var fxHist prices.FxHistoryProvider = prices.NewFrankfurter(nil)
	if len(fxHistory) > 0 && fxHistory[0] != nil {
		fxHist = fxHistory[0]
	}
	mw := &auth.Middleware{DB: d, SessionLifetime: sessionLifetime}
	mux := http.NewServeMux()

	// Public
	mux.HandleFunc("GET /api/v1/version", versionHandler)
	mux.HandleFunc("POST /api/v1/login", loginHandler(d, sessionLifetime))

	protect := func(h http.HandlerFunc) http.Handler { return mw.Handler(h) }

	mux.Handle("POST /api/v1/logout", protect(logoutHandler(d)))
	mux.Handle("POST /api/v1/password", protect(changePasswordHandler(d)))

	mux.Handle("GET /api/v1/me", protect(meHandler(d)))
	mux.Handle("PATCH /api/v1/me", protect(updateMeHandler(d)))

	mux.Handle("GET /api/v1/me/tokens", protect(listMyTokensHandler(d)))
	mux.Handle("POST /api/v1/me/tokens", protect(createMyTokenHandler(d)))
	mux.Handle("DELETE /api/v1/me/tokens/{id}", protect(revokeMyTokenHandler(d)))

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

	mux.Handle("GET /api/v1/fx/rate", protect(fxRateHandler(fxHist)))

	return mux
}
