package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestTransactionCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	u := mustCreateUser(t, db, "tx@test.io")
	acc := mustCreateAccount(t, db, u.ID, domain.USD)
	mustCreateAsset(t, db, "AAPL", domain.AssetStock, domain.USD)

	tx := &domain.Transaction{
		UserID:      u.ID,
		AccountID:   acc.ID,
		AssetSymbol: "AAPL",
		Side:        domain.SideBuy,
		Qty:         3, Price: 198.20, Fee: 0.5,
		FxToBase:   0.92,
		OccurredAt: mustTime(t, "2026-04-10T10:00:00Z"),
		Note:       "weekly DCA",
	}
	if err := db.CreateTransaction(ctx, tx); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetTransaction(ctx, tx.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Qty != 3 || got.Side != domain.SideBuy {
		t.Errorf("mismatch: %+v", got)
	}

	got.Note = "updated"
	got.Qty = 4
	if err := db.UpdateTransaction(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	again, _ := db.GetTransaction(ctx, got.ID)
	if again.Note != "updated" || again.Qty != 4 {
		t.Errorf("update not persisted: %+v", again)
	}

	if err := db.DeleteTransaction(ctx, tx.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetTransaction(ctx, tx.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestListTransactions_Filters(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	u := mustCreateUser(t, db, "flt@test.io")
	acc := mustCreateAccount(t, db, u.ID, domain.USD)
	mustCreateAsset(t, db, "AAPL", domain.AssetStock, domain.USD)
	mustCreateAsset(t, db, "BTC", domain.AssetCrypto, domain.USD)

	seed := []*domain.Transaction{
		{
			UserID: u.ID, AccountID: acc.ID, AssetSymbol: "AAPL", Side: domain.SideBuy,
			Qty: 1, Price: 100, FxToBase: 1,
			OccurredAt: mustTime(t, "2026-01-01T00:00:00Z"),
		},
		{
			UserID: u.ID, AccountID: acc.ID, AssetSymbol: "AAPL", Side: domain.SideSell,
			Qty: 1, Price: 110, FxToBase: 1,
			OccurredAt: mustTime(t, "2026-02-01T00:00:00Z"),
		},
		{
			UserID: u.ID, AccountID: acc.ID, AssetSymbol: "BTC", Side: domain.SideBuy,
			Qty: 0.01, Price: 65000, FxToBase: 1,
			OccurredAt: mustTime(t, "2026-03-01T00:00:00Z"),
		},
	}
	for _, tx := range seed {
		if err := db.CreateTransaction(ctx, tx); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	all, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID})
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}
	// newest first
	if all[0].AssetSymbol != "BTC" {
		t.Errorf("expected BTC first, got %s", all[0].AssetSymbol)
	}

	buys, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID, Side: domain.SideBuy})
	if len(buys) != 2 {
		t.Errorf("expected 2 buys, got %d", len(buys))
	}

	aapls, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID, AssetSymbol: "AAPL"})
	if len(aapls) != 2 {
		t.Errorf("expected 2 AAPL tx, got %d", len(aapls))
	}

	q1 := TxFilter{
		UserID: u.ID,
		From:   mustTime(t, "2026-02-15T00:00:00Z"),
	}
	feb, _ := db.ListTransactions(ctx, q1)
	if len(feb) != 1 || feb[0].AssetSymbol != "BTC" {
		t.Errorf("from-filter: %+v", feb)
	}
}
