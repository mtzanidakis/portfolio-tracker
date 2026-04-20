// Package domain defines the core entities and enumerations used across
// the portfolio tracker (persisted models plus helpers).
package domain

import (
	"fmt"
	"slices"
	"strings"
)

// Currency is an ISO-4217 fiat currency code supported by the tracker.
type Currency string

// Supported fiat currencies.
const (
	USD Currency = "USD"
	EUR Currency = "EUR"
	GBP Currency = "GBP"
	JPY Currency = "JPY"
	CHF Currency = "CHF"
	CAD Currency = "CAD"
	AUD Currency = "AUD"
)

// AllCurrencies is the canonical list of supported currencies.
var AllCurrencies = []Currency{USD, EUR, GBP, JPY, CHF, CAD, AUD}

// Valid reports whether c is a supported currency.
func (c Currency) Valid() bool {
	return slices.Contains(AllCurrencies, c)
}

// Decimals returns the conventional display precision for the currency.
// JPY uses 0 decimal places; all others use 2.
func (c Currency) Decimals() int {
	if c == JPY {
		return 0
	}
	return 2
}

// ParseCurrency parses a user-supplied currency code, case-insensitive.
// Returns an error if the code is not in AllCurrencies.
func ParseCurrency(s string) (Currency, error) {
	c := Currency(strings.ToUpper(strings.TrimSpace(s)))
	if !c.Valid() {
		return "", fmt.Errorf("unknown currency: %q", s)
	}
	return c, nil
}

// AssetType categorizes an asset.
type AssetType string

// Known asset types.
const (
	AssetStock  AssetType = "stock"
	AssetETF    AssetType = "etf"
	AssetCrypto AssetType = "crypto"
	AssetCash   AssetType = "cash"
)

// AllAssetTypes is the canonical list of asset types.
var AllAssetTypes = []AssetType{AssetStock, AssetETF, AssetCrypto, AssetCash}

// Valid reports whether t is a known asset type.
func (t AssetType) Valid() bool {
	return slices.Contains(AllAssetTypes, t)
}

// ParseAssetType parses a user-supplied type, case-insensitive.
func ParseAssetType(s string) (AssetType, error) {
	t := AssetType(strings.ToLower(strings.TrimSpace(s)))
	if !t.Valid() {
		return "", fmt.Errorf("unknown asset type: %q", s)
	}
	return t, nil
}

// TxSide is the side of a transaction (buy or sell).
type TxSide string

// Transaction sides.
const (
	SideBuy  TxSide = "buy"
	SideSell TxSide = "sell"
)

// Valid reports whether s is a known side.
func (s TxSide) Valid() bool {
	return s == SideBuy || s == SideSell
}

// ParseTxSide parses a user-supplied side, case-insensitive.
func ParseTxSide(s string) (TxSide, error) {
	t := TxSide(strings.ToLower(strings.TrimSpace(s)))
	if !t.Valid() {
		return "", fmt.Errorf("unknown transaction side: %q", s)
	}
	return t, nil
}
