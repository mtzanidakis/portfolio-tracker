package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestPriceSnapshots(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	mustCreateAsset(t, db, "AAPL", domain.AssetStock, domain.USD)

	d1 := mustTime(t, "2026-04-10T00:00:00Z")
	d2 := mustTime(t, "2026-04-11T00:00:00Z")
	d3 := mustTime(t, "2026-04-12T00:00:00Z")

	snaps := []PriceSnapshot{
		{Symbol: "AAPL", At: d1, Price: 195.0},
		{Symbol: "AAPL", At: d2, Price: 196.5},
		{Symbol: "AAPL", At: d3, Price: 198.2},
	}
	for _, s := range snaps {
		if err := db.InsertPriceSnapshot(ctx, s); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Idempotent upsert with new price for d1.
	if err := db.InsertPriceSnapshot(ctx, PriceSnapshot{Symbol: "AAPL", At: d1, Price: 200.0}); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := db.ListPriceSnapshots(ctx, "AAPL", d1, d3)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(got))
	}
	if got[0].Price != 200.0 {
		t.Errorf("upsert did not replace d1 price: %v", got[0].Price)
	}
}

func TestLatestPrice(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	mustCreateAsset(t, db, "BTC", domain.AssetCrypto, domain.USD)

	if _, err := db.GetLatestPrice(ctx, "BTC"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound before set, got %v", err)
	}

	now := mustTime(t, "2026-04-20T12:00:00Z")
	if err := db.SetLatestPrice(ctx, LatestPrice{Symbol: "BTC", Price: 67200, FetchedAt: now}); err != nil {
		t.Fatalf("set: %v", err)
	}
	p, err := db.GetLatestPrice(ctx, "BTC")
	if err != nil || p.Price != 67200 {
		t.Errorf("unexpected: %+v, err=%v", p, err)
	}

	later := mustTime(t, "2026-04-20T13:00:00Z")
	_ = db.SetLatestPrice(ctx, LatestPrice{Symbol: "BTC", Price: 67500, FetchedAt: later})
	p, _ = db.GetLatestPrice(ctx, "BTC")
	if p.Price != 67500 || !p.FetchedAt.Equal(later) {
		t.Errorf("upsert did not replace: %+v", p)
	}
}
