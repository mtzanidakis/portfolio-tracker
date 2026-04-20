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

// DefaultCoinGeckoBaseURL is the public (no-key) CoinGecko v3 endpoint.
const DefaultCoinGeckoBaseURL = "https://api.coingecko.com/api/v3"

// CoinGeckoProvider fetches crypto quotes from CoinGecko. An APIKey is
// optional; when set, the x-cg-demo-api-key header is added for the Demo
// tier's dedicated rate limit.
type CoinGeckoProvider struct {
	BaseURL string
	APIKey  string // optional; enables Demo tier
	HTTP    *http.Client
}

// NewCoinGecko returns a provider pointing at the public endpoint.
func NewCoinGecko(httpClient *http.Client, apiKey string) *CoinGeckoProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &CoinGeckoProvider{
		BaseURL: DefaultCoinGeckoBaseURL,
		APIKey:  apiKey,
		HTTP:    httpClient,
	}
}

// Name returns "coingecko".
func (c *CoinGeckoProvider) Name() string { return "coingecko" }

// Fetch queries /simple/price?ids=<ids>&vs_currencies=usd. External IDs are
// CoinGecko coin IDs (e.g., "bitcoin", "ethereum"). Prices are returned in
// USD.
func (c *CoinGeckoProvider) Fetch(ctx context.Context, ids []string) ([]PriceQuote, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	u := c.BaseURL + "/simple/price?" + url.Values{
		"ids":           {strings.Join(ids, ",")},
		"vs_currencies": {"usd"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if c.APIKey != "" {
		req.Header.Set("x-cg-demo-api-key", c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("coingecko: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("coingecko: status %d: %s", resp.StatusCode, body)
	}

	// Response shape: {"bitcoin":{"usd":67200.12},"ethereum":{"usd":3420.4}}
	var parsed map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("coingecko decode: %w", err)
	}
	now := time.Now().UTC()
	out := make([]PriceQuote, 0, len(parsed))
	for id, prices := range parsed {
		p, ok := prices["usd"]
		if !ok {
			continue
		}
		out = append(out, PriceQuote{
			Symbol:    id,
			Price:     p,
			Currency:  domain.USD,
			FetchedAt: now,
		})
	}
	return out, nil
}
