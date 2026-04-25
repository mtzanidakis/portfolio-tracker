package prices

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// historyHourUTC is the wall-clock hour (UTC) at which the daily
// history backfill kicks in. 22:00 UTC sits comfortably after every
// major equity market closes (US 20-21:00 UTC) and Yahoo's chart
// endpoint has had time to publish the day's official close.
const historyHourUTC = 22

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

// RefreshAll runs both the history backfill and the live refresh in
// sequence. Used at process startup and by the manual /prices/refresh
// endpoint, where we want a one-shot full sync. The scheduled
// background loops invoke refreshHistory and refreshLive independently
// at their own cadences instead of going through this wrapper.
func (s *Service) RefreshAll(ctx context.Context) error {
	s.refreshHistory(ctx)
	s.refreshLive(ctx)
	return nil
}

// refreshLive fetches the latest quotes for every asset and the latest
// FX rates. Cheap (one HTTP round-trip per provider) — this is the
// loop that runs every liveEvery and updates today's snapshot. The
// snapshot key is (symbol, midnight UTC), so successive calls within
// the same day collapse via UPSERT — by midnight, today's row holds
// the last live quote of the day, which is then locked to the
// official close by the next history pass.
func (s *Service) refreshLive(ctx context.Context) {
	started := time.Now()
	s.Logger.Info("live refresh starting")
	prices, fxs := s.refreshPrices(ctx), s.refreshFx(ctx)
	if prices > 0 || fxs > 0 {
		s.Logger.Info("live refresh complete",
			"prices_written", prices,
			"fx_written", fxs,
			"elapsed_ms", time.Since(started).Milliseconds())
	}
}

// refreshHistory backfills daily price snapshots and FX rates over the
// range covering every user's earliest transaction (floor: 1 year).
// Heavy — runs once at startup and once every 24 hours after that.
func (s *Service) refreshHistory(ctx context.Context) {
	if err := s.refreshPriceHistory(ctx); err != nil {
		s.Logger.Warn("price history refresh failed", "err", err)
	}
	if err := s.refreshFxHistory(ctx); err != nil {
		s.Logger.Warn("fx history refresh failed", "err", err)
	}
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
		// Yahoo timestamps each daily bar at the exchange's open time
		// (e.g. 13:30 UTC for NYSE). We collapse to midnight UTC so
		// the row shares a primary key with the live refresh's
		// snapshot — the history pass effectively locks each past day
		// to the official close. Today's bar is left to the live
		// refresh, which keeps updating until the day ends.
		todayMidnight := truncateDay(time.Now().UTC())
		for _, snap := range snapshots {
			at := truncateDay(snap.At)
			if !at.Before(todayMidnight) {
				continue
			}
			if err := s.DB.InsertPriceSnapshot(ctx, db.PriceSnapshot{
				Symbol: a.Symbol, At: at, Price: snap.Price,
			}); err != nil {
				s.Logger.Warn("persist snapshot failed",
					"symbol", a.Symbol, "at", at, "err", err)
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

// refreshPrices fetches latest quotes per provider and writes both the
// `prices_latest` row (current quote) and the day-bucketed snapshot
// (collapsed via UPSERT to the last live quote of the day). Returns
// the count of snapshot rows successfully written so the caller can
// log totals; provider/persist failures are warned and skipped.
func (s *Service) refreshPrices(ctx context.Context) int {
	assets, err := s.DB.ListAssets(ctx)
	if err != nil {
		s.Logger.Warn("list assets failed", "err", err)
		return 0
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

	var written int
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
			// Mirror the live quote into today's snapshot row. The next
			// daily history pass will overwrite this with the official
			// close from the chart endpoint once the day rolls over.
			today := truncateDay(q.FetchedAt)
			if err := s.DB.InsertPriceSnapshot(ctx, db.PriceSnapshot{
				Symbol: r.symbol, At: today, Price: q.Price,
			}); err != nil {
				s.Logger.Warn("persist snapshot failed", "symbol", r.symbol, "err", err)
				continue
			}
			written++
		}
	}
	return written
}

// refreshFx fetches the latest USD-base FX rates and writes both the
// `fx_latest` row and today's day-bucketed snapshot. Returns the count
// of snapshot rows written.
func (s *Service) refreshFx(ctx context.Context) int {
	rates, err := s.Fx.Fetch(ctx, domain.AllCurrencies)
	if err != nil {
		s.Logger.Warn("fx fetch failed", "err", err)
		return 0
	}
	now := time.Now().UTC()
	today := truncateDay(now)
	var written int
	for c, r := range rates {
		if err := s.DB.SetLatestFxRate(ctx, db.LatestFxRate{
			Currency: c, USDRate: r, FetchedAt: now,
		}); err != nil {
			s.Logger.Warn("persist fx failed", "currency", c, "err", err)
		}
		if c == domain.USD {
			continue
		}
		if err := s.DB.InsertFxRate(ctx, db.FxRate{
			Currency: c, At: today, USDRate: r,
		}); err != nil {
			s.Logger.Warn("persist fx snapshot failed", "currency", c, "err", err)
			continue
		}
		written++
	}
	return written
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

// Run blocks until ctx is canceled, driving two independent refresh
// loops:
//
//   - Live: every liveEvery, talks to the latest-quote endpoints only.
//     Cheap; updates today's snapshot in place.
//   - History: once at startup, then every day at historyHourUTC.
//     Heavier — fetches a year+ of daily bars per asset and the FX
//     range, then persists. Locks past days to the exchange's
//     official close.
//
// Intended to be launched as a goroutine from the server's main.
func (s *Service) Run(ctx context.Context, liveEvery time.Duration) {
	// Bootstrap synchronously so a freshly-booted process serves
	// requests with data already in place, and the operator's logs
	// confirm both passes ran.
	s.refreshHistory(ctx)
	s.refreshLive(ctx)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		s.runLiveLoop(ctx, liveEvery)
	}()
	go func() {
		defer wg.Done()
		s.runHistoryLoop(ctx)
	}()
	wg.Wait()
	if !errors.Is(ctx.Err(), context.Canceled) {
		s.Logger.Info("price service stopping", "reason", ctx.Err())
	}
}

func (s *Service) runLiveLoop(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.refreshLive(ctx)
		}
	}
}

// runHistoryLoop sleeps until the next historyHourUTC wall clock, runs
// the daily backfill, then sleeps another 24h-ish. Recomputes the
// target on every iteration so day rollovers (DST has no effect since
// we anchor in UTC) and missed ticks don't drift the schedule.
func (s *Service) runHistoryLoop(ctx context.Context) {
	for {
		next := nextDailyUTC(time.Now().UTC(), historyHourUTC, 0)
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Until(next)):
			s.refreshHistory(ctx)
		}
	}
}

// nextDailyUTC returns the next time-of-day in UTC at hour:minute that
// is strictly after `now`. If now is exactly at the target, returns
// the same time tomorrow.
func nextDailyUTC(now time.Time, hour, minute int) time.Time {
	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if !target.After(now) {
		target = target.Add(24 * time.Hour)
	}
	return target
}
