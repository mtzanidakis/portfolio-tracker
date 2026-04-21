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

// Option tweaks optional NewRouter dependencies (FX provider, price
// refresher, …). Call sites that don't need any optional wiring can
// skip this argument entirely.
type Option func(*routerCfg)

type routerCfg struct {
	fxHistory     prices.FxHistoryProvider
	refresher     PriceRefresher
	lookupYahoo   prices.SymbolLookup
	lookupCoinGko prices.SymbolLookup
}

// WithFxHistory overrides the default Frankfurter backing for
// GET /api/v1/fx/rate. Used by tests.
func WithFxHistory(p prices.FxHistoryProvider) Option {
	return func(c *routerCfg) { c.fxHistory = p }
}

// WithPriceRefresher attaches a price refresher so the router exposes
// POST /api/v1/prices/refresh. Without this option, the endpoint is
// not registered.
func WithPriceRefresher(r PriceRefresher) Option {
	return func(c *routerCfg) { c.refresher = r }
}

// WithAssetLookups wires the providers backing GET /api/v1/assets/lookup
// (auto-fill in the "add asset" form). Either argument may be nil — the
// handler responds 503 when the requested provider is missing.
func WithAssetLookups(yahoo, coingecko prices.SymbolLookup) Option {
	return func(c *routerCfg) {
		c.lookupYahoo = yahoo
		c.lookupCoinGko = coingecko
	}
}

// NewRouter returns the v1 API mux. /api/v1/version and /api/v1/login
// are public; every other route requires either a Bearer API token or
// a valid pt_session cookie (with X-CSRF-Token header on mutations).
//
// The returned *http.ServeMux can be extended by the caller (e.g., to
// mount a static-file handler at "/").
func NewRouter(d *db.DB, sessionLifetime time.Duration, opts ...Option) *http.ServeMux {
	if sessionLifetime <= 0 {
		sessionLifetime = DefaultSessionLifetime
	}
	cfg := &routerCfg{fxHistory: prices.NewFrankfurter(nil)}
	for _, opt := range opts {
		opt(cfg)
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
	mux.Handle("GET /api/v1/assets/lookup", protect(lookupAssetHandler(cfg.lookupYahoo, cfg.lookupCoinGko)))
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

	mux.Handle("GET /api/v1/fx/rate", protect(fxRateHandler(cfg.fxHistory)))

	if cfg.refresher != nil {
		mux.Handle("POST /api/v1/prices/refresh", protect(refreshPricesHandler(cfg.refresher)))
	}

	return mux
}
