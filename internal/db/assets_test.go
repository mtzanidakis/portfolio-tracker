package db

import (
	"errors"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestAssetUpsertAndGet(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	a := &domain.Asset{
		Symbol: "AAPL", Name: "Apple", Type: domain.AssetStock,
		Currency: domain.USD,
		Provider: "yahoo", ProviderID: "AAPL",
	}
	if err := db.UpsertAsset(ctx, a); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := db.GetAsset(ctx, "AAPL")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Name != "Apple" || got.Currency != domain.USD {
		t.Errorf("mismatch: %+v", got)
	}

	// Upsert again with new name.
	a.Name = "Apple Inc."
	if err := db.UpsertAsset(ctx, a); err != nil {
		t.Fatalf("upsert 2: %v", err)
	}
	got, _ = db.GetAsset(ctx, "AAPL")
	if got.Name != "Apple Inc." {
		t.Errorf("name not updated: %q", got.Name)
	}
}

func TestListAssets(t *testing.T) {
	db := newTestDB(t)
	mustCreateAsset(t, db, "BTC", domain.AssetCrypto, domain.USD)
	mustCreateAsset(t, db, "AAPL", domain.AssetStock, domain.USD)

	list, err := db.ListAssets(t.Context())
	if err != nil || len(list) != 2 {
		t.Fatalf("list: %d, err=%v", len(list), err)
	}
	if list[0].Symbol != "AAPL" {
		t.Errorf("expected alphabetical order, got %q first", list[0].Symbol)
	}
}

func TestDeleteAsset_NotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.DeleteAsset(t.Context(), "NOPE"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpsertAsset_InvalidType(t *testing.T) {
	db := newTestDB(t)
	a := &domain.Asset{Symbol: "X", Name: "X", Type: "bond", Currency: domain.USD}
	if err := db.UpsertAsset(t.Context(), a); err == nil {
		t.Fatal("expected error for invalid type")
	}
}

// TestDeleteAsset_CascadesPriceChildren ensures the explicit cascade
// in DeleteAsset wipes price_snapshots and prices_latest before
// dropping the asset row. Migration 004 left the FKs without ON
// DELETE CASCADE, so this used to surface as a 787 FK constraint
// failure when trying to delete any asset that had been priced.
func TestDeleteAsset_CascadesPriceChildren(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	mustCreateAsset(t, db, "AAPL", domain.AssetStock, domain.USD)

	at := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	if err := db.InsertPriceSnapshot(ctx, PriceSnapshot{
		Symbol: "AAPL", At: at, Price: 198.20,
	}); err != nil {
		t.Fatalf("insert snapshot: %v", err)
	}
	if err := db.SetLatestPrice(ctx, LatestPrice{
		Symbol: "AAPL", Price: 199, FetchedAt: at,
	}); err != nil {
		t.Fatalf("set latest: %v", err)
	}

	if err := db.DeleteAsset(ctx, "AAPL"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Asset gone.
	if _, err := db.GetAsset(ctx, "AAPL"); !errors.Is(err, ErrNotFound) {
		t.Errorf("asset still present: err=%v", err)
	}
	// Price children gone.
	snaps, _ := db.ListPriceSnapshots(ctx, "AAPL", at.Add(-time.Hour), at.Add(time.Hour))
	if len(snaps) != 0 {
		t.Errorf("snapshot remains: %+v", snaps)
	}
	if _, err := db.GetLatestPrice(ctx, "AAPL"); !errors.Is(err, ErrNotFound) {
		t.Errorf("latest price remains: err=%v", err)
	}
}
