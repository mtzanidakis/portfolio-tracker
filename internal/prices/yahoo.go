package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// DefaultYahooBaseURL is Yahoo Finance's unofficial v7 quote endpoint.
const DefaultYahooBaseURL = "https://query1.finance.yahoo.com"

// YahooProvider fetches quotes from Yahoo Finance. No API key required;
// a custom User-Agent header is mandatory (Yahoo blocks the Go default).
type YahooProvider struct {
	BaseURL string
	HTTP    *http.Client
}

// NewYahoo returns a provider pointing at the public Yahoo endpoint.
func NewYahoo(httpClient *http.Client) *YahooProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &YahooProvider{BaseURL: DefaultYahooBaseURL, HTTP: httpClient}
}

// Name returns "yahoo".
func (y *YahooProvider) Name() string { return "yahoo" }

type yahooResponse struct {
	QuoteResponse struct {
		Result []struct {
			Symbol             string  `json:"symbol"`
			RegularMarketPrice float64 `json:"regularMarketPrice"`
			Currency           string  `json:"currency"`
		} `json:"result"`
		Error any `json:"error"`
	} `json:"quoteResponse"`
}

// Fetch queries Yahoo's /v7/finance/quote endpoint with a comma-separated
// symbol list and returns the latest regular-market price for each.
func (y *YahooProvider) Fetch(ctx context.Context, symbols []string) ([]PriceQuote, error) {
	if len(symbols) == 0 {
		return nil, nil
	}
	u := y.BaseURL + "/v7/finance/quote?symbols=" + url.QueryEscape(strings.Join(symbols, ","))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "portfolio-tracker/1.0")
	req.Header.Set("Accept", "application/json")

	resp, err := y.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("yahoo: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("yahoo: status %d: %s", resp.StatusCode, body)
	}

	var parsed yahooResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("yahoo decode: %w", err)
	}
	now := time.Now().UTC()
	out := make([]PriceQuote, 0, len(parsed.QuoteResponse.Result))
	for _, r := range parsed.QuoteResponse.Result {
		out = append(out, PriceQuote{
			Symbol:    r.Symbol,
			Price:     r.RegularMarketPrice,
			Currency:  domain.Currency(strings.ToUpper(r.Currency)),
			FetchedAt: now,
		})
	}
	return out, nil
}
