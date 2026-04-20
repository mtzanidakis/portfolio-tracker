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

// FetchRate returns the rate "1 from = X to" at the given date. An
// at-time of zero (or today / future) falls back to /latest. Frankfurter
// maps weekend/holiday requests to the nearest preceding business day
// internally, so callers don't need to do that.
func (f *FrankfurterProvider) FetchRate(ctx context.Context, from, to domain.Currency, at time.Time) (float64, error) {
	if from == to {
		return 1.0, nil
	}
	if !from.Valid() || !to.Valid() {
		return 0, fmt.Errorf("invalid currency: from=%q to=%q", from, to)
	}

	path := "/latest"
	// Use the dated endpoint only for past dates; Frankfurter has no
	// data for the current business day until late in the session, and
	// returns 404 for future dates.
	if !at.IsZero() && at.Before(time.Now().UTC().Truncate(24*time.Hour)) {
		path = "/" + at.Format("2006-01-02")
	}

	u := fmt.Sprintf("%s%s?from=%s&to=%s", f.BaseURL, path, from, to)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := f.HTTP.Do(req)
	if err != nil {
		return 0, fmt.Errorf("frankfurter: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return 0, fmt.Errorf("frankfurter: status %d: %s", resp.StatusCode, body)
	}

	var parsed frankfurterLatest
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return 0, fmt.Errorf("frankfurter decode: %w", err)
	}
	rate, ok := parsed.Rates[string(to)]
	if !ok || rate == 0 {
		return 0, fmt.Errorf("no %s→%s rate in response", from, to)
	}
	return rate, nil
}
