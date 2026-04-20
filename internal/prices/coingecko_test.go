package prices

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCoinGecko_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/simple/price" {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("ids") != "bitcoin,ethereum" {
			t.Errorf("ids: %q", q.Get("ids"))
		}
		if q.Get("vs_currencies") != "usd" {
			t.Errorf("vs: %q", q.Get("vs_currencies"))
		}
		_, _ = w.Write([]byte(`{"bitcoin":{"usd":67200.12},"ethereum":{"usd":3420.4}}`))
	}))
	defer srv.Close()

	c := &CoinGeckoProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	out, err := c.Fetch(t.Context(), []string{"bitcoin", "ethereum"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: %d", len(out))
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
	if _, err := c.Fetch(t.Context(), []string{"bitcoin"}); err != nil {
		t.Fatalf("fetch: %v", err)
	}
}
