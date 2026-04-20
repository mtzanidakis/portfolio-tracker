package prices

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// DefaultFrankfurterBaseURL points at the current v2 API (ECB rates).
// The older api.frankfurter.app v1 endpoint can lag by up to a business
// day in some regions; v2 carries the freshest data.
const DefaultFrankfurterBaseURL = "https://api.frankfurter.dev/v2"

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

// v2 returns an array of {date, base, quote, rate} objects.
type frankfurterV2Rate struct {
	Date  string  `json:"date"`
	Base  string  `json:"base"`
	Quote string  `json:"quote"`
	Rate  float64 `json:"rate"`
}

// Fetch queries /rates?base=USD&quotes=<list> and returns "1 c = X USD"
// for each requested currency. USD is always present with rate 1.0.
func (f *FrankfurterProvider) Fetch(ctx context.Context, currencies []domain.Currency) (map[domain.Currency]float64, error) {
	quotes := make([]string, 0, len(currencies))
	for _, c := range currencies {
		if c == domain.USD {
			continue
		}
		if !slices.Contains(quotes, string(c)) {
			quotes = append(quotes, string(c))
		}
	}
	out := map[domain.Currency]float64{domain.USD: 1.0}
	if len(quotes) == 0 {
		return out, nil
	}

	q := url.Values{
		"base":   {"USD"},
		"quotes": {strings.Join(quotes, ",")},
	}
	parsed, err := f.getRates(ctx, q)
	if err != nil {
		return nil, err
	}

	// rate[USD→c] is "1 USD = rate c"; we want "1 c = X USD", which is 1/rate.
	for _, r := range parsed {
		cur := domain.Currency(r.Quote)
		if r.Rate == 0 {
			continue
		}
		out[cur] = 1.0 / r.Rate
	}
	return out, nil
}

// FetchRate returns the rate "1 from = X to" at the given date. A zero
// at (or today/future) maps to the latest rate; past dates use the
// historical endpoint. Frankfurter maps weekend/holiday requests to the
// nearest preceding business day internally.
func (f *FrankfurterProvider) FetchRate(ctx context.Context, from, to domain.Currency, at time.Time) (float64, error) {
	if from == to {
		return 1.0, nil
	}
	if !from.Valid() || !to.Valid() {
		return 0, fmt.Errorf("invalid currency: from=%q to=%q", from, to)
	}

	q := url.Values{
		"base":   {string(from)},
		"quotes": {string(to)},
	}
	// Only past dates use the dated endpoint; today/future hits the
	// default (latest) response.
	if !at.IsZero() && at.Before(time.Now().UTC().Truncate(24*time.Hour)) {
		q.Set("date", at.Format("2006-01-02"))
	}

	parsed, err := f.getRates(ctx, q)
	if err != nil {
		return 0, err
	}
	for _, r := range parsed {
		if domain.Currency(r.Quote) == to && r.Rate > 0 {
			return r.Rate, nil
		}
	}
	return 0, fmt.Errorf("no %s→%s rate in response", from, to)
}

func (f *FrankfurterProvider) getRates(ctx context.Context, q url.Values) ([]frankfurterV2Rate, error) {
	u := f.BaseURL + "/rates?" + q.Encode()
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
	var parsed []frankfurterV2Rate
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("frankfurter decode: %w", err)
	}
	return parsed, nil
}
