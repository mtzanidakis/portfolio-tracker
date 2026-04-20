package db

import (
	"path/filepath"
	"testing"
)

func TestOpenAndMigrate(t *testing.T) {
	db := newTestDB(t)

	// schema_migrations should now contain all embedded migration filenames.
	names, err := listMigrations()
	if err != nil {
		t.Fatalf("list migrations: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("expected at least one migration on disk")
	}

	for _, n := range names {
		applied, err := db.migrationApplied(t.Context(), n)
		if err != nil {
			t.Fatalf("check applied: %v", err)
		}
		if !applied {
			t.Errorf("migration %s not marked applied", n)
		}
	}
}

func TestMigrateIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	// Running again must not error.
	if err := db.Migrate(t.Context()); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestOpenInvalidPath(t *testing.T) {
	// A path inside a non-existent directory should fail on ping.
	bogus := filepath.Join(t.TempDir(), "does-not-exist", "x.db")
	_, err := Open(t.Context(), bogus)
	if err == nil {
		t.Fatal("expected error opening invalid path")
	}
}
