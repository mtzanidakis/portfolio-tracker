package db

import (
	"errors"
	"testing"

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
