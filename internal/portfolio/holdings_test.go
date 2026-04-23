package portfolio

import (
	"math"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func tx(id int64, sym string, side domain.TxSide, qty, price, fee, fx float64, day string) *domain.Transaction {
	t, _ := time.Parse("2006-01-02", day)
	return &domain.Transaction{
		ID: id, AssetSymbol: sym, Side: side,
		Qty: qty, Price: price, Fee: fee, FxToBase: fx,
		OccurredAt: t,
	}
}

func approx(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

func TestHoldings_Empty(t *testing.T) {
	got, err := Holdings(nil)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty, got %v", got)
	}
}

func TestHoldings_SingleBuy(t *testing.T) {
	got, err := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 0.9, "2026-01-01"),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 holding, got %d", len(got))
	}
	h := got[0]
	if h.Symbol != "AAPL" || h.Qty != 10 {
		t.Errorf("wrong symbol/qty: %+v", h)
	}
	if !approx(h.CostNative, 1000) {
		t.Errorf("CostNative: got %v", h.CostNative)
	}
	if !approx(h.CostBase, 900) {
		t.Errorf("CostBase: got %v", h.CostBase)
	}
	if !approx(h.AvgCostNative, 100) {
		t.Errorf("AvgCostNative: got %v", h.AvgCostNative)
	}
	if !approx(h.AvgCostBase, 90) {
		t.Errorf("AvgCostBase: got %v", h.AvgCostBase)
	}
}

func TestHoldings_BuyWithFee(t *testing.T) {
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 5, 1.0, "2026-01-01"),
	})
	h := got[0]
	if !approx(h.CostNative, 1005) {
		t.Errorf("fee should be included in cost: %v", h.CostNative)
	}
	if !approx(h.AvgCostNative, 100.5) {
		t.Errorf("AvgCostNative: %v", h.AvgCostNative)
	}
}

func TestHoldings_AverageCost(t *testing.T) {
	// Buy 10 @ 100, then 10 @ 200 → avg = 150
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideBuy, 10, 200, 0, 1.0, "2026-01-02"),
	})
	h := got[0]
	if h.Qty != 20 {
		t.Errorf("qty: %v", h.Qty)
	}
	if !approx(h.AvgCostNative, 150) {
		t.Errorf("avg should be 150, got %v", h.AvgCostNative)
	}
	if !approx(h.CostNative, 3000) {
		t.Errorf("cost: %v", h.CostNative)
	}
}

func TestHoldings_SellKeepsAvg(t *testing.T) {
	// Buy 10 @ 100 (avg 100), sell 3 → qty=7, avg stays at 100
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 3, 120, 0, 1.0, "2026-01-02"),
	})
	h := got[0]
	if h.Qty != 7 {
		t.Errorf("qty: %v", h.Qty)
	}
	if !approx(h.AvgCostNative, 100) {
		t.Errorf("avg should be unchanged at 100, got %v", h.AvgCostNative)
	}
	if !approx(h.CostNative, 700) {
		t.Errorf("cost: %v", h.CostNative)
	}
}

func TestHoldings_FullyClosed(t *testing.T) {
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 10, 120, 0, 1.0, "2026-01-02"),
	})
	if len(got) != 0 {
		t.Errorf("fully-closed position should be omitted, got %+v", got)
	}
}

func TestHoldings_SellMoreThanOwned(t *testing.T) {
	_, err := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 5, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 10, 120, 0, 1.0, "2026-01-02"),
	})
	if err == nil {
		t.Fatal("expected error for oversell")
	}
}

func TestHoldings_SortsByDate(t *testing.T) {
	// Provide transactions out of order; Holdings must sort them.
	got, err := Holdings([]*domain.Transaction{
		tx(2, "AAPL", domain.SideBuy, 10, 200, 0, 1.0, "2026-01-02"),
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !approx(got[0].AvgCostNative, 150) {
		t.Errorf("chronological processing failed, avg=%v", got[0].AvgCostNative)
	}
}

func TestHoldings_MultipleSymbolsSortedOutput(t *testing.T) {
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "ZZZ", domain.SideBuy, 1, 10, 0, 1.0, "2026-01-01"),
		tx(2, "AAA", domain.SideBuy, 1, 20, 0, 1.0, "2026-01-01"),
		tx(3, "MMM", domain.SideBuy, 1, 30, 0, 1.0, "2026-01-01"),
	})
	if len(got) != 3 {
		t.Fatalf("expected 3, got %d", len(got))
	}
	if got[0].Symbol != "AAA" || got[2].Symbol != "ZZZ" {
		t.Errorf("not sorted alphabetically: %+v", got)
	}
}

func TestHoldings_BaseCostUsesLockedFx(t *testing.T) {
	// Two buys with different fx_to_base; base cost is a weighted mix.
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 0.80, "2026-01-01"),
		tx(2, "AAPL", domain.SideBuy, 10, 100, 0, 1.20, "2026-01-02"),
	})
	h := got[0]
	// CostNative = 2000 (both buys), CostBase = 800 + 1200 = 2000.
	if !approx(h.CostNative, 2000) {
		t.Errorf("native cost: %v", h.CostNative)
	}
	if !approx(h.CostBase, 2000) {
		t.Errorf("base cost: %v", h.CostBase)
	}
	if !approx(h.AvgCostBase, 100) {
		t.Errorf("avg base cost: %v", h.AvgCostBase)
	}
}

func TestHoldings_InvalidSide(t *testing.T) {
	bad := &domain.Transaction{ID: 1, AssetSymbol: "X", Side: "swap", Qty: 1, Price: 1, FxToBase: 1}
	_, err := Holdings([]*domain.Transaction{bad})
	if err == nil {
		t.Fatal("expected error for invalid side")
	}
}

func TestHoldings_CashDepositsAndInterest(t *testing.T) {
	// Deposit 500 at fx 1.0, interest 10 at fx 1.0: qty=510, cost=510.
	// Interest behaves like a deposit: both grow the cost basis.
	got, err := Holdings([]*domain.Transaction{
		tx(1, "CASH-EUR", domain.SideDeposit, 500, 1, 0, 1.0, "2026-01-01"),
		tx(2, "CASH-EUR", domain.SideInterest, 10, 1, 0, 1.0, "2026-02-01"),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(got) != 1 || got[0].Qty != 510 {
		t.Fatalf("wrong qty: %+v", got)
	}
	if !approx(got[0].CostBase, 510) {
		t.Errorf("cost base: got %v, want 510", got[0].CostBase)
	}
}

func TestHoldings_CashWithdraw(t *testing.T) {
	// Deposit 1000, withdraw 300 → qty=700, cost=700 (average-cost).
	got, _ := Holdings([]*domain.Transaction{
		tx(1, "CASH-EUR", domain.SideDeposit, 1000, 1, 0, 1.0, "2026-01-01"),
		tx(2, "CASH-EUR", domain.SideWithdraw, 300, 1, 0, 1.0, "2026-01-10"),
	})
	if got[0].Qty != 700 {
		t.Errorf("qty: %v", got[0].Qty)
	}
	if !approx(got[0].CostBase, 700) {
		t.Errorf("cost: %v", got[0].CostBase)
	}
}

func TestHoldings_CashOverdraft(t *testing.T) {
	_, err := Holdings([]*domain.Transaction{
		tx(1, "CASH-EUR", domain.SideDeposit, 100, 1, 0, 1.0, "2026-01-01"),
		tx(2, "CASH-EUR", domain.SideWithdraw, 500, 1, 0, 1.0, "2026-01-02"),
	})
	if err == nil {
		t.Fatal("expected error for withdraw exceeding balance")
	}
}
