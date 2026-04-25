package exporters

import (
	"bytes"
	"encoding/csv"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestWriteTransactionsCSV(t *testing.T) {
	txs := []*domain.Transaction{{
		ID: 1, AccountID: 7, AssetSymbol: "AAPL", Side: domain.SideBuy,
		Qty: 2.5, Price: 198.2, Fee: 1.0, FxToBase: 0.92,
		OccurredAt: time.Date(2026, 4, 1, 12, 0, 0, 0, time.UTC),
		Note:       "weekly DCA",
	}}
	accountName := map[int64]string{7: "Brokerage"}
	symbolInfo := map[string]*domain.Asset{"AAPL": {
		Symbol: "AAPL", Name: "Apple Inc.",
		Type: domain.AssetStock, Currency: domain.USD,
	}}

	var buf bytes.Buffer
	if err := WriteTransactionsCSV(&buf, txs, accountName, symbolInfo); err != nil {
		t.Fatalf("write: %v", err)
	}

	rows, err := csv.NewReader(&buf).ReadAll()
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header + 1 row, got %d", len(rows))
	}
	if rows[0][0] != "date" || rows[0][1] != "account" {
		t.Errorf("header = %v", rows[0])
	}
	want := []string{
		"2026-04-01", "Brokerage", "AAPL", "Apple Inc.", "stock",
		"buy", "2.5", "198.2", "1", "USD", "0.92", "weekly DCA",
	}
	for i, w := range want {
		if rows[1][i] != w {
			t.Errorf("col %d (%s) = %q, want %q", i, rows[0][i], rows[1][i], w)
		}
	}
}
