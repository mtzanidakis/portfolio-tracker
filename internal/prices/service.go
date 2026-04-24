package prices

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// Service orchestrates price and FX refreshes, persisting the latest
// values into the database for handlers to consume.
type Service struct {
	DB        *db.DB
	Yahoo     PriceProvider
	CoinGecko PriceProvider
	Fx        FxProvider
	Logger    *slog.Logger
}

// New constructs a Service with the default providers wired in. Pass an
// empty apiKey to use CoinGecko's free public tier.
func New(d *db.DB, logger *slog.Logger, coinGeckoAPIKey string) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		DB:        d,
		Yahoo:     NewYahoo(nil),
		CoinGecko: NewCoinGecko(nil, coinGeckoAPIKey),
		Fx:        NewFrankfurter(nil),
		Logger:    logger,
	}
}

// RefreshAll fetches latest prices for every non-cash asset grouped by
// provider, plus FX rates for all supported currencies, and writes them
// to the database. It also backfills a year of daily history for each
// provider (idempotent via upserts) so the performance chart has
// something to plot. Per-provider failures are logged but do not abort.
func (s *Service) RefreshAll(ctx context.Context) error {
	// History first, then latest. The two sources agree on old data but
	// can disagree on today's value (Yahoo chart's intraday close /
	// Frankfurter timeseries vs. the live quote). We want the live quote
	// to win so the performance chart's final point matches the hero.
	if err := s.refreshPriceHistory(ctx); err != nil {
		s.Logger.Warn("price history refresh failed", "err", err)
	}
	if err := s.refreshPrices(ctx); err != nil {
		s.Logger.Warn("price refresh failed", "err", err)
	}
	if err := s.refreshFxHistory(ctx); err != nil {
		s.Logger.Warn("fx history refresh failed", "err", err)
	}
	if err := s.refreshFx(ctx); err != nil {
		s.Logger.Warn("fx refresh failed", "err", err)
	}
	return nil
}

// refreshPriceHistory fills price_snapshots with daily closes going
// back to each asset's earliest transaction (at least 1 year). Called
// after refreshPrices so the DB always has a latest price even if
// history fetching fails.
func (s *Service) refreshPriceHistory(ctx context.Context) error {
	assets, err := s.DB.ListAssets(ctx)
	if err != nil {
		return err
	}
	// Bookend the whole pass with a single start/end log so operators
	// can correlate "the chart updated" with a refresh cycle. Per-asset
	// fetches log only on completion (or warn on failure) to keep the
	// volume reasonable.
	started := time.Now()
	s.Logger.Info("price history backfill starting", "asset_count", len(assets))

	var fetched, persisted int
	for _, a := range assets {
		if a.Type == domain.AssetCash || a.Provider == "" {
			continue
		}
		var provider HistoryProvider
		switch a.Provider {
		case "yahoo":
			if p, ok := s.Yahoo.(HistoryProvider); ok {
				provider = p
			}
		case "coingecko":
			if p, ok := s.CoinGecko.(HistoryProvider); ok {
				provider = p
			}
		}
		if provider == nil {
			continue
		}
		id := a.ProviderID
		if id == "" {
			id = a.Symbol
		}
		from := historyFrom(ctx, s.DB, a.Symbol)
		snapshots, err := provider.FetchHistory(ctx, id, from)
		if err != nil {
			s.Logger.Warn("history fetch failed",
				"symbol", a.Symbol, "from", from.Format("2006-01-02"), "err", err)
			continue
		}
		fetched++
		for _, snap := range snapshots {
			if err := s.DB.InsertPriceSnapshot(ctx, db.PriceSnapshot{
				Symbol: a.Symbol, At: snap.At, Price: snap.Price,
			}); err != nil {
				s.Logger.Warn("persist snapshot failed",
					"symbol", a.Symbol, "at", snap.At, "err", err)
				continue
			}
			persisted++
		}
		s.Logger.Info("history fetched",
			"symbol", a.Symbol,
			"from", from.Format("2006-01-02"),
			"snapshots", len(snapshots))
	}

	s.Logger.Info("price history backfill complete",
		"assets_fetched", fetched,
		"snapshots_written", persisted,
		"elapsed_ms", time.Since(started).Milliseconds())
	return nil
}

// refreshFxHistory pulls daily rates for every non-USD supported
// currency, covering back to the oldest transaction in the system
// (floored at 1 year). Idempotent via InsertFxRate's upsert.
func (s *Service) refreshFxHistory(ctx context.Context) error {
	provider, ok := s.Fx.(FxRangeProvider)
	if !ok {
		return nil
	}
	to := time.Now().UTC()
	from := fxBackfillFrom(ctx, s.DB, to)
	started := time.Now()
	s.Logger.Info("fx history backfill starting",
		"from", from.Format("2006-01-02"),
		"to", to.Format("2006-01-02"))

	rates, err := provider.FetchRange(ctx, domain.AllCurrencies, from, to)
	if err != nil {
		s.Logger.Warn("fx history fetch failed", "err", err)
		return err
	}
	var persisted int
	for _, r := range rates {
		if err := s.DB.InsertFxRate(ctx, db.FxRate{
			Currency: r.Currency, At: r.At, USDRate: r.USDRate,
		}); err != nil {
			s.Logger.Warn("persist fx history failed",
				"currency", r.Currency, "at", r.At, "err", err)
			continue
		}
		persisted++
	}
	s.Logger.Info("fx history backfill complete",
		"rates_written", persisted,
		"elapsed_ms", time.Since(started).Milliseconds())
	return nil
}

// historyFrom returns the earliest tx date for a given asset, floored
// so we always fetch at least one year of history (keeps the chart
// detailed for freshly-added assets with only a few recent buys).
// On any lookup error we fall back to the 1-year default.
func historyFrom(ctx context.Context, d *db.DB, symbol string) time.Time {
	now := time.Now().UTC()
	floor := now.AddDate(-1, 0, 0)
	earliest, err := d.EarliestTxDateForSymbol(ctx, symbol)
	if err != nil {
		return floor
	}
	if earliest.After(floor) {
		return floor
	}
	return earliest
}

// fxBackfillFrom picks the start date for the FX history backfill:
// the earliest transaction across every user (since FX quotes are
// shared globally), floored at one year so we always have a useful
// recent window even on empty databases.
func fxBackfillFrom(ctx context.Context, d *db.DB, now time.Time) time.Time {
	floor := now.AddDate(-1, 0, 0)
	earliest, err := d.EarliestTxDate(ctx)
	if err != nil {
		return floor
	}
	if earliest.After(floor) {
		return floor
	}
	return earliest
}

type assetRef struct {
	symbol     string
	providerID string
}

func (s *Service) refreshPrices(ctx context.Context) error {
	assets, err := s.DB.ListAssets(ctx)
	if err != nil {
		return err
	}
	byProvider := map[string][]assetRef{}
	for _, a := range assets {
		if a.Type == domain.AssetCash || a.Provider == "" {
			continue
		}
		id := a.ProviderID
		if id == "" {
			id = a.Symbol
		}
		byProvider[a.Provider] = append(byProvider[a.Provider], assetRef{
			symbol: a.Symbol, providerID: id,
		})
	}

	for name, refs := range byProvider {
		prov := s.providerByName(name)
		if prov == nil {
			s.Logger.Warn("unknown provider", "name", name)
			continue
		}
		ids := make([]string, len(refs))
		for i, r := range refs {
			ids[i] = r.providerID
		}
		quotes, err := prov.Fetch(ctx, ids)
		if err != nil {
			s.Logger.Warn("provider fetch failed", "name", name, "err", err)
			continue
		}
		byID := make(map[string]PriceQuote, len(quotes))
		for _, q := range quotes {
			byID[q.Symbol] = q
		}
		for _, r := range refs {
			q, ok := byID[r.providerID]
			if !ok {
				continue
			}
			if err := s.DB.SetLatestPrice(ctx, db.LatestPrice{
				Symbol: r.symbol, Price: q.Price, FetchedAt: q.FetchedAt,
			}); err != nil {
				s.Logger.Warn("persist price failed", "symbol", r.symbol, "err", err)
			}
			// Mirror the live quote into the daily snapshot table so the
			// performance chart's last point reflects today's price rather
			// than yesterday's close (Yahoo's chart endpoint lags by ≥1d).
			today := truncateDay(q.FetchedAt)
			if err := s.DB.InsertPriceSnapshot(ctx, db.PriceSnapshot{
				Symbol: r.symbol, At: today, Price: q.Price,
			}); err != nil {
				s.Logger.Warn("persist snapshot failed", "symbol", r.symbol, "err", err)
			}
		}
	}
	return nil
}

func (s *Service) refreshFx(ctx context.Context) error {
	rates, err := s.Fx.Fetch(ctx, domain.AllCurrencies)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	today := truncateDay(now)
	for c, r := range rates {
		if err := s.DB.SetLatestFxRate(ctx, db.LatestFxRate{
			Currency: c, USDRate: r, FetchedAt: now,
		}); err != nil {
			s.Logger.Warn("persist fx failed", "currency", c, "err", err)
		}
		// Same motivation as refreshPrices: keep the per-day FX table in
		// sync with latest so valuations on `today` use the same rate in
		// both the hero (latest) and the chart (snapshots).
		if c == domain.USD {
			continue
		}
		if err := s.DB.InsertFxRate(ctx, db.FxRate{
			Currency: c, At: today, USDRate: r,
		}); err != nil {
			s.Logger.Warn("persist fx snapshot failed", "currency", c, "err", err)
		}
	}
	return nil
}

// truncateDay returns midnight UTC for the given instant. Mirrors
// portfolio.truncateDay but kept private so prices has no dep on
// portfolio.
func truncateDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

func (s *Service) providerByName(name string) PriceProvider {
	switch name {
	case "yahoo":
		return s.Yahoo
	case "coingecko":
		return s.CoinGecko
	default:
		return nil
	}
}

// Run blocks until ctx is canceled, calling RefreshAll at start and then
// every interval thereafter. Intended to be launched as a goroutine by
// the server's main.
func (s *Service) Run(ctx context.Context, interval time.Duration) {
	_ = s.RefreshAll(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if !errors.Is(ctx.Err(), context.Canceled) {
				s.Logger.Info("price service stopping", "reason", ctx.Err())
			}
			return
		case <-ticker.C:
			_ = s.RefreshAll(ctx)
		}
	}
}
