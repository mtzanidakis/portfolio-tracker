package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// DefaultYahooBaseURL is Yahoo Finance's unofficial API base. query2
// (over query1) is what the browser-based Finance UI talks to; the
// authenticated quote/chart endpoints live here.
const DefaultYahooBaseURL = "https://query2.finance.yahoo.com"

const (
	yahooCookieURL = "https://fc.yahoo.com/"
	yahooCrumbPath = "/v1/test/getcrumb"
	// Yahoo rejects non-browser User-Agents. The CLI at
	// github.com/mtzanidakis/yfinance-cli uses the same UA with good
	// success; we mirror it to keep our footprint identical.
	yahooUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
)

// YahooProvider fetches quotes from Yahoo Finance. It performs the
// cookie + crumb handshake the site demands for anonymous clients: a
// GET to fc.yahoo.com seeds an A3 cookie, a GET to /v1/test/getcrumb
// returns a crumb string, and every subsequent API call must carry both
// the cookie and the crumb query parameter.
type YahooProvider struct {
	BaseURL   string
	CookieURL string // overridable for tests
	HTTP      *http.Client

	mu    sync.Mutex
	crumb string
}

// yahooPSL tricks cookiejar into sharing cookies across yahoo.com
// subdomains (fc.yahoo.com and query2.finance.yahoo.com). Without it
// the A3 cookie set by fc.yahoo.com is never sent to the query host.
type yahooPSL struct{}

// PublicSuffix returns the single-label TLD of the given domain. This
// naive implementation lets cookiejar treat `yahoo.com` itself as a
// public suffix, which in turn allows `fc.yahoo.com` and
// `query2.finance.yahoo.com` to share cookies.
func (yahooPSL) PublicSuffix(domain string) string {
	if i := strings.LastIndex(domain, "."); i >= 0 {
		return domain[i+1:]
	}
	return domain
}

// String identifies this public-suffix list in diagnostic output.
func (yahooPSL) String() string { return "yahoo-psl" }

// NewYahoo returns a provider pointing at the public Yahoo endpoint.
// If httpClient is nil a default client is created with a cookie jar;
// otherwise the caller's jar is reused (a default jar is installed if
// absent).
func NewYahoo(httpClient *http.Client) *YahooProvider {
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: yahooPSL{}})
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second, Jar: jar}
	} else if httpClient.Jar == nil {
		httpClient.Jar = jar
	}
	return &YahooProvider{
		BaseURL:   DefaultYahooBaseURL,
		CookieURL: yahooCookieURL,
		HTTP:      httpClient,
	}
}

// Name returns "yahoo".
func (y *YahooProvider) Name() string { return "yahoo" }

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Symbol   string `json:"symbol"`
				Currency string `json:"currency"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
			Indicators struct {
				Quote []struct {
					Close []float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
	} `json:"chart"`
}

// FetchHistory pulls daily closes for symbol via the chart endpoint.
// The range parameter is picked as the smallest supported window
// covering `from` (1y / 2y / 5y / 10y / max). A zero `from` defaults
// to 1y. Returns snapshots in the asset's native currency.
func (y *YahooProvider) FetchHistory(ctx context.Context, symbol string, from time.Time) ([]HistoricalSnapshot, error) {
	params := url.Values{
		"range":    {yahooRangeFor(from)},
		"interval": {"1d"},
	}
	body, err := y.authedGet(ctx, "/v8/finance/chart/"+url.PathEscape(symbol), params)
	if err != nil {
		return nil, err
	}
	var parsed yahooChartResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo chart decode: %w", err)
	}
	if len(parsed.Chart.Result) == 0 || len(parsed.Chart.Result[0].Indicators.Quote) == 0 {
		return nil, nil
	}
	r := parsed.Chart.Result[0]
	closes := r.Indicators.Quote[0].Close
	cur := domain.Currency(strings.ToUpper(r.Meta.Currency))
	out := make([]HistoricalSnapshot, 0, len(r.Timestamp))
	for i, ts := range r.Timestamp {
		if i >= len(closes) || closes[i] == 0 {
			continue
		}
		out = append(out, HistoricalSnapshot{
			Symbol:   r.Meta.Symbol,
			At:       time.Unix(ts, 0).UTC(),
			Price:    closes[i],
			Currency: cur,
		})
	}
	return out, nil
}

type yahooQuoteResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol             string  `json:"symbol"`
			ShortName          string  `json:"shortName"`
			LongName           string  `json:"longName"`
			RegularMarketPrice float64 `json:"regularMarketPrice"`
			Currency           string  `json:"currency"`
			QuoteType          string  `json:"quoteType"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"quoteResponse"`
}

// Fetch queries /v7/finance/quote?symbols=... in one request. The
// crumb is added automatically; on 401/403 the crumb is refreshed and
// the request is retried once.
func (y *YahooProvider) Fetch(ctx context.Context, symbols []string) ([]PriceQuote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	params := url.Values{"symbols": {strings.Join(symbols, ",")}}
	body, err := y.authedGet(ctx, "/v7/finance/quote", params)
	if err != nil {
		return nil, err
	}
	var parsed yahooQuoteResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo decode: %w", err)
	}
	now := time.Now().UTC()
	out := make([]PriceQuote, 0, len(parsed.QuoteResponse.Result))
	for _, r := range parsed.QuoteResponse.Result {
		if r.RegularMarketPrice == 0 {
			continue
		}
		out = append(out, PriceQuote{
			Symbol:    r.Symbol,
			Price:     r.RegularMarketPrice,
			Currency:  domain.Currency(strings.ToUpper(r.Currency)),
			FetchedAt: now,
		})
	}
	return out, nil
}

// LookupSymbol resolves a single ticker to its name/currency/type via
// the same /v7/finance/quote endpoint used by Fetch. Returns nil when
// Yahoo has no match (not an error — the caller may try another
// provider). Name prefers longName and falls back to shortName.
//
// For stocks / ETFs the logo URL is built against Parqet's free
// ticker-keyed logo CDN (https://assets.parqet.com/logos/symbol/{sym}).
// No extra API call is needed — the caller will find out at render time
// whether the image actually exists.
func (y *YahooProvider) LookupSymbol(ctx context.Context, symbol string) (*SymbolInfo, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, nil
	}
	params := url.Values{"symbols": {symbol}}
	body, err := y.authedGet(ctx, "/v7/finance/quote", params)
	if err != nil {
		return nil, err
	}
	var parsed yahooQuoteResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("yahoo decode: %w", err)
	}
	if len(parsed.QuoteResponse.Result) == 0 {
		return nil, nil
	}
	r := parsed.QuoteResponse.Result[0]
	name := r.LongName
	if name == "" {
		name = r.ShortName
	}
	info := &SymbolInfo{
		Symbol:     r.Symbol,
		Name:       name,
		Currency:   domain.Currency(strings.ToUpper(r.Currency)),
		AssetType:  yahooQuoteTypeToAsset(r.QuoteType),
		ProviderID: r.Symbol,
	}
	if info.AssetType == domain.AssetStock || info.AssetType == domain.AssetETF {
		info.LogoURL = parqetLogoURL(r.Symbol)
	}
	return info, nil
}

// parqetLogoURL returns the Parqet CDN URL for a given ticker. Parqet
// responds 404 for unknown symbols, which flows through the proxy and
// becomes an initials fallback in the UI — no harm done.
func parqetLogoURL(symbol string) string {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return ""
	}
	return "https://assets.parqet.com/logos/symbol/" + url.PathEscape(symbol)
}

// yahooRangeFor returns the smallest Yahoo chart range string that
// covers back to `from`. The chart endpoint only accepts a fixed
// set of range aliases (1y / 2y / 5y / 10y / max), so we round up to
// the next available step. A zero `from` means "no preference" and
// uses the previous default of 1y.
func yahooRangeFor(from time.Time) string {
	if from.IsZero() {
		return "1y"
	}
	age := time.Since(from)
	// 365.25 days to absorb leap years without a calendar library.
	years := age.Hours() / (24 * 365.25)
	switch {
	case years <= 1:
		return "1y"
	case years <= 2:
		return "2y"
	case years <= 5:
		return "5y"
	case years <= 10:
		return "10y"
	default:
		return "max"
	}
}

// yahooQuoteTypeToAsset maps Yahoo's quoteType enum to our AssetType.
// Unknown values fall back to stock so the form stays usable.
func yahooQuoteTypeToAsset(qt string) domain.AssetType {
	switch strings.ToUpper(qt) {
	case "ETF":
		return domain.AssetETF
	case "CRYPTOCURRENCY":
		return domain.AssetCrypto
	case "EQUITY", "MUTUALFUND":
		return domain.AssetStock
	default:
		return domain.AssetStock
	}
}

// authedGet performs a crumb-protected GET, refreshing the crumb once
// on 401/403.
func (y *YahooProvider) authedGet(ctx context.Context, path string, params url.Values) ([]byte, error) {
	crumb, err := y.ensureCrumb(ctx)
	if err != nil {
		return nil, err
	}
	body, status, err := y.doGet(ctx, path, params, crumb)
	if err != nil {
		return nil, err
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden {
		y.mu.Lock()
		y.crumb = ""
		y.mu.Unlock()
		crumb, err = y.ensureCrumb(ctx)
		if err != nil {
			return nil, fmt.Errorf("refresh crumb: %w", err)
		}
		body, status, err = y.doGet(ctx, path, params, crumb)
		if err != nil {
			return nil, err
		}
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("yahoo: status %d: %s", status, truncate(body, 200))
	}
	return body, nil
}

func (y *YahooProvider) ensureCrumb(ctx context.Context) (string, error) {
	y.mu.Lock()
	defer y.mu.Unlock()
	if y.crumb != "" {
		return y.crumb, nil
	}
	if err := y.fetchCrumbLocked(ctx); err != nil {
		return "", err
	}
	return y.crumb, nil
}

func (y *YahooProvider) fetchCrumbLocked(ctx context.Context) error {
	// 1. Seed cookies. fc.yahoo.com returns 404 but writes the A3 cookie.
	cReq, err := http.NewRequestWithContext(ctx, http.MethodGet, y.CookieURL, nil)
	if err != nil {
		return err
	}
	cReq.Header.Set("User-Agent", yahooUA)
	cResp, err := y.HTTP.Do(cReq)
	if err != nil {
		return fmt.Errorf("yahoo cookie: %w", err)
	}
	_ = cResp.Body.Close()

	// 2. Fetch the crumb using the cookie set in step 1.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, y.BaseURL+yahooCrumbPath, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", yahooUA)
	resp, err := y.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("yahoo crumb: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("yahoo crumb: status %d: %s", resp.StatusCode, body)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("yahoo crumb read: %w", err)
	}
	y.crumb = strings.TrimSpace(string(body))
	return nil
}

func (y *YahooProvider) doGet(ctx context.Context, path string, params url.Values, crumb string) ([]byte, int, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("crumb", crumb)
	u := y.BaseURL + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", yahooUA)
	req.Header.Set("Accept", "application/json")
	resp, err := y.HTTP.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("yahoo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
