package db

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

const migrationsDir = "migrations"

// Migrate creates the schema_migrations table if missing, then applies any
// embedded SQL migrations whose version is not yet recorded. Migrations are
// applied in sorted filename order, each inside its own transaction.
func (db *DB) Migrate(ctx context.Context) error {
	if _, err := db.ExecContext(ctx, `
        CREATE TABLE IF NOT EXISTS schema_migrations (
            version    TEXT PRIMARY KEY,
            applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
        )`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := listMigrations()
	if err != nil {
		return err
	}
	for _, name := range names {
		applied, err := db.migrationApplied(ctx, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		if err := db.applyMigration(ctx, name); err != nil {
			return fmt.Errorf("apply %s: %w", name, err)
		}
	}
	return nil
}

func listMigrations() ([]string, error) {
	entries, err := fs.ReadDir(migrationsFS, migrationsDir)
	if err != nil {
		return nil, fmt.Errorf("read migrations: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}

func (db *DB) migrationApplied(ctx context.Context, name string) (bool, error) {
	var got string
	err := db.QueryRowContext(ctx,
		"SELECT version FROM schema_migrations WHERE version = ?", name,
	).Scan(&got)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return true, nil
}

func (db *DB) applyMigration(ctx context.Context, name string) error {
	body, err := fs.ReadFile(migrationsFS, path.Join(migrationsDir, name))
	if err != nil {
		return fmt.Errorf("read %s: %w", name, err)
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, string(body)); err != nil {
		return fmt.Errorf("exec: %w", err)
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations(version) VALUES(?)", name,
	); err != nil {
		return fmt.Errorf("record: %w", err)
	}
	return tx.Commit()
}
