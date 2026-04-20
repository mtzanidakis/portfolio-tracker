package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestFxHistorical(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	rates := []FxRate{
		{Currency: domain.EUR, At: mustTime(t, "2026-01-01T00:00:00Z"), USDRate: 1.08},
		{Currency: domain.EUR, At: mustTime(t, "2026-02-01T00:00:00Z"), USDRate: 1.09},
		{Currency: domain.EUR, At: mustTime(t, "2026-03-01T00:00:00Z"), USDRate: 1.10},
	}
	for _, r := range rates {
		if err := db.InsertFxRate(ctx, r); err != nil {
			t.Fatalf("insert: %v", err)
		}
	}

	// Exact date hit.
	got, err := db.GetFxRateAt(ctx, domain.EUR, mustTime(t, "2026-02-01T00:00:00Z"))
	if err != nil || got.USDRate != 1.09 {
		t.Errorf("exact: %+v, err=%v", got, err)
	}

	// Between dates → earliest on-or-before.
	got, err = db.GetFxRateAt(ctx, domain.EUR, mustTime(t, "2026-02-15T00:00:00Z"))
	if err != nil || got.USDRate != 1.09 {
		t.Errorf("between: %+v, err=%v", got, err)
	}

	// Before any rate.
	_, err = db.GetFxRateAt(ctx, domain.EUR, mustTime(t, "2025-12-01T00:00:00Z"))
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestFxLatest(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	if _, err := db.GetLatestFxRate(ctx, domain.EUR); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}

	r := LatestFxRate{
		Currency:  domain.EUR,
		USDRate:   1.095,
		FetchedAt: mustTime(t, "2026-04-20T12:00:00Z"),
	}
	if err := db.SetLatestFxRate(ctx, r); err != nil {
		t.Fatalf("set: %v", err)
	}

	got, err := db.GetLatestFxRate(ctx, domain.EUR)
	if err != nil || got.USDRate != 1.095 {
		t.Errorf("unexpected: %+v, err=%v", got, err)
	}
}

func TestInsertFxRate_InvalidCurrency(t *testing.T) {
	db := newTestDB(t)
	err := db.InsertFxRate(t.Context(), FxRate{Currency: "XYZ", USDRate: 1})
	if err == nil {
		t.Fatal("expected error for invalid currency")
	}
}
