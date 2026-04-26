package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// testSecret is the cookie-signing key used across api tests. Random,
// stable for the test run so request fixtures can be replayed.
var testSecret = []byte("api-test-secret-32-bytes-padding!")

// testEnv bundles a started test server with a user + active token.
type testEnv struct {
	t     *testing.T
	db    *db.DB
	srv   *httptest.Server
	user  *domain.User
	token string
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	d, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(t.Context()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	u := &domain.User{Email: "api@test.io", Name: "API User", BaseCurrency: domain.EUR}
	if err := d.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	plain, hash, err := auth.GenerateToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if err := d.CreateToken(t.Context(), &domain.Token{
		UserID: u.ID, Name: "api-test", Hash: hash,
	}); err != nil {
		t.Fatalf("create token: %v", err)
	}

	srv := httptest.NewServer(NewRouter(d, time.Hour, testSecret))
	t.Cleanup(srv.Close)
	return &testEnv{t: t, db: d, srv: srv, user: u, token: plain}
}

func (e *testEnv) do(method, path string, body any) *http.Response {
	e.t.Helper()
	var bodyR io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyR = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(e.t.Context(), method, e.srv.URL+path, bodyR)
	if err != nil {
		e.t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		e.t.Fatalf("do: %v", err)
	}
	e.t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func (e *testEnv) decode(resp *http.Response, v any) {
	e.t.Helper()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		e.t.Fatalf("decode: %v", err)
	}
}

func TestVersion_Public(t *testing.T) {
	env := setup(t)
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, env.srv.URL+"/api/v1/version", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestMe_Protected(t *testing.T) {
	env := setup(t)

	// Without token → 401.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, env.srv.URL+"/api/v1/me", nil)
	resp, _ := http.DefaultClient.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 unauth, got %d", resp.StatusCode)
	}

	// With token.
	resp2 := env.do(http.MethodGet, "/api/v1/me", nil)
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("status: %d", resp2.StatusCode)
	}
	var u domain.User
	env.decode(resp2, &u)
	if u.ID != env.user.ID {
		t.Errorf("wrong user: %+v", u)
	}
}

func TestUpdateMe_BaseCurrency(t *testing.T) {
	env := setup(t)
	resp := env.do(http.MethodPatch, "/api/v1/me",
		map[string]string{"base_currency": "USD"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: %d", resp.StatusCode)
	}
	got, _ := env.db.GetUser(t.Context(), env.user.ID)
	if got.BaseCurrency != domain.USD {
		t.Errorf("base currency not updated: %s", got.BaseCurrency)
	}
}

func TestAccountCRUD_HTTP(t *testing.T) {
	env := setup(t)

	// Create.
	resp := env.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name":     "Broker",
		"type":     "Brokerage",
		"short":    "BR",
		"color":    "#c8502a",
		"currency": "USD",
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create status: %d", resp.StatusCode)
	}
	var acc domain.Account
	env.decode(resp, &acc)

	// List.
	resp = env.do(http.MethodGet, "/api/v1/accounts", nil)
	var accs []*domain.Account
	env.decode(resp, &accs)
	if len(accs) != 1 {
		t.Errorf("list len: %d", len(accs))
	}

	// Update.
	resp = env.do(http.MethodPatch, "/api/v1/accounts/"+itoa(acc.ID),
		map[string]any{"name": "Renamed"})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("update status: %d", resp.StatusCode)
	}

	// Delete.
	resp = env.do(http.MethodDelete, "/api/v1/accounts/"+itoa(acc.ID), nil)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("delete status: %d", resp.StatusCode)
	}
}

func TestAssetUpsertAndSearch(t *testing.T) {
	env := setup(t)

	resp := env.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"symbol": "AAPL", "name": "Apple Inc.",
		"type": "stock", "currency": "USD",
	})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create: %d", resp.StatusCode)
	}

	resp = env.do(http.MethodGet, "/api/v1/assets?q=app", nil)
	var list []domain.Asset
	env.decode(resp, &list)
	if len(list) != 1 || list[0].Symbol != "AAPL" {
		t.Errorf("search: %+v", list)
	}
}

func TestTransactionCreate_RequiresOwnedAccount(t *testing.T) {
	env := setup(t)

	// Create asset + account.
	_ = env.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"symbol": "AAPL", "name": "Apple", "type": "stock", "currency": "USD",
	})
	resp := env.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name": "X", "type": "T", "short": "X", "color": "#000", "currency": "USD",
	})
	var acc domain.Account
	env.decode(resp, &acc)

	// Create tx.
	resp = env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   acc.ID,
		"asset_symbol": "AAPL",
		"side":         "buy",
		"qty":          3,
		"price":        100,
		"fee":          0,
		"fx_to_base":   0.9,
		"occurred_at":  time.Now().UTC().Format(time.RFC3339),
	})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: %d", resp.StatusCode)
	}

	// List.
	resp = env.do(http.MethodGet, "/api/v1/transactions", nil)
	var txs []*domain.Transaction
	env.decode(resp, &txs)
	if len(txs) != 1 {
		t.Errorf("list len: %d", len(txs))
	}
}

func TestHoldings_Computes(t *testing.T) {
	env := setup(t)

	// Seed one asset + account + transaction.
	_ = env.do(http.MethodPost, "/api/v1/assets", map[string]any{
		"symbol": "AAPL", "name": "Apple", "type": "stock", "currency": "USD",
	})
	resp := env.do(http.MethodPost, "/api/v1/accounts", map[string]any{
		"name": "X", "type": "T", "short": "X", "color": "#000", "currency": "USD",
	})
	var acc domain.Account
	env.decode(resp, &acc)

	_ = env.do(http.MethodPost, "/api/v1/transactions", map[string]any{
		"account_id":   acc.ID,
		"asset_symbol": "AAPL",
		"side":         "buy",
		"qty":          10,
		"price":        100,
		"fx_to_base":   0.9,
		"occurred_at":  "2026-01-01T00:00:00Z",
	})

	// Seed a latest price + FX for EUR (user's base) and USD (native).
	_ = env.db.SetLatestPrice(t.Context(), db.LatestPrice{
		Symbol: "AAPL", Price: 110, FetchedAt: time.Now(),
	})
	_ = env.db.SetLatestFxRate(t.Context(), db.LatestFxRate{
		Currency: domain.EUR, USDRate: 1.10, FetchedAt: time.Now(),
	})

	resp = env.do(http.MethodGet, "/api/v1/holdings", nil)
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: %d body=%s", resp.StatusCode, body)
	}
	if !strings.Contains(string(body), "AAPL") {
		t.Errorf("expected AAPL in response: %s", body)
	}
}

func TestPerformance_Minimal(t *testing.T) {
	env := setup(t)
	resp := env.do(http.MethodGet, "/api/v1/performance?tf=1M", nil)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: %d", resp.StatusCode)
	}
	var p performanceResponse
	env.decode(resp, &p)
	if p.Timeframe != "1M" || p.Currency != "EUR" {
		t.Errorf("unexpected: %+v", p)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }
