package main

import (
	"flag"
	"fmt"
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
	_, _ = fmt.Fprintln(w, "ID\tNAME\tTYPE\tCCY\tCONNECTED")
	for _, a := range rows {
		_, _ = fmt.Fprintf(w, "%v\t%v\t%v\t%v\t%v\n",
			a["id"], a["name"], a["type"], a["currency"], a["connected"])
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
