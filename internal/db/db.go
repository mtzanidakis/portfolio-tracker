// Package db provides the SQLite connection, schema migrations, and
// repository functions for portfolio tracker entities.
package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	_ "modernc.org/sqlite" // pure-Go SQLite driver (CGO-free)
)

// ErrNotFound is returned when a lookup does not match a row.
var ErrNotFound = errors.New("not found")

// DB wraps *sql.DB with repository methods.
type DB struct {
	*sql.DB
}

// Open opens a SQLite database at the given path (or ":memory:") and applies
// recommended pragmas. The caller is responsible for invoking Migrate.
func Open(ctx context.Context, path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := conn.PingContext(ctx); err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	pragmas := []string{
		"PRAGMA foreign_keys = ON",
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA synchronous = NORMAL",
	}
	for _, p := range pragmas {
		if _, err := conn.ExecContext(ctx, p); err != nil {
			_ = conn.Close()
			return nil, fmt.Errorf("pragma %q: %w", p, err)
		}
	}

	// SQLite works best with a single writer; keep the pool tight.
	conn.SetMaxOpenConns(1)
	conn.SetMaxIdleConns(1)

	return &DB{conn}, nil
}
