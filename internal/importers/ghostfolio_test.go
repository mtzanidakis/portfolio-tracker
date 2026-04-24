package importers

import (
	"testing"
)

// minimal export fixture exercising every code path: YAHOO stock,
// COINGECKO crypto, MANUAL crypto with a profile, a skipped DIVIDEND,
// an account with no activities (dropped), and a balances[] so the
// "balance history ignored" warning fires.
const fixtureGhostfolio = `{
  "meta": {"date": "2026-04-24T22:11:12.679Z", "version": "2.255.0"},
  "accounts": [
    {"id": "acc-1", "name": "Revolut Crypto", "currency": "EUR",
     "balances": [{"date":"2026-01-19T00:00:00.000Z","value":0}]},
    {"id": "acc-2", "name": "Unused", "currency": "EUR"}
  ],
  "assetProfiles": [
    {"symbol":"BTC","dataSource":"MANUAL","assetClass":"LIQUIDITY",
     "assetSubClass":"CRYPTOCURRENCY","currency":"EUR","name":"Bitcoin"}
  ],
  "activities": [
    {"accountId":"acc-1","type":"BUY","symbol":"BTC","dataSource":"MANUAL",
     "currency":"EUR","unitPrice":40000,"quantity":0.1,"fee":1,
     "date":"2024-01-04T22:00:00.000Z","comment":"first"},
    {"accountId":"acc-1","type":"BUY","symbol":"VWCE.DE","dataSource":"YAHOO",
     "currency":"EUR","unitPrice":100,"quantity":3,"fee":0.5,
     "date":"2024-02-01T00:00:00.000Z"},
    {"accountId":"acc-1","type":"BUY","symbol":"ethereum","dataSource":"COINGECKO",
     "currency":"EUR","unitPrice":2000,"quantity":0.5,"fee":0,
     "date":"2024-03-01T00:00:00.000Z"},
    {"accountId":"acc-1","type":"DIVIDEND","symbol":"VWCE.DE","dataSource":"YAHOO",
     "currency":"EUR","unitPrice":2,"quantity":3,"fee":0,
     "date":"2024-04-01T00:00:00.000Z"}
  ]
}`

func TestParseGhostfolio(t *testing.T) {
	plan, err := ParseGhostfolio([]byte(fixtureGhostfolio))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := len(plan.Transactions); got != 3 {
		t.Errorf("transactions: got %d, want 3 (DIVIDEND dropped)", got)
	}
	if got := len(plan.Accounts); got != 1 {
		t.Errorf("accounts: got %d, want 1 (Unused dropped)", got)
	}
	if got := len(plan.Assets); got != 3 {
		t.Errorf("assets: got %d, want 3", got)
	}

	bySym := map[string]ImportAsset{}
	for _, a := range plan.Assets {
		bySym[a.Symbol] = a
	}
	if a := bySym["BTC"]; a.Provider != "manual" || a.Type != "crypto" {
		t.Errorf("BTC asset = %+v", a)
	}
	if a := bySym["VWCE.DE"]; a.Provider != "yahoo" || a.Type != "stock" {
		t.Errorf("VWCE.DE asset = %+v (default stock before enrichment)", a)
	}
	if a := bySym["ETHEREUM"]; a.Provider != "coingecko" || a.Type != "crypto" {
		t.Errorf("ETHEREUM asset = %+v", a)
	}

	if !containsSubstr(plan.Warnings, "DIVIDEND") {
		t.Errorf("expected DIVIDEND warning, got %v", plan.Warnings)
	}
	if !containsSubstr(plan.Warnings, "balance history is ignored") {
		t.Errorf("expected balance-history warning, got %v", plan.Warnings)
	}
}

func containsSubstr(ss []string, needle string) bool {
	for _, s := range ss {
		for i := 0; i+len(needle) <= len(s); i++ {
			if s[i:i+len(needle)] == needle {
				return true
			}
		}
	}
	return false
}
