package prices

import (
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestFrankfurter_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rates" {
			t.Errorf("path: %q", r.URL.Path)
		}
		q := r.URL.Query()
		if q.Get("base") != "USD" {
			t.Errorf("base: %q", q.Get("base"))
		}
		quotes := strings.Split(q.Get("quotes"), ",")
		if len(quotes) != 3 {
			t.Errorf("quotes: %v", quotes)
		}
		_, _ = w.Write([]byte(`[
			{"date":"2026-04-20","base":"USD","quote":"EUR","rate":0.92},
			{"date":"2026-04-20","base":"USD","quote":"GBP","rate":0.79},
			{"date":"2026-04-20","base":"USD","quote":"JPY","rate":150.5}
		]`))
	}))
	defer srv.Close()

	f := &FrankfurterProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	out, err := f.Fetch(t.Context(), []domain.Currency{domain.EUR, domain.GBP, domain.JPY})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	// 1 EUR = 1/0.92 ≈ 1.0869 USD
	if math.Abs(out[domain.EUR]-1.08695652) > 1e-6 {
		t.Errorf("EUR: %v", out[domain.EUR])
	}
	if out[domain.USD] != 1.0 {
		t.Errorf("USD: %v", out[domain.USD])
	}
}

func TestFrankfurter_Fetch_USDOnly(t *testing.T) {
	// When only USD is requested no HTTP call is issued.
	f := &FrankfurterProvider{BaseURL: "http://unreachable.example", HTTP: http.DefaultClient}
	out, err := f.Fetch(t.Context(), []domain.Currency{domain.USD})
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if out[domain.USD] != 1.0 || len(out) != 1 {
		t.Errorf("unexpected output: %+v", out)
	}
}

func TestFrankfurter_FetchRate_Latest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("base") != "USD" || q.Get("quotes") != "EUR" {
			t.Errorf("params: %v", q)
		}
		if q.Get("date") != "" {
			t.Errorf("latest call should not carry a date, got %q", q.Get("date"))
		}
		_, _ = w.Write([]byte(`[{"date":"2026-04-20","base":"USD","quote":"EUR","rate":0.84899}]`))
	}))
	defer srv.Close()

	f := &FrankfurterProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	rate, err := f.FetchRate(t.Context(), domain.USD, domain.EUR, time.Time{})
	if err != nil {
		t.Fatalf("fetch rate: %v", err)
	}
	if math.Abs(rate-0.84899) > 1e-9 {
		t.Errorf("rate: %v", rate)
	}
}

func TestFrankfurter_FetchRate_Historical(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("date"); got != "2026-01-10" {
			t.Errorf("date param: %q", got)
		}
		_, _ = w.Write([]byte(`[{"date":"2026-01-10","base":"USD","quote":"EUR","rate":0.90}]`))
	}))
	defer srv.Close()

	f := &FrankfurterProvider{BaseURL: srv.URL, HTTP: srv.Client()}
	at, _ := time.Parse("2006-01-02", "2026-01-10")
	rate, err := f.FetchRate(t.Context(), domain.USD, domain.EUR, at)
	if err != nil {
		t.Fatalf("fetch rate: %v", err)
	}
	if rate != 0.90 {
		t.Errorf("rate: %v", rate)
	}
}

func TestFrankfurter_FetchRate_SameCurrencyNoCall(t *testing.T) {
	f := &FrankfurterProvider{BaseURL: "http://unreachable.example", HTTP: http.DefaultClient}
	rate, err := f.FetchRate(t.Context(), domain.USD, domain.USD, time.Time{})
	if err != nil {
		t.Fatalf("fetch rate: %v", err)
	}
	if rate != 1.0 {
		t.Errorf("rate: %v", rate)
	}
}
