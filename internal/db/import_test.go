package db

import (
	"strings"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/importers"
)

// txDate is the single date every test in this file uses for its
// imported transaction. Kept fixed so the FX rate map keys stay
// trivially predictable.
var txDate = time.Date(2026, 4, 1, 15, 0, 0, 0, time.UTC)

// basicPlan returns a minimal ImportPlan: one new account, one new
// asset, one transaction referencing them. Tests start from this and
// mutate what they're exercising.
func basicPlan() *importers.ImportPlan {
	return &importers.ImportPlan{
		Source: "test",
		Accounts: []importers.ImportAccount{{
			SourceID: "a1", Name: "Imported Brokerage",
			Currency: domain.EUR, TxCount: 1, Selected: true,
		}},
		Assets: []importers.ImportAsset{{
			SourceID: "yahoo::AAPL", Symbol: "AAPL",
			Name: "Apple Inc.", Type: domain.AssetStock,
			Currency: domain.USD, Provider: "yahoo", ProviderID: "AAPL",
			LogoURL: "https://example/aapl.png",
			TxCount: 1, Selected: true,
		}},
		Transactions: []importers.ImportTransaction{{
			AccountSourceID: "a1", AssetSourceID: "yahoo::AAPL",
			Side: domain.SideBuy, Qty: 5, Price: 198, Fee: 1,
			Currency:   domain.USD,
			OccurredAt: txDate,
		}},
	}
}

func TestApplyImport_CreateNew(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "imp1@test.io")
	plan := basicPlan()

	rates := map[FxKey]float64{
		{From: domain.USD, Date: "2026-04-01"}: 0.92,
	}
	res, err := db.ApplyImport(ctx, u.ID, domain.EUR, plan, rates)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.AccountsCreated != 1 || res.AssetsCreated != 1 || res.TransactionsCreated != 1 {
		t.Errorf("counts = %+v, want 1/1/1", res)
	}

	// Account was created with the expected currency.
	accs, _ := db.ListAccounts(ctx, u.ID)
	if len(accs) != 1 || accs[0].Currency != domain.EUR || accs[0].Name != "Imported Brokerage" {
		t.Errorf("account = %+v", accs)
	}

	// Asset was created and carries the logo URL through.
	a, err := db.GetAsset(ctx, "AAPL")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if a.LogoURL != "https://example/aapl.png" || a.Type != domain.AssetStock {
		t.Errorf("asset = %+v", a)
	}

	// Transaction was inserted with fx_to_base resolved from the rate map.
	txs, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID})
	if len(txs) != 1 || txs[0].FxToBase != 0.92 {
		t.Errorf("tx = %+v", txs)
	}
}

func TestApplyImport_SameCurrencyAutoFx(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "imp2@test.io")
	plan := basicPlan()
	plan.Transactions[0].Currency = domain.EUR // matches user base
	plan.Assets[0].Currency = domain.EUR

	// Empty rate map — same-currency tx must short-circuit to 1.0.
	res, err := db.ApplyImport(ctx, u.ID, domain.EUR, plan, map[FxKey]float64{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.TransactionsCreated != 1 {
		t.Fatalf("tx not created: %+v", res)
	}
	txs, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID})
	if txs[0].FxToBase != 1.0 {
		t.Errorf("expected fx=1, got %v", txs[0].FxToBase)
	}
}

func TestApplyImport_MissingFxRolledBack(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "imp3@test.io")
	plan := basicPlan()

	// USD tx, EUR base, no rate provided → ApplyImport must error.
	if _, err := db.ApplyImport(ctx, u.ID, domain.EUR, plan, map[FxKey]float64{}); err == nil {
		t.Fatal("expected missing-fx error, got nil")
	}

	// Atomicity: nothing should have been created.
	accs, _ := db.ListAccounts(ctx, u.ID)
	if len(accs) != 0 {
		t.Errorf("expected 0 accounts after rollback, got %d", len(accs))
	}
	if _, err := db.GetAsset(ctx, "AAPL"); err == nil {
		t.Error("asset should not exist after rollback")
	}
	txs, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID})
	if len(txs) != 0 {
		t.Errorf("expected 0 transactions, got %d", len(txs))
	}
}

func TestApplyImport_MapToExistingAccount(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "imp4@test.io")
	existing := mustCreateAccount(t, db, u.ID, domain.EUR)

	plan := basicPlan()
	plan.Accounts[0].MapToID = existing.ID // reuse, don't create

	rates := map[FxKey]float64{{From: domain.USD, Date: "2026-04-01"}: 0.92}
	res, err := db.ApplyImport(ctx, u.ID, domain.EUR, plan, rates)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.AccountsCreated != 0 || res.AccountsReused != 1 {
		t.Errorf("counts = %+v, want 0 created / 1 reused", res)
	}
	accs, _ := db.ListAccounts(ctx, u.ID)
	if len(accs) != 1 {
		t.Errorf("account list = %d, want still 1", len(accs))
	}

	// Tx points at the reused account.
	txs, _ := db.ListTransactions(ctx, TxFilter{UserID: u.ID})
	if txs[0].AccountID != existing.ID {
		t.Errorf("tx account = %d, want %d", txs[0].AccountID, existing.ID)
	}
}

func TestApplyImport_MapToExistingAsset(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "imp5@test.io")
	mustCreateAsset(t, db, "AAPL", domain.AssetStock, domain.USD)

	plan := basicPlan()
	plan.Assets[0].MapToSymbol = "AAPL" // reuse stored row
	plan.Assets[0].LogoURL = "ignored"  // would clobber if we didn't reuse
	plan.Assets[0].Name = "Wrong name"

	rates := map[FxKey]float64{{From: domain.USD, Date: "2026-04-01"}: 0.92}
	res, err := db.ApplyImport(ctx, u.ID, domain.EUR, plan, rates)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.AssetsReused != 1 {
		t.Errorf("expected reused asset, got %+v", res)
	}
	a, _ := db.GetAsset(ctx, "AAPL")
	if a.Name == "Wrong name" || a.LogoURL == "ignored" {
		t.Errorf("reuse path clobbered stored asset: %+v", a)
	}
}

func TestApplyImport_RejectsForeignAccount(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	owner := mustCreateUser(t, db, "owner@test.io")
	ownerAcc := mustCreateAccount(t, db, owner.ID, domain.EUR)
	intruder := mustCreateUser(t, db, "intruder@test.io")

	plan := basicPlan()
	plan.Accounts[0].MapToID = ownerAcc.ID

	rates := map[FxKey]float64{{From: domain.USD, Date: "2026-04-01"}: 0.92}
	_, err := db.ApplyImport(ctx, intruder.ID, domain.EUR, plan, rates)
	if err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("expected ownership error, got %v", err)
	}
}
