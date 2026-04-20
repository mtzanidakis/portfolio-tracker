package prices

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestYahoo_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v7/finance/quote" {
			t.Errorf("bad path: %s", r.URL.Path)
		}
		syms := r.URL.Query().Get("symbols")
		if syms != "AAPL,NVDA" {
			t.Errorf("bad symbols: %q", syms)
		}
		if ua := r.Header.Get("User-Agent"); ua == "" {
			t.Error("missing User-Agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"quoteResponse":{"result":[
            {"symbol":"AAPL","regularMarketPrice":198.2,"currency":"USD"},
            {"symbol":"NVDA","regularMarketPrice":462.8,"currency":"USD"}
        ],"error":null}}`))
	}))
	defer srv.Close()

	y := &YahooProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	out, err := y.Fetch(t.Context(), []string{"AAPL", "NVDA"})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("len: %d", len(out))
	}
	if out[0].Symbol != "AAPL" || out[0].Price != 198.2 {
		t.Errorf("AAPL: %+v", out[0])
	}
}

func TestYahoo_EmptySymbols(t *testing.T) {
	y := &YahooProvider{BaseURL: "http://unused", HTTP: http.DefaultClient}
	out, err := y.Fetch(t.Context(), nil)
	if err != nil || out != nil {
		t.Errorf("expected nil,nil; got %v, %v", out, err)
	}
}

func TestYahoo_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte("rate limited"))
	}))
	defer srv.Close()

	y := &YahooProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	if _, err := y.Fetch(t.Context(), []string{"X"}); err == nil {
		t.Fatal("expected error on 429")
	}
}
