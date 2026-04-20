package portfolio

import (
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func staticPrice(m map[string]float64) PriceLookup {
	return func(s string) (float64, bool) {
		v, ok := m[s]
		return v, ok
	}
}

func staticFx(m map[domain.Currency]float64) FxLookup {
	return func(c domain.Currency) (float64, bool) {
		v, ok := m[c]
		return v, ok
	}
}

func staticCur(m map[string]domain.Currency) AssetCurrencyLookup {
	return func(s string) (domain.Currency, bool) {
		v, ok := m[s]
		return v, ok
	}
}

func TestValueHoldings_NativeEqBase(t *testing.T) {
	h := []Holding{{
		Symbol: "AAPL", Qty: 10,
		CostNative: 1000, CostBase: 1000,
		AvgCostNative: 100, AvgCostBase: 100,
	}}
	vals := ValueHoldings(h,
		staticPrice(map[string]float64{"AAPL": 110}),
		staticFx(map[domain.Currency]float64{domain.USD: 1.0}),
		staticCur(map[string]domain.Currency{"AAPL": domain.USD}),
		domain.USD,
	)
	if len(vals) != 1 {
		t.Fatalf("expected 1, got %d", len(vals))
	}
	v := vals[0]
	if !approx(v.ValueNative, 1100) || !approx(v.ValueBase, 1100) {
		t.Errorf("value: native=%v base=%v", v.ValueNative, v.ValueBase)
	}
	if !approx(v.PnLNative, 100) || !approx(v.PnLBase, 100) {
		t.Errorf("pnl: native=%v base=%v", v.PnLNative, v.PnLBase)
	}
	if !approx(v.PnLPctBase, 10.0) {
		t.Errorf("pct: %v", v.PnLPctBase)
	}
}

func TestValueHoldings_CrossCurrency(t *testing.T) {
	// Holding is AAPL (USD), user base is EUR.
	// 1 USD = 1.0 USD, 1 EUR = 1.10 USD → 1 USD = 1/1.10 EUR ≈ 0.909 EUR.
	h := []Holding{{
		Symbol: "AAPL", Qty: 10,
		CostNative: 1000, CostBase: 909.09,
		AvgCostNative: 100, AvgCostBase: 90.909,
	}}
	vals := ValueHoldings(h,
		staticPrice(map[string]float64{"AAPL": 110}),
		staticFx(map[domain.Currency]float64{domain.USD: 1.0, domain.EUR: 1.10}),
		staticCur(map[string]domain.Currency{"AAPL": domain.USD}),
		domain.EUR,
	)
	v := vals[0]
	// ValueNative = 1100 USD; ValueBase = 1100 * (1/1.10) = 1000 EUR.
	if !approx(v.ValueNative, 1100) {
		t.Errorf("ValueNative: %v", v.ValueNative)
	}
	if !approx(v.ValueBase, 1000) {
		t.Errorf("ValueBase: %v", v.ValueBase)
	}
}

func TestValueHoldings_MissingPriceSkipped(t *testing.T) {
	h := []Holding{
		{Symbol: "AAPL", Qty: 1, CostNative: 100, CostBase: 100},
		{Symbol: "BTC", Qty: 1, CostNative: 50000, CostBase: 50000},
	}
	vals := ValueHoldings(h,
		staticPrice(map[string]float64{"AAPL": 110}),
		staticFx(map[domain.Currency]float64{domain.USD: 1.0}),
		staticCur(map[string]domain.Currency{"AAPL": domain.USD, "BTC": domain.USD}),
		domain.USD,
	)
	if len(vals) != 1 || vals[0].Symbol != "AAPL" {
		t.Errorf("expected only AAPL priced, got %+v", vals)
	}
}

func TestValueHoldings_MissingFxSkipped(t *testing.T) {
	h := []Holding{{Symbol: "SAP", Qty: 1, CostNative: 150, CostBase: 165}}
	vals := ValueHoldings(h,
		staticPrice(map[string]float64{"SAP": 200}),
		staticFx(map[domain.Currency]float64{domain.USD: 1.0}), // missing EUR
		staticCur(map[string]domain.Currency{"SAP": domain.EUR}),
		domain.USD,
	)
	if len(vals) != 0 {
		t.Errorf("expected skipped, got %+v", vals)
	}
}

func TestValueHoldings_ZeroCostNoPct(t *testing.T) {
	h := []Holding{{Symbol: "GIFT", Qty: 1, CostNative: 0, CostBase: 0}}
	vals := ValueHoldings(h,
		staticPrice(map[string]float64{"GIFT": 50}),
		staticFx(map[domain.Currency]float64{domain.USD: 1.0}),
		staticCur(map[string]domain.Currency{"GIFT": domain.USD}),
		domain.USD,
	)
	if vals[0].PnLPctBase != 0 || vals[0].PnLPctNative != 0 {
		t.Errorf("zero cost should yield 0%% PnL, got %+v", vals[0])
	}
}

func TestTotals(t *testing.T) {
	vals := []HoldingValue{
		{Holding: Holding{CostBase: 500}, ValueBase: 600},
		{Holding: Holding{CostBase: 300}, ValueBase: 400},
	}
	if TotalValueBase(vals) != 1000 {
		t.Errorf("TotalValueBase: %v", TotalValueBase(vals))
	}
	if TotalCostBase(vals) != 800 {
		t.Errorf("TotalCostBase: %v", TotalCostBase(vals))
	}
}
