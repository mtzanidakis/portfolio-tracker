package exporters

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestWriteJSON_Envelope(t *testing.T) {
	accounts := []*domain.Account{{
		ID: 1, Name: "Brokerage", Currency: domain.EUR,
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}}
	assets := []*domain.Asset{{
		Symbol: "AAPL", Name: "Apple Inc.",
		Type: domain.AssetStock, Currency: domain.USD, Provider: "yahoo",
	}}
	txs := []*domain.Transaction{{
		ID: 1, AccountID: 1, AssetSymbol: "AAPL", Side: domain.SideBuy,
		Qty: 1, Price: 100, FxToBase: 0.92,
		OccurredAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
	}}

	var buf bytes.Buffer
	if err := WriteJSON(&buf, "test-1.0", domain.EUR, accounts, assets, txs); err != nil {
		t.Fatalf("write: %v", err)
	}

	var out Snapshot
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v\nbody: %s", err, buf.String())
	}
	if out.Meta.Source != "portfolio-tracker" {
		t.Errorf("meta.source = %q, want portfolio-tracker", out.Meta.Source)
	}
	if out.Meta.Version != "test-1.0" {
		t.Errorf("meta.version = %q", out.Meta.Version)
	}
	if out.BaseCurrency != domain.EUR {
		t.Errorf("base = %q", out.BaseCurrency)
	}
	if len(out.Accounts) != 1 || len(out.Assets) != 1 || len(out.Transactions) != 1 {
		t.Errorf("counts wrong: %d / %d / %d",
			len(out.Accounts), len(out.Assets), len(out.Transactions))
	}
	// Smoke-check the JSON is pretty-printed (download-friendly).
	if !strings.Contains(buf.String(), "\n  ") {
		t.Error("expected indented JSON output")
	}
}
