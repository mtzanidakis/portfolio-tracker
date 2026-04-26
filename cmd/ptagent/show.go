package main

import (
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"text/tabwriter"
)

func parseShowFlags(name string, args []string, extra func(*flag.FlagSet)) (asJSON, ok bool) {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	j := fs.Bool("json", false, "output raw JSON")
	if extra != nil {
		extra(fs)
	}
	if err := fs.Parse(args); err != nil {
		return false, false
	}
	return *j, true
}

func cmdMe(cfg *config, args []string) int {
	asJSON, ok := parseShowFlags("me", args, nil)
	if !ok {
		return 2
	}
	var me map[string]any
	if err := apiGET(cfg, "/api/v1/me", &me); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(me)
		return 0
	}
	fmt.Printf("id:            %v\n", me["id"])
	fmt.Printf("email:         %v\n", me["email"])
	fmt.Printf("name:          %v\n", me["name"])
	fmt.Printf("base_currency: %v\n", me["base_currency"])
	return 0
}

func cmdHoldings(cfg *config, args []string) int {
	asJSON, ok := parseShowFlags("holdings", args, nil)
	if !ok {
		return 2
	}
	var rows []map[string]any
	if err := apiGET(cfg, "/api/v1/holdings", &rows); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(rows)
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SYMBOL\tQTY\tAVG (BASE)\tVALUE (BASE)\tPNL\tPNL%")
	for _, h := range rows {
		_, _ = fmt.Fprintf(w, "%v\t%v\t%.2f\t%.2f\t%.2f\t%.2f%%\n",
			h["Symbol"], h["Qty"],
			toFloat(h["AvgCostBase"]),
			toFloat(h["ValueBase"]),
			toFloat(h["PnLBase"]),
			toFloat(h["PnLPctBase"]),
		)
	}
	_ = w.Flush()
	return 0
}

func cmdPerformance(cfg *config, args []string) int {
	var tf string
	asJSON, ok := parseShowFlags("performance", args, func(fs *flag.FlagSet) {
		fs.StringVar(&tf, "tf", "6M", "timeframe")
	})
	if !ok {
		return 2
	}
	var p map[string]any
	if err := apiGET(cfg, "/api/v1/performance?tf="+tf, &p); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(p)
		return 0
	}
	fmt.Printf("timeframe: %v (%v)\n", p["timeframe"], p["currency"])
	fmt.Printf("total:     %.2f\n", toFloat(p["total"]))
	fmt.Printf("cost:      %.2f\n", toFloat(p["cost"]))
	fmt.Printf("pnl:       %.2f (%.2f%%)\n", toFloat(p["pnl"]), toFloat(p["pnl_pct"]))
	return 0
}

func cmdAllocations(cfg *config, args []string) int {
	var group string
	asJSON, ok := parseShowFlags("allocations", args, func(fs *flag.FlagSet) {
		fs.StringVar(&group, "group", "asset", "asset|type|account")
	})
	if !ok {
		return 2
	}
	var rows []map[string]any
	if err := apiGET(cfg, "/api/v1/allocations?group="+group, &rows); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(rows)
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "KEY\tLABEL\tVALUE\tSHARE")
	for _, r := range rows {
		_, _ = fmt.Fprintf(w, "%v\t%v\t%.2f\t%.1f%%\n",
			r["key"], r["label"], toFloat(r["value"]), toFloat(r["fraction"])*100)
	}
	_ = w.Flush()
	return 0
}

func cmdAccounts(cfg *config, args []string) int {
	asJSON, ok := parseShowFlags("accounts", args, nil)
	if !ok {
		return 2
	}
	var rows []map[string]any
	if err := apiGET(cfg, "/api/v1/accounts", &rows); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(rows)
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tNAME\tTYPE\tCCY")
	for _, a := range rows {
		_, _ = fmt.Fprintf(w, "%v\t%v\t%v\t%v\n",
			a["id"], a["name"], a["type"], a["currency"])
	}
	_ = w.Flush()
	return 0
}

func cmdAssets(cfg *config, args []string) int {
	var q string
	asJSON, ok := parseShowFlags("assets", args, func(fs *flag.FlagSet) {
		fs.StringVar(&q, "q", "", "search")
	})
	if !ok {
		return 2
	}
	path := "/api/v1/assets"
	if q != "" {
		path += "?q=" + q
	}
	var rows []map[string]any
	if err := apiGET(cfg, path, &rows); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(rows)
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "SYMBOL\tNAME\tTYPE\tCCY\tPROVIDER")
	for _, a := range rows {
		_, _ = fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
			a["symbol"], a["name"], a["type"], a["currency"], a["provider"])
	}
	_ = w.Flush()
	return 0
}

func cmdTransactions(cfg *config, args []string) int {
	var symbol, side string
	var limit int
	asJSON, ok := parseShowFlags("transactions", args, func(fs *flag.FlagSet) {
		fs.StringVar(&symbol, "symbol", "", "filter by symbol")
		fs.StringVar(&side, "side", "", "buy|sell")
		fs.IntVar(&limit, "limit", 0, "max rows")
	})
	if !ok {
		return 2
	}
	qs := "?"
	if symbol != "" {
		qs += "symbol=" + symbol + "&"
	}
	if side != "" {
		qs += "side=" + side + "&"
	}
	if limit > 0 {
		qs += fmt.Sprintf("limit=%d&", limit)
	}
	path := "/api/v1/transactions"
	if qs != "?" {
		path += qs
	}
	var rows []map[string]any
	if err := apiGET(cfg, path, &rows); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(rows)
		return 0
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "ID\tDATE\tSIDE\tSYMBOL\tQTY\tPRICE\tACCOUNT\tNOTE")
	for _, t := range rows {
		_, _ = fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\t%v\t%v\t%v\n",
			t["id"], t["occurred_at"], t["side"], t["asset_symbol"],
			t["qty"], t["price"], t["account_id"], t["note"])
	}
	_ = w.Flush()
	return 0
}

func cmdAssetLookup(cfg *config, args []string) int {
	var symbol, provider string
	asJSON, ok := parseShowFlags("asset-lookup", args, func(fs *flag.FlagSet) {
		fs.StringVar(&symbol, "symbol", "", "ticker (required)")
		fs.StringVar(&provider, "provider", "yahoo", "yahoo|coingecko")
	})
	if !ok {
		return 2
	}
	if symbol == "" {
		return errf("--symbol is required")
	}
	path := "/api/v1/assets/lookup?symbol=" + url.QueryEscape(symbol) +
		"&provider=" + url.QueryEscape(provider)
	var info map[string]any
	if err := apiGET(cfg, path, &info); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(info)
		return 0
	}
	fmt.Printf("symbol:      %v\n", info["symbol"])
	fmt.Printf("name:        %v\n", info["name"])
	fmt.Printf("type:        %v\n", info["type"])
	fmt.Printf("currency:    %v\n", info["currency"])
	fmt.Printf("provider:    %v\n", info["provider"])
	fmt.Printf("provider_id: %v\n", info["provider_id"])
	return 0
}

func cmdAssetPrice(cfg *config, args []string) int {
	var symbol string
	asJSON, ok := parseShowFlags("asset-price", args, func(fs *flag.FlagSet) {
		fs.StringVar(&symbol, "symbol", "", "ticker (required)")
	})
	if !ok {
		return 2
	}
	if symbol == "" {
		return errf("--symbol is required")
	}
	var p map[string]any
	if err := apiGET(cfg, "/api/v1/assets/"+url.PathEscape(symbol)+"/price", &p); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(p)
		return 0
	}
	stale := ""
	if v, ok := p["stale"].(bool); ok && v {
		stale = " (stale)"
	}
	fmt.Printf("symbol:   %v\n", p["symbol"])
	fmt.Printf("price:    %.4f %v%s\n", toFloat(p["price"]), p["currency"], stale)
	if v, ok := p["at"].(string); ok && v != "" {
		fmt.Printf("at:       %v\n", v)
	}
	return 0
}

func cmdFxRate(cfg *config, args []string) int {
	var from, to, at string
	asJSON, ok := parseShowFlags("fx-rate", args, func(fs *flag.FlagSet) {
		fs.StringVar(&from, "from", "", "source currency (required)")
		fs.StringVar(&to, "to", "", "target currency (required)")
		fs.StringVar(&at, "at", "", "YYYY-MM-DD (default: latest)")
	})
	if !ok {
		return 2
	}
	if from == "" || to == "" {
		return errf("--from and --to are required")
	}
	path := "/api/v1/fx/rate?from=" + url.QueryEscape(from) + "&to=" + url.QueryEscape(to)
	if at != "" {
		path += "&at=" + url.QueryEscape(at)
	}
	var r map[string]any
	if err := apiGET(cfg, path, &r); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(r)
		return 0
	}
	fmt.Printf("1 %v = %.6f %v", r["from"], toFloat(r["rate"]), r["to"])
	if v, ok := r["at"].(string); ok && v != "" {
		fmt.Printf(" (%s)", v)
	}
	fmt.Println()
	return 0
}

func cmdTxSummary(cfg *config, args []string) int {
	asJSON, ok := parseShowFlags("tx-summary", args, nil)
	if !ok {
		return 2
	}
	var s map[string]any
	if err := apiGET(cfg, "/api/v1/transactions/summary", &s); err != nil {
		return errf("%v", err)
	}
	if asJSON {
		printJSON(s)
		return 0
	}
	fmt.Printf("transactions: %v (%v assets, %v accounts)\n",
		s["count"], s["asset_count"], s["account_count"])
	fmt.Printf("buys:         %.2f (%v)\n", toFloat(s["total_buys"]), s["buy_count"])
	fmt.Printf("sells:        %.2f (%v)\n", toFloat(s["total_sells"]), s["sell_count"])
	fmt.Printf("deposits:     %.2f\n", toFloat(s["total_deposits"]))
	fmt.Printf("withdraws:    %.2f\n", toFloat(s["total_withdraws"]))
	fmt.Printf("interest:     %.2f\n", toFloat(s["total_interest"]))
	return 0
}

// cmdRefreshPrices triggers a server-side price + FX refresh. It only
// touches the global price cache (no user-data mutation), so it skips
// the --yes confirmation that the actual write commands require.
func cmdRefreshPrices(cfg *config, args []string) int {
	fs := flag.NewFlagSet("refresh-prices", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	var resp map[string]any
	if err := apiPOST(cfg, "/api/v1/prices/refresh", map[string]any{}, &resp); err != nil {
		return errf("%v", err)
	}
	fmt.Printf("prices refreshed (%v)\n", resp["status"])
	return 0
}

func cmdExport(cfg *config, args []string) int {
	var format, out string
	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	fs.StringVar(&format, "format", "json", "json|csv")
	fs.StringVar(&out, "out", "", "output file (default: stdout)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	body, err := apiGETStream(cfg, "/api/v1/export?format="+url.QueryEscape(format))
	if err != nil {
		return errf("%v", err)
	}
	defer func() { _ = body.Close() }()

	sink := io.Writer(os.Stdout)
	if out != "" {
		f, ferr := os.Create(out) //nolint:gosec // path comes from a CLI flag the user explicitly chose
		if ferr != nil {
			return errf("%v", ferr)
		}
		defer func() { _ = f.Close() }()
		sink = f
	}
	if _, err := io.Copy(sink, body); err != nil {
		return errf("%v", err)
	}
	if out != "" {
		fmt.Fprintf(os.Stderr, "wrote %s\n", out)
	}
	return 0
}

func toFloat(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int:
		return float64(x)
	case int64:
		return float64(x)
	default:
		return 0
	}
}
