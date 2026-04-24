package exporters

import (
	"encoding/csv"
	"io"
	"strconv"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// csvHeader is the column order the CSV exporter emits. Kept stable so
// downstream spreadsheets / scripts can rely on positional decoding.
var csvHeader = []string{
	"date", "account", "symbol", "asset_name", "asset_type",
	"side", "qty", "price", "fee", "currency", "fx_to_base", "note",
}

// WriteTransactionsCSV writes one CSV row per transaction with the
// account and asset joined in from the provided lookups.
//
// accountName maps account IDs to the user-facing label and symbolInfo
// maps asset symbols to (name, type, currency). Rows whose references
// miss a lookup are still written with empty values — losing the
// reference is a data issue worth exposing, not silently dropping.
func WriteTransactionsCSV(
	w io.Writer,
	txs []*domain.Transaction,
	accountName map[int64]string,
	symbolInfo map[string]*domain.Asset,
) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write(csvHeader); err != nil {
		return err
	}
	for _, t := range txs {
		var name, typ, cur string
		if a, ok := symbolInfo[t.AssetSymbol]; ok && a != nil {
			name = a.Name
			typ = string(a.Type)
			cur = string(a.Currency)
		}
		if err := cw.Write([]string{
			t.OccurredAt.UTC().Format("2006-01-02"),
			accountName[t.AccountID],
			t.AssetSymbol,
			name,
			typ,
			string(t.Side),
			strconv.FormatFloat(t.Qty, 'f', -1, 64),
			strconv.FormatFloat(t.Price, 'f', -1, 64),
			strconv.FormatFloat(t.Fee, 'f', -1, 64),
			cur,
			strconv.FormatFloat(t.FxToBase, 'f', -1, 64),
			t.Note,
		}); err != nil {
			return err
		}
	}
	return cw.Error()
}
