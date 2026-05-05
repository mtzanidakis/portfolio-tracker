package prices

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestCoinGecko_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple/price" {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		ids := strings.Split(q.Get("ids"), ",")
		sort.Strings(ids)
		if strings.Join(ids, ",") != "bitcoin,ethereum" {
			t.Errorf("ids: %q", q.Get("ids"))
		}
		if q.Get("vs_currencies") != "usd" {
			t.Errorf("vs: %q", q.Get("vs_currencies"))
		}
		_, _ = w.Write([]byte(`{"bitcoin":{"usd":67200.12},"ethereum":{"usd":3420.4}}`))
	}))
	defer srv.Close()

	c := &CoinGeckoProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	out, err := c.Fetch(t.Context(), []AssetFetchRef{
		{ID: "bitcoin", Currency: domain.USD},
		{ID: "ethereum", Currency: domain.USD},
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: %d", len(out))
	}
	for _, q := range out {
		if q.Currency != domain.USD {
			t.Errorf("quote currency: got %s, want USD", q.Currency)
		}
	}
}

// TestCoinGecko_Fetch_PerAssetCurrency pins the EUR-native flow: an
// asset with Currency=EUR must hit /simple/price with vs_currencies=eur,
// and the returned PriceQuote.Currency must echo EUR — no FX hop
// through USD.
func TestCoinGecko_Fetch_PerAssetCurrency(t *testing.T) {
	var sawCurrencies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vs := r.URL.Query().Get("vs_currencies")
		sawCurrencies = append(sawCurrencies, vs)
		switch vs {
		case "eur":
			_, _ = w.Write([]byte(`{"bitcoin":{"eur":69000.5}}`))
		case "usd":
			_, _ = w.Write([]byte(`{"ethereum":{"usd":3420.4}}`))
		default:
			t.Errorf("unexpected vs_currencies=%q", vs)
		}
	}))
	defer srv.Close()

	c := &CoinGeckoProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	out, err := c.Fetch(t.Context(), []AssetFetchRef{
		{ID: "bitcoin", Currency: domain.EUR},
		{ID: "ethereum", Currency: domain.USD},
	})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: %d", len(out))
	}
	byID := map[string]PriceQuote{}
	for _, q := range out {
		byID[q.Symbol] = q
	}
	if byID["bitcoin"].Currency != domain.EUR || byID["bitcoin"].Price != 69000.5 {
		t.Errorf("bitcoin quote: %+v", byID["bitcoin"])
	}
	if byID["ethereum"].Currency != domain.USD || byID["ethereum"].Price != 3420.4 {
		t.Errorf("ethereum quote: %+v", byID["ethereum"])
	}
	sort.Strings(sawCurrencies)
	if strings.Join(sawCurrencies, ",") != "eur,usd" {
		t.Errorf("vs_currencies seen: %v", sawCurrencies)
	}
}

func TestCoinGecko_FetchHistory_PerAssetCurrency(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/coins/bitcoin/market_chart") {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		if vs := r.URL.Query().Get("vs_currency"); vs != "eur" {
			t.Errorf("vs_currency: got %q, want eur", vs)
		}
		_, _ = w.Write([]byte(`{"prices":[[1714521600000,69000.5],[1714608000000,69500.25]]}`))
	}))
	defer srv.Close()

	c := &CoinGeckoProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	out, err := c.FetchHistory(t.Context(), AssetFetchRef{ID: "bitcoin", Currency: domain.EUR}, time.Time{})
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: %d", len(out))
	}
	for _, s := range out {
		if s.Currency != domain.EUR {
			t.Errorf("snapshot currency: got %s, want EUR", s.Currency)
		}
		if s.Symbol != "bitcoin" {
			t.Errorf("snapshot symbol: %s", s.Symbol)
		}
	}
}

func TestCoinGecko_WithAPIKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if k := r.Header.Get("x-cg-demo-api-key"); k != "my-key" {
			t.Errorf("api key header missing: %q", k)
		}
		_, _ = w.Write([]byte(`{"bitcoin":{"usd":67000}}`))
	}))
	defer srv.Close()

	c := &CoinGeckoProvider{BaseURL: srv.URL, APIKey: "my-key", HTTP: srv.Client()}
	if _, err := c.Fetch(t.Context(), []AssetFetchRef{{ID: "bitcoin", Currency: domain.USD}}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
}
