package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// DefaultCoinGeckoBaseURL is the public (no-key) CoinGecko v3 endpoint.
const DefaultCoinGeckoBaseURL = "https://api.coingecko.com/api/v3"

// coingeckoHistoryDays is the furthest back /coins/{id}/market_chart
// reaches on the public and Demo tiers. Asking for even one day more is
// rejected outright (error 10012), so every backfill requests exactly
// this window — older transactions only get the most recent year.
const coingeckoHistoryDays = "365"

// Minimum spacing between paced requests, from CoinGecko's documented
// rate limits: the keyless public API can drop to 5 calls/min under
// load, the Demo tier allows 30 calls/min.
const (
	coingeckoPublicInterval = 12 * time.Second
	coingeckoDemoInterval   = 2 * time.Second
)

// CoinGeckoProvider fetches crypto quotes from CoinGecko. An APIKey is
// optional; when set, the x-cg-demo-api-key header is added for the Demo
// tier's dedicated rate limit.
type CoinGeckoProvider struct {
	BaseURL string
	APIKey  string // optional; enables Demo tier
	HTTP    *http.Client
	// MinInterval spaces out Fetch and FetchHistory requests so the
	// history backfill (one call per crypto asset, back to back) and
	// the live refresh that follows it stay under the rate limit. Zero
	// disables pacing. LookupSymbol is never paced: it serves the
	// interactive add-asset form.
	MinInterval time.Duration

	mu   sync.Mutex
	last time.Time
}

// NewCoinGecko returns a provider pointing at the public endpoint, paced
// for the tier the apiKey selects.
func NewCoinGecko(httpClient *http.Client, apiKey string) *CoinGeckoProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	interval := coingeckoPublicInterval
	if apiKey != "" {
		interval = coingeckoDemoInterval
	}
	return &CoinGeckoProvider{
		BaseURL:     DefaultCoinGeckoBaseURL,
		APIKey:      apiKey,
		HTTP:        httpClient,
		MinInterval: interval,
	}
}

// pace blocks until MinInterval has passed since the previous paced
// request. Holding the mutex while waiting queues concurrent callers
// (the live and history loops) behind each other.
func (c *CoinGeckoProvider) pace(ctx context.Context) error {
	if c.MinInterval <= 0 {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if wait := time.Until(c.last.Add(c.MinInterval)); wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
		}
	}
	c.last = time.Now()
	return nil
}

// Name returns "coingecko".
func (c *CoinGeckoProvider) Name() string { return "coingecko" }

// Fetch queries /simple/price?ids=<ids>&vs_currencies=<cur>. External
// IDs are CoinGecko coin IDs (e.g., "bitcoin", "ethereum"). Refs are
// grouped by currency and one HTTP call is issued per group, so an
// asset configured with native currency EUR is priced directly in EUR
// (no FX hop through USD). The returned PriceQuote.Currency echoes the
// requested currency.
func (c *CoinGeckoProvider) Fetch(ctx context.Context, refs []AssetFetchRef) ([]PriceQuote, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	byCur := map[domain.Currency][]string{}
	for _, r := range refs {
		byCur[r.Currency] = append(byCur[r.Currency], r.ID)
	}
	now := time.Now().UTC()
	out := make([]PriceQuote, 0, len(refs))
	for cur, ids := range byCur {
		vs := strings.ToLower(string(cur))
		u := c.BaseURL + "/simple/price?" + url.Values{
			"ids":           {strings.Join(ids, ",")},
			"vs_currencies": {vs},
		}.Encode()
		parsed, err := c.getSimplePrice(ctx, u)
		if err != nil {
			return nil, err
		}
		// Response shape: {"bitcoin":{"eur":69000.12}, …}
		for id, prices := range parsed {
			p, ok := prices[vs]
			if !ok {
				continue
			}
			out = append(out, PriceQuote{
				Symbol:    id,
				Price:     p,
				Currency:  cur,
				FetchedAt: now,
			})
		}
	}
	return out, nil
}

func (c *CoinGeckoProvider) getSimplePrice(ctx context.Context, u string) (map[string]map[string]float64, error) {
	if err := c.pace(ctx); err != nil {
		return nil, fmt.Errorf("coingecko: %w", err)
	}
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
	var parsed map[string]map[string]float64
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("coingecko decode: %w", err)
	}
	return parsed, nil
}

type coingeckoChart struct {
	Prices [][]float64 `json:"prices"` // pairs of [ms_epoch, price]
}

type coingeckoSearchResponse struct {
	Coins []struct {
		ID            string `json:"id"`
		Name          string `json:"name"`
		Symbol        string `json:"symbol"`
		MarketCapRank int    `json:"market_cap_rank"`
		Large         string `json:"large"`
		Thumb         string `json:"thumb"`
	} `json:"coins"`
}

// LookupSymbol resolves a ticker (e.g. "BTC") to a CoinGecko coin via
// /search. Prefers a symbol-exact match with the best (lowest, non-zero)
// market-cap rank so "BTC" picks Bitcoin rather than a long-tail token.
// Returns nil when nothing matches.
func (c *CoinGeckoProvider) LookupSymbol(ctx context.Context, symbol string) (*SymbolInfo, error) {
	symbol = strings.TrimSpace(symbol)
	if symbol == "" {
		return nil, nil
	}
	u := c.BaseURL + "/search?" + url.Values{"query": {symbol}}.Encode()
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
		return nil, fmt.Errorf("coingecko search: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("coingecko search: status %d: %s", resp.StatusCode, body)
	}
	var parsed coingeckoSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("coingecko search decode: %w", err)
	}
	want := strings.ToLower(symbol)
	var best *SymbolInfo
	bestRank := int(^uint(0) >> 1) // maxInt
	for _, coin := range parsed.Coins {
		if strings.ToLower(coin.Symbol) != want {
			continue
		}
		rank := coin.MarketCapRank
		if rank <= 0 {
			rank = bestRank - 1
		}
		if best == nil || rank < bestRank {
			bestRank = rank
			logo := coin.Large
			if logo == "" {
				logo = coin.Thumb
			}
			best = &SymbolInfo{
				Symbol:     strings.ToUpper(coin.Symbol),
				Name:       coin.Name,
				Currency:   domain.USD,
				AssetType:  domain.AssetCrypto,
				ProviderID: coin.ID,
				LogoURL:    logo,
			}
		}
	}
	return best, nil
}

// FetchHistory pulls daily closes for the given CoinGecko coin id via
// /coins/{id}/market_chart, denominated in ref.Currency (the asset's
// native currency). The requested "from" is ignored: the public and
// Demo tiers cap the endpoint at coingeckoHistoryDays, so that window is
// always fetched. Callers hand in a coin id (not a ticker); see
// LookupSymbol for the ticker-to-id mapping.
func (c *CoinGeckoProvider) FetchHistory(ctx context.Context, ref AssetFetchRef, _ time.Time) ([]HistoricalSnapshot, error) {
	if err := c.pace(ctx); err != nil {
		return nil, fmt.Errorf("coingecko history: %w", err)
	}
	vs := strings.ToLower(string(ref.Currency))
	u := c.BaseURL + "/coins/" + url.PathEscape(ref.ID) + "/market_chart?" + url.Values{
		"vs_currency": {vs},
		"days":        {coingeckoHistoryDays},
		"interval":    {"daily"},
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
		return nil, fmt.Errorf("coingecko history: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("coingecko history: status %d: %s", resp.StatusCode, body)
	}
	var parsed coingeckoChart
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("coingecko history decode: %w", err)
	}
	out := make([]HistoricalSnapshot, 0, len(parsed.Prices))
	for _, pair := range parsed.Prices {
		if len(pair) < 2 || pair[1] == 0 {
			continue
		}
		out = append(out, HistoricalSnapshot{
			Symbol:   ref.ID,
			At:       time.UnixMilli(int64(pair[0])).UTC(),
			Price:    pair[1],
			Currency: ref.Currency,
		})
	}
	return out, nil
}
