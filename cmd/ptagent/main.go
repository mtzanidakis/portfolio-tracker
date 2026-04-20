// ptagent is a thin HTTP client for the portfolio-tracker API, intended
// to be driven by a Claude Code skill (see skill/SKILL.md). Read commands
// work by default; write commands require --yes.
//
// Configuration is via environment:
//
//	PT_API_URL  base URL (default: http://localhost:8082)
//	PT_TOKEN    required Bearer token from `ptadmin token create`
package main

import (
	"fmt"
	"os"

	"github.com/mtzanidakis/portfolio-tracker/internal/version"
)

const usageText = `ptagent — portfolio-tracker API client

Usage:
  ptagent <command> [flags]

Read commands:
  me
  holdings
  performance   [--tf 1D|1W|1M|3M|6M|1Y|ALL]
  allocations   [--group asset|type|account]
  accounts
  assets        [--q SEARCH]
  transactions  [--symbol SYM] [--side buy|sell] [--limit N]

Write commands (require --yes):
  add-tx --account-id ID --symbol SYM --side buy|sell --qty N --price N
         [--fee N] [--fx N] [--date YYYY-MM-DD] [--note TXT] --yes
  add-account --name NAME --type TYPE --short XX --color #RRGGBB
              --currency USD|EUR|... [--connected] --yes
  add-asset --symbol SYM --name NAME --type stock|etf|crypto|cash
            --currency USD|EUR|... [--provider P] [--provider-id ID] --yes
  delete-tx --id ID --yes
  delete-account --id ID --yes
  set-base-currency --currency USD|EUR|... --yes

Global flags:
  --json        output raw JSON (default: human table)

Environment:
  PT_API_URL   API base (default http://localhost:8082)
  PT_TOKEN     required Bearer token
`

func main() {
	os.Exit(run())
}

func run() int {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
	switch os.Args[1] {
	case "version":
		fmt.Println("ptagent", version.Version)
		return 0
	case "-h", "--help":
		fmt.Print(usageText)
		return 0
	}

	cfg, err := loadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptagent: %v\n", err)
		return 1
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "me":
		return cmdMe(cfg, args)
	case "holdings":
		return cmdHoldings(cfg, args)
	case "performance":
		return cmdPerformance(cfg, args)
	case "allocations":
		return cmdAllocations(cfg, args)
	case "accounts":
		return cmdAccounts(cfg, args)
	case "assets":
		return cmdAssets(cfg, args)
	case "transactions":
		return cmdTransactions(cfg, args)

	case "add-tx":
		return cmdAddTx(cfg, args)
	case "add-account":
		return cmdAddAccount(cfg, args)
	case "add-asset":
		return cmdAddAsset(cfg, args)
	case "delete-tx":
		return cmdDeleteTx(cfg, args)
	case "delete-account":
		return cmdDeleteAccount(cfg, args)
	case "set-base-currency":
		return cmdSetBaseCurrency(cfg, args)

	default:
		fmt.Fprintf(os.Stderr, "ptagent: unknown command %q\n\n", cmd)
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
}

func errf(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "ptagent: "+format+"\n", args...)
	return 1
}
