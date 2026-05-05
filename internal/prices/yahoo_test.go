package prices

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func aaplRefs(symbols ...string) []AssetFetchRef {
	out := make([]AssetFetchRef, len(symbols))
	for i, s := range symbols {
		out[i] = AssetFetchRef{ID: s, Currency: domain.USD}
	}
	return out
}

// fakeYahoo stitches together the cookie + crumb + quote flow in a
// single test server. It enforces that:
//   - /cookie is hit before /v1/test/getcrumb,
//   - /v1/test/getcrumb is hit before any API request,
//   - every API request carries the crumb.
type fakeYahoo struct {
	crumb string
	hits  struct {
		cookie int64
		crumb  int64
		quote  int64
	}
}

func (f *fakeYahoo) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cookie":
			atomic.AddInt64(&f.hits.cookie, 1)
			http.SetCookie(w, &http.Cookie{Name: "A3", Value: "test-cookie", Path: "/"})
			w.WriteHeader(http.StatusNotFound) // fc.yahoo.com does this
		case "/v1/test/getcrumb":
			atomic.AddInt64(&f.hits.crumb, 1)
			if _, err := r.Cookie("A3"); err != nil {
				http.Error(w, "no cookie", http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(f.crumb))
		case "/v7/finance/quote":
			atomic.AddInt64(&f.hits.quote, 1)
			if r.URL.Query().Get("crumb") != f.crumb {
				http.Error(w, `{"finance":{"error":{"code":"Unauthorized"}}}`, http.StatusUnauthorized)
				return
			}
			if r.URL.Query().Get("symbols") == "" {
				t.Errorf("missing symbols")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"quoteResponse":{"result":[
                {"symbol":"AAPL","regularMarketPrice":198.2,"currency":"USD"},
                {"symbol":"NVDA","regularMarketPrice":462.8,"currency":"USD"}
            ],"error":null}}`))
		default:
			http.NotFound(w, r)
		}
	})
}

func newFakeYahoo(t *testing.T, f *fakeYahoo) (*YahooProvider, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(f.handler(t))
	p := NewYahoo(nil)
	p.BaseURL = srv.URL
	p.CookieURL = srv.URL + "/cookie"
	return p, srv
}

func TestYahoo_Fetch_CookieCrumbFlow(t *testing.T) {
	f := &fakeYahoo{crumb: "abc123"}
	p, srv := newFakeYahoo(t, f)
	defer srv.Close()

	out, err := p.Fetch(t.Context(), aaplRefs("AAPL", "NVDA"))
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected 2, got %d", len(out))
	}

	// Second call reuses the crumb; no new cookie/crumb hits.
	if _, err := p.Fetch(t.Context(), aaplRefs("AAPL")); err != nil {
		t.Fatalf("fetch 2: %v", err)
	}
	if f.hits.cookie != 1 || f.hits.crumb != 1 {
		t.Errorf("expected crumb reuse, hits=%+v", f.hits)
	}
	if f.hits.quote != 2 {
		t.Errorf("quote hits: %d", f.hits.quote)
	}
}

func TestYahoo_Fetch_RetriesOn401(t *testing.T) {
	f := &fakeYahoo{crumb: "valid-crumb"}
	p, srv := newFakeYahoo(t, f)
	defer srv.Close()

	// Simulate a stale cached crumb from a previous session.
	p.crumb = "stale"

	if _, err := p.Fetch(t.Context(), aaplRefs("AAPL")); err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if p.crumb != f.crumb {
		t.Errorf("crumb not refreshed: have %q", p.crumb)
	}
	if f.hits.quote < 2 {
		t.Errorf("expected 2 quote hits (first 401, retry 200); got %d", f.hits.quote)
	}
}

func TestYahoo_EmptySymbols(t *testing.T) {
	y := &YahooProvider{BaseURL: "http://unused", HTTP: http.DefaultClient}
	out, err := y.Fetch(t.Context(), nil)
	if err != nil || out != nil {
		t.Errorf("expected nil,nil; got %v, %v", out, err)
	}
}

func TestYahoo_UserAgentSet(t *testing.T) {
	var seenUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cookie" {
			seenUA = r.Header.Get("User-Agent")
			http.SetCookie(w, &http.Cookie{Name: "A3", Value: "x", Path: "/"})
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	p := NewYahoo(nil)
	p.BaseURL = srv.URL
	p.CookieURL = srv.URL + "/cookie"
	_, _ = p.Fetch(t.Context(), aaplRefs("AAPL")) // will error at crumb step; we only check UA
	if !strings.Contains(seenUA, "Mozilla") {
		t.Errorf("expected browser UA, got %q", seenUA)
	}
}
