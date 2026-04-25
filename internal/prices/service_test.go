package prices

import (
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

type fakePriceProvider struct {
	name   string
	quotes []PriceQuote
	err    error
	seen   []string
}

func (f *fakePriceProvider) Name() string { return f.name }

func (f *fakePriceProvider) Fetch(_ context.Context, ids []string) ([]PriceQuote, error) {
	f.seen = append(f.seen, ids...)
	return f.quotes, f.err
}

type fakeFxProvider struct {
	rates map[domain.Currency]float64
	err   error
}

func (f *fakeFxProvider) Name() string { return "fake-fx" }

func (f *fakeFxProvider) Fetch(_ context.Context, _ []domain.Currency) (map[domain.Currency]float64, error) {
	return f.rates, f.err
}

func newServiceTestDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "svc.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(t.Context()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func TestService_RefreshAll_Persists(t *testing.T) {
	d := newServiceTestDB(t)
	ctx := t.Context()

	// Seed assets.
	_ = d.UpsertAsset(ctx, &domain.Asset{
		Symbol: "AAPL", Name: "Apple", Type: domain.AssetStock,
		Currency: domain.USD, Provider: "yahoo", ProviderID: "AAPL",
	})
	_ = d.UpsertAsset(ctx, &domain.Asset{
		Symbol: "BTC", Name: "Bitcoin", Type: domain.AssetCrypto,
		Currency: domain.USD, Provider: "coingecko", ProviderID: "bitcoin",
	})
	// A cash asset — should be ignored by refresher.
	_ = d.UpsertAsset(ctx, &domain.Asset{
		Symbol: "USD", Name: "US Dollar", Type: domain.AssetCash,
		Currency: domain.USD,
	})

	now := time.Now().UTC()
	yahoo := &fakePriceProvider{
		name:   "yahoo",
		quotes: []PriceQuote{{Symbol: "AAPL", Price: 198.5, Currency: domain.USD, FetchedAt: now}},
	}
	cg := &fakePriceProvider{
		name:   "coingecko",
		quotes: []PriceQuote{{Symbol: "bitcoin", Price: 67200, Currency: domain.USD, FetchedAt: now}},
	}
	fx := &fakeFxProvider{rates: map[domain.Currency]float64{
		domain.USD: 1.0,
		domain.EUR: 1.10,
		domain.GBP: 1.27,
	}}
	s := &Service{
		DB: d, Yahoo: yahoo, CoinGecko: cg, Fx: fx,
		Logger: slog.Default(),
	}

	if err := s.RefreshAll(ctx); err != nil {
		t.Fatalf("refresh: %v", err)
	}

	aapl, err := d.GetLatestPrice(ctx, "AAPL")
	if err != nil || aapl.Price != 198.5 {
		t.Errorf("AAPL not persisted: %+v, err=%v", aapl, err)
	}
	btc, err := d.GetLatestPrice(ctx, "BTC")
	if err != nil || btc.Price != 67200 {
		t.Errorf("BTC not persisted: %+v, err=%v", btc, err)
	}
	eur, err := d.GetLatestFxRate(ctx, domain.EUR)
	if err != nil || eur.USDRate != 1.10 {
		t.Errorf("EUR not persisted: %+v, err=%v", eur, err)
	}

	if len(yahoo.seen) != 1 || yahoo.seen[0] != "AAPL" {
		t.Errorf("yahoo saw: %v", yahoo.seen)
	}
	if len(cg.seen) != 1 || cg.seen[0] != "bitcoin" {
		t.Errorf("coingecko saw: %v", cg.seen)
	}
}

func TestService_RefreshAll_ProviderErrorIsolated(t *testing.T) {
	d := newServiceTestDB(t)
	ctx := t.Context()
	_ = d.UpsertAsset(ctx, &domain.Asset{
		Symbol: "AAPL", Name: "Apple", Type: domain.AssetStock,
		Currency: domain.USD, Provider: "yahoo", ProviderID: "AAPL",
	})

	yahoo := &fakePriceProvider{name: "yahoo", err: errors.New("boom")}
	fx := &fakeFxProvider{rates: map[domain.Currency]float64{domain.USD: 1.0}}
	s := &Service{DB: d, Yahoo: yahoo, Fx: fx, Logger: slog.Default()}

	// Should not propagate provider error.
	if err := s.RefreshAll(ctx); err != nil {
		t.Errorf("refresh returned error: %v", err)
	}
	// No AAPL price was persisted.
	if _, err := d.GetLatestPrice(ctx, "AAPL"); err == nil {
		t.Error("expected AAPL to be unpersisted")
	}
}

func TestService_Run_ExitsOnCancel(t *testing.T) {
	d := newServiceTestDB(t)
	fx := &fakeFxProvider{rates: map[domain.Currency]float64{domain.USD: 1.0}}
	s := &Service{DB: d, Fx: fx, Logger: slog.Default()}

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})
	go func() {
		s.Run(ctx, 10*time.Millisecond)
		close(done)
	}()
	time.Sleep(30 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// OK
	case <-time.After(time.Second):
		t.Fatal("Run did not exit on cancel")
	}
}

// Tests for the schedule + provider-range helpers. Pure functions, no
// I/O — kept in this file rather than a new one because they share
// package internals.

func TestNextDailyUTC(t *testing.T) {
	cases := []struct {
		name string
		now  time.Time
		hour int
		want time.Time
	}{
		{
			name: "before today's target",
			now:  time.Date(2026, 4, 25, 12, 0, 0, 0, time.UTC),
			hour: 22,
			want: time.Date(2026, 4, 25, 22, 0, 0, 0, time.UTC),
		},
		{
			name: "after today's target rolls to tomorrow",
			now:  time.Date(2026, 4, 25, 23, 0, 0, 0, time.UTC),
			hour: 22,
			want: time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC),
		},
		{
			name: "exactly at target also rolls forward",
			now:  time.Date(2026, 4, 25, 22, 0, 0, 0, time.UTC),
			hour: 22,
			want: time.Date(2026, 4, 26, 22, 0, 0, 0, time.UTC),
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nextDailyUTC(c.now, c.hour, 0)
			if !got.Equal(c.want) {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}
}

func TestYahooRangeFor(t *testing.T) {
	now := time.Now()
	cases := []struct {
		from time.Time
		want string
	}{
		{time.Time{}, "1y"},
		{now.AddDate(0, -6, 0), "1y"},
		{now.AddDate(-1, -6, 0), "2y"},
		{now.AddDate(-3, 0, 0), "5y"},
		{now.AddDate(-7, 0, 0), "10y"},
		{now.AddDate(-15, 0, 0), "max"},
	}
	for _, c := range cases {
		got := yahooRangeFor(c.from)
		if got != c.want {
			t.Errorf("from=%v: got %q, want %q", c.from, got, c.want)
		}
	}
}

func TestCoingeckoDaysFor(t *testing.T) {
	now := time.Now()
	if got := coingeckoDaysFor(time.Time{}); got != "365" {
		t.Errorf("zero from: got %q, want 365", got)
	}
	if got := coingeckoDaysFor(now.AddDate(0, -3, 0)); got != "365" {
		t.Errorf("3-month range: got %q, want 365 (floor)", got)
	}
	got := coingeckoDaysFor(now.AddDate(-2, 0, 0))
	if got == "365" {
		t.Errorf("2-year range: got %q, want >365", got)
	}
}
