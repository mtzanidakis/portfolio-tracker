package api

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// stubFx is a deterministic FxHistoryProvider. rate is returned for any
// pair; err (when set) short-circuits every call so we can exercise the
// "provider is down" path without touching the network.
type stubFx struct {
	rate  float64
	err   error
	calls int
}

func (s *stubFx) FetchRate(_ context.Context, _, _ domain.Currency, _ time.Time) (float64, error) {
	s.calls++
	if s.err != nil {
		return 0, s.err
	}
	return s.rate, nil
}

// txFixture creates a EUR-based user's account plus one USD-denominated
// asset and returns the account id.
func txFixture(t *testing.T, env *testEnv, symbol string, cur domain.Currency) int64 {
	t.Helper()
	acc := &domain.Account{
		UserID: env.user.ID, Name: "Brokerage", Type: "Brokerage",
		Short: "BR", Color: "#c8502a", Currency: domain.USD,
	}
	if err := env.db.CreateAccount(t.Context(), acc); err != nil {
		t.Fatalf("create account: %v", err)
	}
	if err := env.db.UpsertAsset(t.Context(), &domain.Asset{
		Symbol: symbol, Name: symbol + " Inc", Type: domain.AssetStock, Currency: cur,
	}); err != nil {
		t.Fatalf("upsert asset: %v", err)
	}
	return acc.ID
}

// A client that omits fx_to_base must get the rate the server resolves
// from the *asset's* currency — never a silent 1.0, which would record
// the cost basis in the wrong currency (the SOPH/OPEN regression).
func TestCreateTransaction_ResolvesFxWhenOmitted(t *testing.T) {
	fx := &stubFx{rate: 0.877}
	env := setupWithFx(t, fx)
	accID := txFixture(t, env, "SOPH", domain.USD)

	resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   accID,
		"asset_symbol": "SOPH",
		"side":         "buy",
		"qty":          10.0,
		"price":        6.54,
		"occurred_at":  "2026-07-22T12:00:00Z",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got domain.Transaction
	env.decode(resp, &got)
	if got.FxToBase != 0.877 {
		t.Errorf("fx_to_base = %v, want 0.877 (resolved server-side)", got.FxToBase)
	}
	if fx.calls != 1 {
		t.Errorf("provider calls = %d, want 1", fx.calls)
	}
}

// Same-currency assets need no provider round-trip.
func TestCreateTransaction_SameCurrencySkipsProvider(t *testing.T) {
	fx := &stubFx{rate: 0.877}
	env := setupWithFx(t, fx)
	accID := txFixture(t, env, "VWCE.DE", domain.EUR)

	resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   accID,
		"asset_symbol": "VWCE.DE",
		"side":         "buy",
		"qty":          3.0,
		"price":        165.32,
		"occurred_at":  "2026-07-16T12:00:00Z",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got domain.Transaction
	env.decode(resp, &got)
	if got.FxToBase != 1 {
		t.Errorf("fx_to_base = %v, want 1", got.FxToBase)
	}
	if fx.calls != 0 {
		t.Errorf("provider called %d times for a base-currency asset", fx.calls)
	}
}

// An explicit rate is honoured verbatim (ptagent --fx, import replay).
func TestCreateTransaction_ExplicitFxWins(t *testing.T) {
	fx := &stubFx{rate: 0.877}
	env := setupWithFx(t, fx)
	accID := txFixture(t, env, "SOPH", domain.USD)

	resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   accID,
		"asset_symbol": "SOPH",
		"side":         "buy",
		"qty":          10.0,
		"price":        6.54,
		"fx_to_base":   0.5,
		"occurred_at":  "2026-07-22T12:00:00Z",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status: %d", resp.StatusCode)
	}
	var got domain.Transaction
	env.decode(resp, &got)
	if got.FxToBase != 0.5 {
		t.Errorf("fx_to_base = %v, want 0.5", got.FxToBase)
	}
	if fx.calls != 0 {
		t.Errorf("provider called %d times despite an explicit rate", fx.calls)
	}
}

// A provider failure must reject the write. Recording the transaction
// with a guessed rate is worse than not recording it at all.
func TestCreateTransaction_FxFailureRejects(t *testing.T) {
	fx := &stubFx{err: errors.New("frankfurter: context deadline exceeded")}
	env := setupWithFx(t, fx)
	accID := txFixture(t, env, "SOPH", domain.USD)

	resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   accID,
		"asset_symbol": "SOPH",
		"side":         "buy",
		"qty":          10.0,
		"price":        6.54,
		"occurred_at":  "2026-07-22T12:00:00Z",
	})
	if resp.StatusCode != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", resp.StatusCode)
	}
	txs, err := env.db.ListTransactions(t.Context(), db.TxFilter{UserID: env.user.ID})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(txs) != 0 {
		t.Errorf("transaction persisted despite the FX failure: %+v", txs)
	}
}

// Zero or negative rates are rejected outright rather than stored.
func TestCreateTransaction_RejectsNonPositiveFx(t *testing.T) {
	env := setupWithFx(t, &stubFx{rate: 0.877})
	accID := txFixture(t, env, "SOPH", domain.USD)

	for _, bad := range []float64{0, -1} {
		resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
			"account_id":   accID,
			"asset_symbol": "SOPH",
			"side":         "buy",
			"qty":          10.0,
			"price":        6.54,
			"fx_to_base":   bad,
			"occurred_at":  "2026-07-22T12:00:00Z",
		})
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("fx_to_base=%v: status = %d, want 400", bad, resp.StatusCode)
		}
	}
}

// Moving a transaction to a new date re-locks the rate for that date.
func TestUpdateTransaction_RelocksFxOnDateChange(t *testing.T) {
	fx := &stubFx{rate: 0.877}
	env := setupWithFx(t, fx)
	accID := txFixture(t, env, "SOPH", domain.USD)

	resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   accID,
		"asset_symbol": "SOPH",
		"side":         "buy",
		"qty":          10.0,
		"price":        6.54,
		"occurred_at":  "2026-07-22T12:00:00Z",
	})
	var created domain.Transaction
	env.decode(resp, &created)

	fx.rate = 0.91
	patch := env.do(http.MethodPatch, "/api/v1/transactions/"+itoa(created.ID), map[string]any{
		"occurred_at": "2026-06-30T12:00:00Z",
	})
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", patch.StatusCode)
	}
	var updated domain.Transaction
	env.decode(patch, &updated)
	if updated.FxToBase != 0.91 {
		t.Errorf("fx_to_base = %v, want 0.91 (re-locked for the new date)", updated.FxToBase)
	}
}

// Editing an unrelated field leaves the original lock alone.
func TestUpdateTransaction_KeepsFxOnUnrelatedEdit(t *testing.T) {
	fx := &stubFx{rate: 0.877}
	env := setupWithFx(t, fx)
	accID := txFixture(t, env, "SOPH", domain.USD)

	resp := env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   accID,
		"asset_symbol": "SOPH",
		"side":         "buy",
		"qty":          10.0,
		"price":        6.54,
		"occurred_at":  "2026-07-22T12:00:00Z",
	})
	var created domain.Transaction
	env.decode(resp, &created)

	fx.rate = 0.91
	patch := env.do(http.MethodPatch, "/api/v1/transactions/"+itoa(created.ID), map[string]any{
		"note": "rebalance",
	})
	if patch.StatusCode != http.StatusOK {
		t.Fatalf("status: %d", patch.StatusCode)
	}
	var updated domain.Transaction
	env.decode(patch, &updated)
	if updated.FxToBase != 0.877 {
		t.Errorf("fx_to_base = %v, want the original 0.877", updated.FxToBase)
	}
}
