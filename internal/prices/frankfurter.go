package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// DefaultFrankfurterBaseURL is the public Frankfurter endpoint (ECB rates).
const DefaultFrankfurterBaseURL = "https://api.frankfurter.app"

// FrankfurterProvider fetches ECB FX rates with USD as the reference base.
type FrankfurterProvider struct {
	BaseURL string
	HTTP    *http.Client
}

// NewFrankfurter returns a provider pointing at the public endpoint.
func NewFrankfurter(httpClient *http.Client) *FrankfurterProvider {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &FrankfurterProvider{BaseURL: DefaultFrankfurterBaseURL, HTTP: httpClient}
}

// Name returns "frankfurter".
func (f *FrankfurterProvider) Name() string { return "frankfurter" }

type frankfurterLatest struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"`
	Rates map[string]float64 `json:"rates"`
}

// Fetch queries /latest?from=USD and returns a map keyed by currency with
// value "1 currency = X USD". Currencies not present in the Frankfurter
// response are silently dropped. USD is always included with rate 1.0.
func (f *FrankfurterProvider) Fetch(ctx context.Context, currencies []domain.Currency) (map[domain.Currency]float64, error) {
	u := f.BaseURL + "/latest?from=USD"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("frankfurter: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("frankfurter: status %d: %s", resp.StatusCode, body)
	}

	var parsed frankfurterLatest
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("frankfurter decode: %w", err)
	}

	// rates[c] is USD→c (1 USD buys rates[c] units of c). We want c→USD,
	// i.e., 1 c = 1/rates[c] USD.
	out := map[domain.Currency]float64{domain.USD: 1.0}
	for code, r := range parsed.Rates {
		cur := domain.Currency(code)
		if !slices.Contains(currencies, cur) {
			continue
		}
		if r == 0 {
			continue
		}
		out[cur] = 1.0 / r
	}
	return out, nil
}
