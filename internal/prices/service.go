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
	if err := s.refreshPrices(ctx); err != nil {
		s.Logger.Warn("price refresh failed", "err", err)
	}
	if err := s.refreshPriceHistory(ctx); err != nil {
		s.Logger.Warn("price history refresh failed", "err", err)
	}
	if err := s.refreshFx(ctx); err != nil {
		s.Logger.Warn("fx refresh failed", "err", err)
	}
	if err := s.refreshFxHistory(ctx); err != nil {
		s.Logger.Warn("fx history refresh failed", "err", err)
	}
	return nil
}

// refreshPriceHistory fills price_snapshots with daily closes for the
// last year (provider-dependent). Called after refreshPrices so the
// DB always has a latest price even if history fetching fails.
func (s *Service) refreshPriceHistory(ctx context.Context) error {
	assets, err := s.DB.ListAssets(ctx)
	if err != nil {
		return err
	}
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
		snapshots, err := provider.FetchHistory(ctx, id)
		if err != nil {
			s.Logger.Warn("history fetch failed", "symbol", a.Symbol, "err", err)
			continue
		}
		for _, snap := range snapshots {
			if err := s.DB.InsertPriceSnapshot(ctx, db.PriceSnapshot{
				Symbol: a.Symbol, At: snap.At, Price: snap.Price,
			}); err != nil {
				s.Logger.Warn("persist snapshot failed",
					"symbol", a.Symbol, "at", snap.At, "err", err)
			}
		}
	}
	return nil
}

// refreshFxHistory pulls a year of daily rates for every non-USD
// supported currency and upserts them into fx_rates.
func (s *Service) refreshFxHistory(ctx context.Context) error {
	provider, ok := s.Fx.(FxRangeProvider)
	if !ok {
		return nil
	}
	to := time.Now().UTC()
	from := to.AddDate(-1, 0, 0)
	rates, err := provider.FetchRange(ctx, domain.AllCurrencies, from, to)
	if err != nil {
		return err
	}
	for _, r := range rates {
		if err := s.DB.InsertFxRate(ctx, db.FxRate{
			Currency: r.Currency, At: r.At, USDRate: r.USDRate,
		}); err != nil {
			s.Logger.Warn("persist fx history failed",
				"currency", r.Currency, "at", r.At, "err", err)
		}
	}
	return nil
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
	for c, r := range rates {
		if err := s.DB.SetLatestFxRate(ctx, db.LatestFxRate{
			Currency: c, USDRate: r, FetchedAt: now,
		}); err != nil {
			s.Logger.Warn("persist fx failed", "currency", c, "err", err)
		}
	}
	return nil
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
