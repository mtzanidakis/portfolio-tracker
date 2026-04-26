// ptadmin is the administrative CLI for portfolio-tracker: user and token
// management, plus database operations. It talks directly to the SQLite
// database and does not require the HTTP server to be running.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/version"
)

const usageText = `ptadmin — portfolio-tracker administration

Usage:
  ptadmin <command> [flags]

Commands:
  user add       --email EMAIL --name NAME [--base-currency CODE]
                 [--password PW | --no-password]   (prompts interactively
                 if neither flag is given)
  user password  --email EMAIL                     (prompts; or --password PW)
  user list
  user delete    (--id ID | --email EMAIL)

  token create   --user EMAIL --name NAME
  token list     [--user EMAIL]
  token revoke   --id ID
  token delete   --id ID

  db migrate
  db backup      --to PATH

  version

Environment:
  PT_DB   path to sqlite file (default: ./data/pt.db)
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
		fmt.Println("ptadmin", version.Version)
		return 0
	case "-h", "--help":
		fmt.Print(usageText)
		return 0
	}

	ctx := context.Background()
	dbPath := envOr("PT_DB", "./data/pt.db")

	conn, err := db.Open(ctx, dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ptadmin: open db: %v\n", err)
		return 1
	}
	defer func() { _ = conn.Close() }()
	if err := conn.Migrate(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "ptadmin: migrate: %v\n", err)
		return 1
	}

	cmd, sub, rest := os.Args[1], "", os.Args[2:]
	if len(rest) > 0 {
		sub, rest = rest[0], rest[1:]
	}

	switch cmd {
	case "user":
		return runUser(ctx, conn, sub, rest)
	case "token":
		return runToken(ctx, conn, sub, rest)
	case "db":
		return runDB(ctx, conn, sub, rest, dbPath)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n\n", cmd)
		fmt.Fprint(os.Stderr, usageText)
		return 2
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func errf(format string, args ...any) int {
	fmt.Fprintf(os.Stderr, "ptadmin: "+format+"\n", args...)
	return 1
}
