package prices

import (
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestFrankfurter_Fetch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("from") != "USD" {
			t.Errorf("from: %q", r.URL.Query().Get("from"))
		}
		_, _ = w.Write([]byte(`{"base":"USD","date":"2026-04-20","rates":{
			"EUR":0.92,"GBP":0.79,"JPY":150.5,"XXX":1.5
		}}`))
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
	// USD is always 1.0.
	if out[domain.USD] != 1.0 {
		t.Errorf("USD: %v", out[domain.USD])
	}
	// XXX was filtered out (not requested).
	if _, ok := out[domain.Currency("XXX")]; ok {
		t.Error("XXX should have been dropped")
	}
}
