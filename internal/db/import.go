package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/importers"
)

// ImportResult summarises what ApplyImport did. Returned to the caller
// (and surfaced in the UI "done" step) so the user can see whether
// their selections matched up with existing data or not.
type ImportResult struct {
	AccountsCreated     int      `json:"accounts_created"`
	AccountsReused      int      `json:"accounts_reused"`
	AssetsCreated       int      `json:"assets_created"`
	AssetsReused        int      `json:"assets_reused"`
	TransactionsCreated int      `json:"transactions_created"`
	Warnings            []string `json:"warnings,omitempty"`
}

// FxKey is the composite key used by the FX rate map passed into
// ApplyImport. The date component is the activity's calendar day in
// UTC (YYYY-MM-DD) so Frankfurter's per-day resolution is preserved
// without leaking time.Time equality rules through the map.
type FxKey struct {
	From domain.Currency
	Date string
}

// ApplyImport creates the selected accounts / assets from the plan and
// inserts every transaction in a single SQLite transaction. Any error
// rolls back; on success all rows are visible atomically.
//
// fxRates pre-computes the (fromCurrency, YYYY-MM-DD) → fxToBase
// mapping so the DB transaction doesn't hold the write lock while an
// external HTTP provider is called. Rates for "currency == base" are
// implicit (1.0) and don't need to be in the map.
//
// userBase is the user's base currency — used both to short-circuit
// same-currency transactions to fxToBase=1 and to surface a useful
// error when a required rate is missing from the map.
func (db *DB) ApplyImport(
	ctx context.Context,
	userID int64,
	userBase domain.Currency,
	plan *importers.ImportPlan,
	fxRates map[FxKey]float64,
) (ImportResult, error) {
	res := ImportResult{Warnings: plan.Warnings}

	// Maps sourceID → resolved DB id / symbol. Populated as we walk
	// the selected accounts/assets; consumed by the transaction loop.
	accountIDs := map[string]int64{}
	assetSyms := map[string]string{}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("begin import tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after Commit

	// --- accounts ------------------------------------------------------
	for _, a := range plan.Accounts {
		if !a.Selected {
			continue
		}
		if a.MapToID != 0 {
			// Caller asked us to reuse an existing account — verify
			// ownership so a hostile client can't hand us someone
			// else's account id. Same query shape as GetAccount.
			var (
				owner int64
				name  string
				cur   string
			)
			err := tx.QueryRowContext(ctx,
				`SELECT user_id, name, currency FROM accounts WHERE id = ?`,
				a.MapToID,
			).Scan(&owner, &name, &cur)
			if err != nil {
				return res, fmt.Errorf("map account %d: %w", a.MapToID, err)
			}
			if owner != userID {
				return res, fmt.Errorf("account %d not owned by current user", a.MapToID)
			}
			accountIDs[a.SourceID] = a.MapToID
			res.AccountsReused++
			continue
		}
		if !a.Currency.Valid() {
			return res, fmt.Errorf("account %q has invalid currency %q", a.Name, a.Currency)
		}
		result, err := tx.ExecContext(ctx, `
            INSERT INTO accounts(user_id, name, type, short, color, currency)
            VALUES (?, ?, ?, ?, ?, ?)`,
			userID, a.Name, "", shortFromName(a.Name), "", string(a.Currency),
		)
		if err != nil {
			return res, fmt.Errorf("insert account %q: %w", a.Name, err)
		}
		id, err := result.LastInsertId()
		if err != nil {
			return res, err
		}
		accountIDs[a.SourceID] = id
		res.AccountsCreated++
	}

	// --- assets --------------------------------------------------------
	for _, as := range plan.Assets {
		if !as.Selected {
			continue
		}
		if as.MapToSymbol != "" {
			// Verify the target asset exists (shared across users, so
			// no ownership check is required). Same SQL as GetAsset.
			var got string
			err := tx.QueryRowContext(ctx,
				`SELECT symbol FROM assets WHERE symbol = ?`, as.MapToSymbol,
			).Scan(&got)
			if err != nil {
				return res, fmt.Errorf("map asset %q: %w", as.MapToSymbol, err)
			}
			assetSyms[as.SourceID] = got
			res.AssetsReused++
			continue
		}
		if !as.Type.Valid() {
			return res, fmt.Errorf("asset %q has invalid type %q", as.Symbol, as.Type)
		}
		if !as.Currency.Valid() {
			return res, fmt.Errorf("asset %q has invalid currency %q", as.Symbol, as.Currency)
		}
		_, err := tx.ExecContext(ctx, `
            INSERT INTO assets(symbol, name, type, currency, provider, provider_id, logo_url)
            VALUES (?, ?, ?, ?, ?, ?, ?)
            ON CONFLICT(symbol) DO UPDATE SET
                name        = excluded.name,
                type        = excluded.type,
                currency    = excluded.currency,
                provider    = excluded.provider,
                provider_id = excluded.provider_id,
                logo_url    = excluded.logo_url`,
			as.Symbol, as.Name, string(as.Type), string(as.Currency),
			as.Provider, as.ProviderID, as.LogoURL,
		)
		if err != nil {
			return res, fmt.Errorf("upsert asset %q: %w", as.Symbol, err)
		}
		assetSyms[as.SourceID] = as.Symbol
		res.AssetsCreated++
	}

	// --- transactions --------------------------------------------------
	for i, t := range plan.Transactions {
		accID, ok := accountIDs[t.AccountSourceID]
		if !ok {
			// Account was deselected or unknown — skip silently.
			// Counted as a warning at the end of the loop.
			continue
		}
		sym, ok := assetSyms[t.AssetSourceID]
		if !ok {
			continue
		}
		if !t.Side.Valid() {
			return res, fmt.Errorf("transaction %d has invalid side %q", i, t.Side)
		}
		fx, err := resolveFx(t.Currency, userBase, t.OccurredAt, fxRates)
		if err != nil {
			return res, err
		}
		if _, err := tx.ExecContext(ctx, `
            INSERT INTO transactions(user_id, account_id, asset_symbol, side,
                                     qty, price, fee, fx_to_base, occurred_at, note)
            VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			userID, accID, sym, string(t.Side),
			t.Qty, t.Price, t.Fee, fx, t.OccurredAt, t.Note,
		); err != nil {
			return res, fmt.Errorf("insert tx %d: %w", i, err)
		}
		res.TransactionsCreated++
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("commit import: %w", err)
	}

	// Count transactions dropped because their account / asset was
	// deselected. Useful signal for the user.
	dropped := len(plan.Transactions) - res.TransactionsCreated
	if dropped > 0 {
		res.Warnings = append(res.Warnings,
			fmt.Sprintf("%d transaction(s) skipped because their account or asset was not imported", dropped))
	}

	return res, nil
}

// resolveFx returns fxToBase for a transaction in cur → base at at.
// Missing rates produce a hard error (we never make up an FX rate) so
// the apply phase fails before partially writing.
func resolveFx(cur, base domain.Currency, at time.Time, rates map[FxKey]float64) (float64, error) {
	if cur == base {
		return 1.0, nil
	}
	key := FxKey{From: cur, Date: at.UTC().Format("2006-01-02")}
	if v, ok := rates[key]; ok && v > 0 {
		return v, nil
	}
	return 0, fmt.Errorf("missing fx rate %s→%s on %s", cur, base, key.Date)
}

// shortFromName derives a 2-3 letter tag for the account badge (same
// convention as the AccountModal's quick-fill). Importers don't let
// the user pick one, so we produce something readable from the name.
func shortFromName(name string) string {
	parts := strings.Fields(strings.TrimSpace(name))
	if len(parts) == 0 {
		return "??"
	}
	if len(parts) == 1 {
		s := strings.ToUpper(parts[0])
		if len(s) > 3 {
			return s[:3]
		}
		return s
	}
	var b strings.Builder
	for i, p := range parts {
		if i >= 3 {
			break
		}
		b.WriteByte(strings.ToUpper(p)[0])
	}
	return b.String()
}

// RequiredFxKeys walks the plan's transactions and returns the set of
// (currency, date) pairs that need a Frankfurter lookup to apply —
// same-currency transactions are excluded. The analyze/apply handler
// uses this to pre-fetch all rates before opening the DB write tx.
func RequiredFxKeys(plan *importers.ImportPlan, base domain.Currency) []FxKey {
	seen := map[FxKey]struct{}{}
	out := []FxKey{}
	for _, t := range plan.Transactions {
		if t.Currency == "" || t.Currency == base {
			continue
		}
		k := FxKey{From: t.Currency, Date: t.OccurredAt.UTC().Format("2006-01-02")}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		out = append(out, k)
	}
	return out
}

// Compile-time sanity check: sql.Tx must implement the subset of the
// DB method surface we use above (QueryRowContext + ExecContext).
var _ interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryRowContext(context.Context, string, ...any) *sql.Row
} = (*sql.Tx)(nil)
