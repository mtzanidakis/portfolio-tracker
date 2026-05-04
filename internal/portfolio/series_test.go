package portfolio

import (
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// helper: build a constant priceAt that always returns p for sym, ok=false
// for anything else.
func priceFn(sym string, p float64) PriceAtFn {
	return func(s string, _ time.Time) (float64, bool) {
		if s == sym {
			return p, true
		}
		return 0, false
	}
}

func curFnUSD(string) (domain.Currency, bool) { return domain.USD, true }

// fxAt that always returns 1.0 (USD-denominated, base=USD).
func fxAtIdentity(_ domain.Currency, _ time.Time) (float64, bool) { return 1.0, true }

// TestSeries_TopUpUsesMarketForPreDayQty pins the regression behind the
// "Today" -€1545 bug: a top-up tx on an existing position must not
// collapse the whole holding's value to costBase. The previous-day qty
// is valued at the snapshot price; only today's *added* qty is anchored
// to its tx.Price.
func TestSeries_TopUpUsesMarketForPreDayQty(t *testing.T) {
	// 10 shares bought yesterday at $100 (cost basis $1000).
	// Today the market moved to $120 and we top up with 2 more shares
	// at $121. End-of-day qty = 12; the chart should value the 10
	// pre-day shares at $120 (market) and the 2 added shares at $121
	// (cost), for a total of 1200 + 242 = $1442.
	txs := []*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideBuy, 2, 121, 0, 1.0, "2026-01-02"),
	}
	from, _ := time.Parse("2006-01-02", "2026-01-02")
	to := from
	got := SeriesFromTransactions(txs, from, to,
		priceFn("AAPL", 120), fxAtIdentity, curFnUSD, domain.USD)
	if len(got) != 1 {
		t.Fatalf("expected 1 point, got %d", len(got))
	}
	want := 10*120.0 + 2*121.0 // pre-day at market + today at cost
	if !approx(got[0].Value, want) {
		t.Errorf("Value: got %v, want %v", got[0].Value, want)
	}
	if !approx(got[0].Cost, 1000+242) {
		t.Errorf("Cost: got %v, want %v", got[0].Cost, 1242.0)
	}
}

// TestSeries_FirstBuyAnchoredToCost confirms the day-zero anti-phantom
// behavior is preserved: a brand-new position bought today is valued at
// its tx.Price, not the snapshot, so the chart doesn't show a phantom
// gain/loss between buy time and EOD close.
func TestSeries_FirstBuyAnchoredToCost(t *testing.T) {
	txs := []*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
	}
	from, _ := time.Parse("2006-01-02", "2026-01-01")
	to := from
	got := SeriesFromTransactions(txs, from, to,
		priceFn("AAPL", 105), fxAtIdentity, curFnUSD, domain.USD)
	if len(got) != 1 {
		t.Fatalf("expected 1 point, got %d", len(got))
	}
	// Pre-day qty = 0, today's added qty = 10 at $100 each.
	if !approx(got[0].Value, 1000) {
		t.Errorf("Value: got %v, want 1000", got[0].Value)
	}
}

// TestSeries_NoTxTodayUsesMarket sanity-checks the no-tx path: a static
// holding on a quiet day is valued at qty × snapshot price.
func TestSeries_NoTxTodayUsesMarket(t *testing.T) {
	txs := []*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
	}
	from, _ := time.Parse("2006-01-02", "2026-01-02")
	to := from
	got := SeriesFromTransactions(txs, from, to,
		priceFn("AAPL", 120), fxAtIdentity, curFnUSD, domain.USD)
	if !approx(got[0].Value, 1200) {
		t.Errorf("Value: got %v, want 1200", got[0].Value)
	}
}

// TestSeries_SellTodayDoesNotAnchor confirms that a sell-only day uses
// market price for the surviving qty (sells deploy no new capital, so
// the anti-phantom anchor doesn't apply).
func TestSeries_SellTodayDoesNotAnchor(t *testing.T) {
	txs := []*domain.Transaction{
		tx(1, "AAPL", domain.SideBuy, 10, 100, 0, 1.0, "2026-01-01"),
		tx(2, "AAPL", domain.SideSell, 4, 130, 0, 1.0, "2026-01-02"),
	}
	from, _ := time.Parse("2006-01-02", "2026-01-02")
	to := from
	got := SeriesFromTransactions(txs, from, to,
		priceFn("AAPL", 120), fxAtIdentity, curFnUSD, domain.USD)
	if len(got) != 1 {
		t.Fatalf("expected 1 point, got %d", len(got))
	}
	// 6 surviving shares × $120 snapshot = $720.
	if !approx(got[0].Value, 720) {
		t.Errorf("Value: got %v, want 720", got[0].Value)
	}
}
