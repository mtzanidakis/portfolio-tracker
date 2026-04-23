package portfolio

import (
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestRealizedPnL_None(t *testing.T) {
	// No sells → no realised PnL even with buys.
	got, err := RealizedPnL([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
	})
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if !approx(got, 0) {
		t.Errorf("got %v, want 0", got)
	}
}

func TestRealizedPnL_Loss(t *testing.T) {
	// Buy 16 @ $7.42 (cost 118.72), sell 16 @ $6.68 (proceeds 106.88).
	// Realised = 106.88 − 118.72 = −11.84.
	got, _ := RealizedPnL([]*domain.Transaction{
		tx(1, "ACHR", domain.SideBuy, 16, 7.42, 0, 1.0, "2026-02-09"),
		tx(2, "ACHR", domain.SideSell, 16, 6.68, 0, 1.0, "2026-02-13"),
	})
	if !approx(got, -11.84) {
		t.Errorf("got %v, want -11.84", got)
	}
}

func TestRealizedPnL_Gain(t *testing.T) {
	got, _ := RealizedPnL([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 10, 110, 0, 1.0, "2026-02-01"),
	})
	if !approx(got, 100) {
		t.Errorf("got %v, want 100", got)
	}
}

func TestRealizedPnL_FeesReduceGain(t *testing.T) {
	// Buy fee adds to cost, sell fee reduces proceeds. So a flat
	// round-trip at the same price with both sides charging fee
	// surfaces as a loss equal to (buy_fee + sell_fee).
	got, _ := RealizedPnL([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 1, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 10, 100, 1, 1.0, "2026-02-01"),
	})
	if !approx(got, -2) {
		t.Errorf("got %v, want -2", got)
	}
}

func TestRealizedPnL_PartialSell(t *testing.T) {
	// Buy 10 @ 100, sell 4 @ 120 → realised = 4*(120−100) = 80.
	got, _ := RealizedPnL([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 4, 120, 0, 1.0, "2026-02-01"),
	})
	if !approx(got, 80) {
		t.Errorf("got %v, want 80", got)
	}
}

func TestRealizedPnL_CashOpsNoRealized(t *testing.T) {
	// Deposits, interest and withdrawals on cash assets don't
	// generate realised PnL in this model.
	got, _ := RealizedPnL([]*domain.Transaction{
		tx(1, "CASH-EUR", domain.SideDeposit, 1000, 1, 0, 1.0, "2026-01-01"),
		tx(2, "CASH-EUR", domain.SideInterest, 10, 1, 0, 1.0, "2026-02-01"),
		tx(3, "CASH-EUR", domain.SideWithdraw, 500, 1, 0, 1.0, "2026-03-01"),
	})
	if !approx(got, 0) {
		t.Errorf("got %v, want 0", got)
	}
}

func TestRealizedPnL_BaseCurrencyFx(t *testing.T) {
	// Buy at fx 0.80 (cost in base = 10*100*0.80 = 800), sell at
	// fx 1.20 (proceeds in base = 10*100*1.20 = 1200). Realised = 400.
	got, _ := RealizedPnL([]*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 0.80, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 10, 100, 0, 1.20, "2026-02-01"),
	})
	if !approx(got, 400) {
		t.Errorf("got %v, want 400", got)
	}
}
