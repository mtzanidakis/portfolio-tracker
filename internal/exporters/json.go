// Package exporters serialises a user's accounts, assets and
// transactions into portable formats the tracker can hand back.
//
// Two formats are supported today:
//
//   - WriteJSON emits a full snapshot (meta + accounts + assets +
//     transactions) intended to be re-importable by a future
//     "import from portfolio-tracker" source.
//   - WriteCSV emits transactions only, joined with the owning
//     account and asset for readability, for use in spreadsheets.
package exporters

import (
	"encoding/json"
	"io"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// Snapshot is the JSON envelope. Kept self-describing (Meta.Source and
// Meta.Version) so a future version of the app can detect and adapt
// to format changes without heuristics.
type Snapshot struct {
	Meta         SnapshotMeta          `json:"meta"`
	BaseCurrency domain.Currency       `json:"base_currency"`
	Accounts     []*domain.Account     `json:"accounts"`
	Assets       []*domain.Asset       `json:"assets"`
	Transactions []*domain.Transaction `json:"transactions"`
}

// SnapshotMeta records when and by whom the snapshot was produced.
type SnapshotMeta struct {
	Source     string    `json:"source"`  // always "portfolio-tracker"
	Version    string    `json:"version"` // server build version
	ExportedAt time.Time `json:"exported_at"`
}

// WriteJSON encodes the snapshot as indented JSON, suitable for
// downloading as a human-readable backup.
func WriteJSON(w io.Writer, version string, base domain.Currency,
	accounts []*domain.Account, assets []*domain.Asset, txs []*domain.Transaction,
) error {
	snap := Snapshot{
		Meta: SnapshotMeta{
			Source:     "portfolio-tracker",
			Version:    version,
			ExportedAt: time.Now().UTC(),
		},
		BaseCurrency: base,
		Accounts:     accounts,
		Assets:       assets,
		Transactions: txs,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(snap)
}
