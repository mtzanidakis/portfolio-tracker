package importers

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// ghostfolioExport mirrors the top-level shape of a Ghostfolio export
// dump. We only decode the fields we actually use.
type ghostfolioExport struct {
	Meta struct {
		Date    string `json:"date"`
		Version string `json:"version"`
	} `json:"meta"`
	Accounts      []ghostfolioAccount  `json:"accounts"`
	Activities    []ghostfolioActivity `json:"activities"`
	AssetProfiles []ghostfolioProfile  `json:"assetProfiles"`
}

type ghostfolioAccount struct {
	ID       string           `json:"id"`
	Name     string           `json:"name"`
	Currency string           `json:"currency"`
	Balances []map[string]any `json:"balances"`
}

type ghostfolioActivity struct {
	AccountID  string  `json:"accountId"`
	Type       string  `json:"type"`
	Symbol     string  `json:"symbol"`
	DataSource string  `json:"dataSource"`
	Currency   string  `json:"currency"`
	UnitPrice  float64 `json:"unitPrice"`
	Quantity   float64 `json:"quantity"`
	Fee        float64 `json:"fee"`
	Date       string  `json:"date"`
	Comment    string  `json:"comment"`
}

type ghostfolioProfile struct {
	AssetClass    string `json:"assetClass"`
	AssetSubClass string `json:"assetSubClass"`
	Currency      string `json:"currency"`
	DataSource    string `json:"dataSource"`
	Name          string `json:"name"`
	Symbol        string `json:"symbol"`
}

// ParseGhostfolio converts a Ghostfolio export (raw JSON body) into a
// normalised ImportPlan. It skips accounts with no activities and
// activity types we don't yet support, noting both in plan.Warnings so
// the UI can show the user what was dropped. No network I/O —
// enrichment (Yahoo lookups, existing match detection) happens in the
// analyze handler.
func ParseGhostfolio(body []byte) (*ImportPlan, error) {
	var raw ghostfolioExport
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse ghostfolio json: %w", err)
	}

	plan := &ImportPlan{
		Source: "ghostfolio",
		SourceMeta: map[string]any{
			"version":     raw.Meta.Version,
			"exported_at": raw.Meta.Date,
		},
	}

	// Index asset profiles so we can resolve assetSubClass for MANUAL
	// assets (where Ghostfolio stores classification) without a
	// provider lookup.
	profileByKey := make(map[string]ghostfolioProfile, len(raw.AssetProfiles))
	for _, p := range raw.AssetProfiles {
		profileByKey[profileKey(p.DataSource, p.Symbol)] = p
	}

	// Pass 1: transactions. Filter out unsupported types, count per
	// account and per asset so the review step can show activity
	// counts next to each row.
	accountTxs := map[string]int{}
	assetTxs := map[string]int{}
	firstAct := map[string]ghostfolioActivity{}
	skipped := map[string]int{}

	for _, a := range raw.Activities {
		side, ok := ghostfolioSide(a.Type)
		if !ok {
			skipped[strings.ToUpper(a.Type)]++
			continue
		}
		cur, err := domain.ParseCurrency(a.Currency)
		if err != nil {
			skipped["BAD_CURRENCY"]++
			continue
		}
		occurred, err := parseGhostfolioDate(a.Date)
		if err != nil {
			skipped["BAD_DATE"]++
			continue
		}
		assetKey := profileKey(a.DataSource, a.Symbol)

		plan.Transactions = append(plan.Transactions, ImportTransaction{
			AccountSourceID: a.AccountID,
			AssetSourceID:   assetKey,
			Side:            side,
			Qty:             a.Quantity,
			Price:           a.UnitPrice,
			Fee:             a.Fee,
			Currency:        cur,
			OccurredAt:      occurred,
			Note:            a.Comment,
		})
		accountTxs[a.AccountID]++
		assetTxs[assetKey]++
		if _, seen := firstAct[assetKey]; !seen {
			firstAct[assetKey] = a
		}
	}

	// Pass 2: accounts referenced by at least one accepted activity.
	hasBalances := false
	for _, acc := range raw.Accounts {
		if len(acc.Balances) > 0 {
			hasBalances = true
		}
		n := accountTxs[acc.ID]
		if n == 0 {
			continue
		}
		cur, err := domain.ParseCurrency(acc.Currency)
		if err != nil {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("Account %q has unsupported currency %q — skipped", acc.Name, acc.Currency))
			continue
		}
		plan.Accounts = append(plan.Accounts, ImportAccount{
			SourceID: acc.ID,
			Name:     acc.Name,
			Currency: cur,
			TxCount:  n,
			Selected: true,
		})
	}

	// Pass 3: one asset entry per (dataSource, symbol) with
	// activities. Type defaults to what the dataSource + assetSubClass
	// imply; the analyze handler refines YAHOO-sourced assets via a
	// real provider lookup.
	for key, first := range firstAct {
		assetCur, err := domain.ParseCurrency(first.Currency)
		if err != nil {
			plan.Warnings = append(plan.Warnings,
				fmt.Sprintf("Asset %q has unsupported currency %q — skipped", first.Symbol, first.Currency))
			continue
		}
		profile := profileByKey[key]
		typ := ghostfolioAssetType(first.DataSource, profile.AssetSubClass)
		provider, providerID := ghostfolioProviderMapping(first.DataSource, first.Symbol)
		name := profile.Name
		if name == "" {
			name = first.Symbol
		}
		plan.Assets = append(plan.Assets, ImportAsset{
			SourceID:   key,
			Symbol:     trackerSymbol(first.DataSource, first.Symbol),
			Name:       name,
			Type:       typ,
			Currency:   assetCur,
			Provider:   provider,
			ProviderID: providerID,
			TxCount:    assetTxs[key],
			Selected:   true,
		})
	}

	for reason, n := range skipped {
		plan.Warnings = append(plan.Warnings,
			fmt.Sprintf("Skipped %d activity(ies): %s", n, ghostfolioSkipLabel(reason)))
	}
	if hasBalances {
		plan.Warnings = append(plan.Warnings,
			"Ghostfolio account balance history is ignored — only BUY/SELL activities are imported")
	}
	return plan, nil
}

// ghostfolioSide maps a Ghostfolio activity type onto our TxSide enum.
// DIVIDEND / INTEREST / FEE / ITEM / LIABILITY fall through and the
// parser accumulates them into a skipped-count warning.
func ghostfolioSide(t string) (domain.TxSide, bool) {
	switch strings.ToUpper(strings.TrimSpace(t)) {
	case "BUY":
		return domain.SideBuy, true
	case "SELL":
		return domain.SideSell, true
	}
	return "", false
}

// ghostfolioAssetType resolves a tracker asset type. COINGECKO is
// always crypto; YAHOO without a hint is assumed stock (the analyze
// handler refines this with a real Yahoo lookup); MANUAL trusts the
// profile's assetSubClass.
func ghostfolioAssetType(dataSource, subClass string) domain.AssetType {
	switch strings.ToUpper(dataSource) {
	case "COINGECKO":
		return domain.AssetCrypto
	case "YAHOO":
		switch strings.ToUpper(subClass) {
		case "ETF":
			return domain.AssetETF
		case "CRYPTOCURRENCY":
			return domain.AssetCrypto
		}
		return domain.AssetStock
	}
	switch strings.ToUpper(subClass) {
	case "CRYPTOCURRENCY":
		return domain.AssetCrypto
	case "ETF":
		return domain.AssetETF
	case "STOCK":
		return domain.AssetStock
	}
	return domain.AssetStock
}

func ghostfolioProviderMapping(dataSource, symbol string) (string, string) {
	switch strings.ToUpper(dataSource) {
	case "YAHOO":
		return "yahoo", symbol
	case "COINGECKO":
		return "coingecko", symbol
	}
	return "manual", ""
}

// trackerSymbol chooses the tracker-side asset symbol. Yahoo and MANUAL
// keep their symbol as-is (BTC, VWCE.DE). CoinGecko coin IDs like
// "ethereum" are uppercased for consistency with the rest of our asset
// list.
func trackerSymbol(dataSource, symbol string) string {
	if strings.EqualFold(dataSource, "COINGECKO") {
		return strings.ToUpper(symbol)
	}
	return symbol
}

func profileKey(dataSource, symbol string) string {
	return strings.ToLower(dataSource) + "::" + symbol
}

func ghostfolioSkipLabel(reason string) string {
	switch reason {
	case "BAD_CURRENCY":
		return "unsupported currency"
	case "BAD_DATE":
		return "unparseable date"
	default:
		return "unsupported type " + reason
	}
}

// parseGhostfolioDate accepts Ghostfolio's RFC3339 format (with or
// without fractional seconds; always UTC, Z-suffixed).
func parseGhostfolioDate(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339Nano,
		"2006-01-02T15:04:05.000Z",
		time.RFC3339,
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognised date format %q", s)
}
