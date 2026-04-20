package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
)

func runDB(ctx context.Context, conn *db.DB, sub string, args []string, dbPath string) int {
	switch sub {
	case "migrate":
		return dbMigrate(ctx, conn)
	case "backup":
		return dbBackup(ctx, conn, args, dbPath)
	default:
		return errf("db: unknown subcommand %q", sub)
	}
}

func dbMigrate(ctx context.Context, conn *db.DB) int {
	// main() already ran Migrate on Open. Running again confirms idempotency.
	if err := conn.Migrate(ctx); err != nil {
		return errf("db migrate: %v", err)
	}
	fmt.Println("migrations up-to-date")
	return 0
}

func dbBackup(ctx context.Context, conn *db.DB, args []string, dbPath string) int {
	fs := flag.NewFlagSet("db backup", flag.ContinueOnError)
	to := fs.String("to", "", "destination file (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *to == "" {
		return errf("db backup: --to is required")
	}
	// VACUUM INTO produces a consistent, compact snapshot.
	if _, err := conn.ExecContext(ctx, "VACUUM INTO ?", *to); err != nil {
		return errf("db backup: %v", err)
	}
	fmt.Printf("backup of %s written to %s\n", dbPath, *to)
	return 0
}
