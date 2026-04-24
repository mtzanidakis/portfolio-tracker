package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/importers"
	"github.com/mtzanidakis/portfolio-tracker/internal/prices"
)

// maxImportBytes caps request bodies for /import/* endpoints. The
// reference Ghostfolio export in our repo is ~120 KB — 10 MB leaves
// comfortable headroom for multi-year exports while still bounding a
// misbehaving client.
const maxImportBytes = 10 << 20

// importAnalyzeHandler parses the uploaded source-specific dump,
// enriches asset metadata where a provider lookup is cheap (Yahoo
// name + stock/etf classification), and annotates each account/asset
// with whether the current user already has a match. The returned
// plan is meant to be edited client-side and echoed back to /apply.
func importAnalyzeHandler(d *db.DB, yahoo, coingecko prices.SymbolLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		source := strings.ToLower(strings.TrimSpace(r.PathValue("source")))

		body, err := io.ReadAll(io.LimitReader(r.Body, maxImportBytes+1))
		if err != nil {
			writeError(w, http.StatusBadRequest, "read body: "+err.Error())
			return
		}
		if len(body) > maxImportBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "import file too large")
			return
		}

		var plan *importers.ImportPlan
		switch source {
		case "ghostfolio":
			plan, err = importers.ParseGhostfolio(body)
		default:
			writeError(w, http.StatusBadRequest, "unknown import source")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		enrichAssets(r.Context(), plan, yahoo, coingecko)
		if err := annotateExistingMatches(r.Context(), d, u.ID, plan); err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, plan)
	}
}

// enrichAssets fills in provider-derived metadata (proper name,
// stock/etf classification, logo URL) for each asset in the plan.
// Best-effort: on error or missing match the parser's defaults stay.
//
// Routing rules: YAHOO-sourced assets go through the Yahoo lookup
// (returns a parqet logo URL for stock/etf). COINGECKO-sourced and
// MANUAL-crypto assets go through the CoinGecko lookup — Ghostfolio
// stores a lot of user-entered crypto as MANUAL (BTC in our fixture).
// When the CoinGecko lookup hits a MANUAL asset, we also promote its
// provider so future refreshes find the coin.
func enrichAssets(ctx context.Context, plan *importers.ImportPlan, yahoo, coingecko prices.SymbolLookup) {
	apply := func(a *importers.ImportAsset, info *prices.SymbolInfo) {
		if info == nil {
			return
		}
		if info.Name != "" {
			a.Name = info.Name
		}
		if info.AssetType != "" && info.AssetType.Valid() {
			a.Type = info.AssetType
		}
		if info.Currency != "" && info.Currency.Valid() {
			a.Currency = info.Currency
		}
		if info.ProviderID != "" {
			a.ProviderID = info.ProviderID
		}
		if info.LogoURL != "" {
			a.LogoURL = info.LogoURL
		}
	}
	for i := range plan.Assets {
		a := &plan.Assets[i]
		switch {
		case a.Provider == "yahoo" && yahoo != nil:
			info, err := yahoo.LookupSymbol(ctx, a.Symbol)
			if err == nil {
				apply(a, info)
			}
		case a.Provider == "coingecko" && coingecko != nil:
			info, err := coingecko.LookupSymbol(ctx, a.Symbol)
			if err == nil {
				apply(a, info)
			}
		case a.Provider == "manual" && a.Type == domain.AssetCrypto && coingecko != nil:
			info, err := coingecko.LookupSymbol(ctx, a.Symbol)
			if err == nil && info != nil {
				apply(a, info)
				// Promote to a real provider so background price
				// refreshes (and the logo proxy) have something to
				// work with going forward.
				a.Provider = "coingecko"
			}
		}
	}
}

// annotateExistingMatches populates ExistingID / ExistingMatch so the
// UI can prefill the "map to existing" dropdown. Account matching is
// case-insensitive on name; asset matching is case-insensitive on
// symbol (assets are shared across users).
func annotateExistingMatches(ctx context.Context, d *db.DB, userID int64, plan *importers.ImportPlan) error {
	accs, err := d.ListAccounts(ctx, userID)
	if err != nil {
		return err
	}
	byName := map[string]int64{}
	for _, a := range accs {
		byName[strings.ToLower(a.Name)] = a.ID
	}
	for i := range plan.Accounts {
		if id, ok := byName[strings.ToLower(plan.Accounts[i].Name)]; ok {
			plan.Accounts[i].ExistingID = id
		}
	}

	assets, err := d.ListAssets(ctx)
	if err != nil {
		return err
	}
	symSet := map[string]struct{}{}
	for _, a := range assets {
		symSet[strings.ToLower(a.Symbol)] = struct{}{}
	}
	// Build a second map preserving the original-cased symbol so we
	// can set MapToSymbol to the exact DB casing (SQLite is case-
	// sensitive on symbol lookups).
	symByLower := map[string]string{}
	for _, a := range assets {
		symByLower[strings.ToLower(a.Symbol)] = a.Symbol
	}
	for i := range plan.Assets {
		key := strings.ToLower(plan.Assets[i].Symbol)
		if stored, ok := symByLower[key]; ok {
			plan.Assets[i].ExistingMatch = true
			// Default to "reuse existing" so the apply phase doesn't
			// upsert and clobber the stored name / provider / logo
			// with whatever the importer extracted. The UI can still
			// show this as "Already exists — will reuse" without any
			// additional state.
			if plan.Assets[i].MapToSymbol == "" {
				plan.Assets[i].MapToSymbol = stored
			}
		}
	}
	return nil
}

// importApplyHandler takes the (possibly edited) plan from the client,
// pre-fetches every FX rate it needs, then hands the work to
// DB.ApplyImport inside a single SQLite transaction. FX fetches happen
// before the write transaction so we don't hold the SQLite writer
// lock during external HTTP I/O.
func importApplyHandler(d *db.DB, fx prices.FxHistoryProvider) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var plan importers.ImportPlan
		r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
		if err := decodeJSON(r, &plan); err != nil {
			writeError(w, http.StatusBadRequest, "invalid plan: "+err.Error())
			return
		}

		user, err := d.GetUser(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}

		needed := db.RequiredFxKeys(&plan, user.BaseCurrency)
		rates := make(map[db.FxKey]float64, len(needed))
		for _, k := range needed {
			date, perr := time.Parse("2006-01-02", k.Date)
			if perr != nil {
				writeError(w, http.StatusInternalServerError, "bad fx date: "+k.Date)
				return
			}
			rate, err := fx.FetchRate(r.Context(), k.From, user.BaseCurrency, date)
			if err != nil {
				writeError(w, http.StatusBadGateway,
					fmt.Sprintf("fx %s→%s on %s: %v", k.From, user.BaseCurrency, k.Date, err))
				return
			}
			rates[k] = rate
		}

		res, err := d.ApplyImport(r.Context(), u.ID, user.BaseCurrency, &plan, rates)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, res)
	}
}
