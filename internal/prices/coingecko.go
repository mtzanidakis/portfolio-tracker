package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
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
// native currency). `days` is derived from `from`: the CoinGecko
// endpoint auto-selects daily granularity when days > 90, hourly/5-min
// for shorter ranges. Zero from defaults to 365 days. Callers hand in a
// coin id (not a ticker); see LookupSymbol for the ticker-to-id mapping.
func (c *CoinGeckoProvider) FetchHistory(ctx context.Context, ref AssetFetchRef, from time.Time) ([]HistoricalSnapshot, error) {
	vs := strings.ToLower(string(ref.Currency))
	u := c.BaseURL + "/coins/" + url.PathEscape(ref.ID) + "/market_chart?" + url.Values{
		"vs_currency": {vs},
		"days":        {coingeckoDaysFor(from)},
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

// coingeckoDaysFor converts a "from" timestamp into the "days" query
// parameter accepted by /coins/{id}/market_chart. The endpoint auto-
// selects daily granularity when days > 90, so any value above that
// is fine; we clamp sub-year requests up to 365 to match the previous
// default. A zero from also returns 365.
func coingeckoDaysFor(from time.Time) string {
	if from.IsZero() {
		return "365"
	}
	age := time.Since(from)
	days := max(int(math.Ceil(age.Hours()/24)), 365)
	return strconv.Itoa(days)
}
