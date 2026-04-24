// Package importers converts portfolio exports from external software
// into a normalised ImportPlan that the server can apply atomically.
//
// Each supported source has its own parser (e.g. ghostfolio.go). The
// plan is designed to be round-tripped to the browser unchanged —
// analyse returns it, the user edits selections / mappings, and apply
// echoes it back. That keeps the server stateless between the two
// phases.
package importers

import (
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// ImportPlan is the normalised, source-agnostic representation of a
// portfolio dump. Accounts and assets carry a SourceID used by
// transactions to reference them; on apply the server maps each
// SourceID to either an existing entity (MapToID / MapToSymbol) or a
// newly-created one.
type ImportPlan struct {
	Source       string              `json:"source"`
	SourceMeta   map[string]any      `json:"source_meta"`
	Accounts     []ImportAccount     `json:"accounts"`
	Assets       []ImportAsset       `json:"assets"`
	Transactions []ImportTransaction `json:"transactions"`
	// Warnings are surfaced verbatim in the UI — parser-level issues
	// that don't block the import but the user should see (e.g.
	// "skipped 4 activities of unsupported type DIVIDEND").
	Warnings []string `json:"warnings,omitempty"`
}

// ImportAccount is a single account entry in the plan.
//
// The first four fields are populated by the parser. ExistingID is
// filled by the analyze handler from the user's current accounts list
// (case-insensitive name match). The last two fields are the user's
// selections, sent back on apply: Selected=false skips the account and
// any transactions referencing it; MapToID != 0 means "don't create a
// new row — reuse this existing account".
type ImportAccount struct {
	SourceID   string          `json:"source_id"`
	Name       string          `json:"name"`
	Currency   domain.Currency `json:"currency"`
	TxCount    int             `json:"tx_count"`
	ExistingID int64           `json:"existing_id,omitempty"`
	Selected   bool            `json:"selected"`
	MapToID    int64           `json:"map_to_id,omitempty"`
}

// ImportAsset is a single asset entry in the plan. Same lifecycle as
// ImportAccount: parser fills Symbol/Name/Type/Currency/Provider, the
// analyze handler fills ExistingMatch from the current assets table,
// and the client flips Selected / sets MapToSymbol.
type ImportAsset struct {
	SourceID      string           `json:"source_id"`
	Symbol        string           `json:"symbol"`
	Name          string           `json:"name"`
	Type          domain.AssetType `json:"type"`
	Currency      domain.Currency  `json:"currency"`
	Provider      string           `json:"provider"`
	ProviderID    string           `json:"provider_id"`
	LogoURL       string           `json:"logo_url,omitempty"`
	TxCount       int              `json:"tx_count"`
	ExistingMatch bool             `json:"existing_match,omitempty"`
	Selected      bool             `json:"selected"`
	MapToSymbol   string           `json:"map_to_symbol,omitempty"`
}

// ImportTransaction is one transaction keyed by the plan's SourceIDs.
// fx_to_base is not carried here — the apply phase computes it against
// the current user's base currency using Frankfurter.
type ImportTransaction struct {
	AccountSourceID string          `json:"account_source_id"`
	AssetSourceID   string          `json:"asset_source_id"`
	Side            domain.TxSide   `json:"side"`
	Qty             float64         `json:"qty"`
	Price           float64         `json:"price"`
	Fee             float64         `json:"fee"`
	Currency        domain.Currency `json:"currency"`
	OccurredAt      time.Time       `json:"occurred_at"`
	Note            string          `json:"note"`
}
