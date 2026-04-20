package main

import (
	"flag"
	"fmt"
	"time"
)

// requireYes parses common write-command flags. Returns false if --yes
// is missing, after printing a helpful error.
func requireYes(fs *flag.FlagSet, args []string) bool {
	yes := fs.Bool("yes", false, "confirm write")
	if err := fs.Parse(args); err != nil {
		return false
	}
	if !*yes {
		_, _ = fmt.Fprintln(fs.Output(), "refused: pass --yes to confirm this write")
		return false
	}
	return true
}

func cmdAddTx(cfg *config, args []string) int {
	fs := flag.NewFlagSet("add-tx", flag.ContinueOnError)
	var (
		account int64
		symbol  = fs.String("symbol", "", "asset symbol (required)")
		side    = fs.String("side", "buy", "buy|sell")
		qty     = fs.Float64("qty", 0, "quantity (required)")
		price   = fs.Float64("price", 0, "per-unit price (required)")
		fee     = fs.Float64("fee", 0, "fee in asset currency")
		fx      = fs.Float64("fx", 1.0, "fx rate asset→base at trade time")
		date    = fs.String("date", time.Now().UTC().Format("2006-01-02"), "YYYY-MM-DD")
		note    = fs.String("note", "", "free-text note")
	)
	fs.Int64Var(&account, "account-id", 0, "account id (required)")
	if !requireYes(fs, args) {
		return 2
	}
	if account == 0 || *symbol == "" || *qty <= 0 || *price <= 0 {
		return errf("--account-id, --symbol, --qty, --price are required")
	}
	body := map[string]any{
		"account_id":   account,
		"asset_symbol": *symbol,
		"side":         *side,
		"qty":          *qty,
		"price":        *price,
		"fee":          *fee,
		"fx_to_base":   *fx,
		"occurred_at":  *date + "T12:00:00Z",
		"note":         *note,
	}
	var out map[string]any
	if err := apiPOST(cfg, "/api/v1/transactions", body, &out); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("created tx id=%v %v %v %v @ %v\n", out["id"], out["side"], out["qty"], out["asset_symbol"], out["price"])
	return 0
}

func cmdAddAccount(cfg *config, args []string) int {
	fs := flag.NewFlagSet("add-account", flag.ContinueOnError)
	var (
		name      = fs.String("name", "", "display name (required)")
		typ       = fs.String("type", "Brokerage", "free-text type label")
		short     = fs.String("short", "", "2-3 char label")
		color     = fs.String("color", "#c8502a", "hex color")
		currency  = fs.String("currency", "USD", "default currency")
		connected = fs.Bool("connected", true, "mark as connected")
	)
	if !requireYes(fs, args) {
		return 2
	}
	if *name == "" {
		return errf("--name is required")
	}
	body := map[string]any{
		"name": *name, "type": *typ, "short": *short, "color": *color,
		"currency": *currency, "connected": *connected,
	}
	var out map[string]any
	if err := apiPOST(cfg, "/api/v1/accounts", body, &out); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("created account id=%v %v\n", out["id"], out["name"])
	return 0
}

func cmdAddAsset(cfg *config, args []string) int {
	fs := flag.NewFlagSet("add-asset", flag.ContinueOnError)
	var (
		symbol     = fs.String("symbol", "", "ticker/symbol (required)")
		name       = fs.String("name", "", "display name (required)")
		typ        = fs.String("type", "stock", "stock|etf|crypto|cash")
		currency   = fs.String("currency", "USD", "native currency")
		color      = fs.String("color", "", "hex color for UI")
		provider   = fs.String("provider", "", "yahoo|coingecko|(empty for cash)")
		providerID = fs.String("provider-id", "", "external ID at provider")
	)
	if !requireYes(fs, args) {
		return 2
	}
	if *symbol == "" || *name == "" {
		return errf("--symbol and --name are required")
	}
	body := map[string]any{
		"symbol": *symbol, "name": *name, "type": *typ,
		"currency": *currency, "color": *color,
		"provider": *provider, "provider_id": *providerID,
	}
	var out map[string]any
	if err := apiPOST(cfg, "/api/v1/assets", body, &out); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("upserted asset %v (%v)\n", out["symbol"], out["name"])
	return 0
}

func cmdDeleteTx(cfg *config, args []string) int {
	fs := flag.NewFlagSet("delete-tx", flag.ContinueOnError)
	id := fs.Int64("id", 0, "transaction id (required)")
	if !requireYes(fs, args) {
		return 2
	}
	if *id == 0 {
		return errf("--id is required")
	}
	if err := apiDELETE(cfg, fmt.Sprintf("/api/v1/transactions/%d", *id)); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("deleted tx id=%d\n", *id)
	return 0
}

func cmdDeleteAccount(cfg *config, args []string) int {
	fs := flag.NewFlagSet("delete-account", flag.ContinueOnError)
	id := fs.Int64("id", 0, "account id (required)")
	if !requireYes(fs, args) {
		return 2
	}
	if *id == 0 {
		return errf("--id is required")
	}
	if err := apiDELETE(cfg, fmt.Sprintf("/api/v1/accounts/%d", *id)); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("deleted account id=%d\n", *id)
	return 0
}

func cmdSetBaseCurrency(cfg *config, args []string) int {
	fs := flag.NewFlagSet("set-base-currency", flag.ContinueOnError)
	currency := fs.String("currency", "", "USD|EUR|... (required)")
	if !requireYes(fs, args) {
		return 2
	}
	if *currency == "" {
		return errf("--currency is required")
	}
	body := map[string]any{"base_currency": *currency}
	var out map[string]any
	if err := apiPATCH(cfg, "/api/v1/me", body, &out); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("base currency updated to %v\n", out["base_currency"])
	return 0
}
