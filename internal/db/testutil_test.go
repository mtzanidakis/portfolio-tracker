package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// newTestDB spins up a fresh, migrated SQLite database in a temp directory
// and registers cleanup on the test.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	db, err := Open(t.Context(), path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := db.Migrate(t.Context()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

// mustCreateUser creates a user and returns it, failing the test on error.
func mustCreateUser(t *testing.T, db *DB, email string) *domain.User {
	t.Helper()
	u := &domain.User{Email: email, Name: "Test", BaseCurrency: domain.EUR}
	if err := db.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	return u
}

// mustCreateAsset inserts an asset and returns it.
func mustCreateAsset(t *testing.T, db *DB, symbol string, typ domain.AssetType, cur domain.Currency) *domain.Asset {
	t.Helper()
	a := &domain.Asset{Symbol: symbol, Name: symbol, Type: typ, Currency: cur}
	if err := db.UpsertAsset(t.Context(), a); err != nil {
		t.Fatalf("upsert asset: %v", err)
	}
	return a
}

// mustCreateAccount inserts an account and returns it.
func mustCreateAccount(t *testing.T, db *DB, userID int64, cur domain.Currency) *domain.Account {
	t.Helper()
	acc := &domain.Account{
		UserID:   userID,
		Name:     "Test Account",
		Type:     "Brokerage",
		Short:    "TA",
		Color:    "#000000",
		Currency: cur,
	}
	if err := db.CreateAccount(t.Context(), acc); err != nil {
		t.Fatalf("create account: %v", err)
	}
	return acc
}

func mustTime(t *testing.T, s string) time.Time {
	t.Helper()
	tm, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse time: %v", err)
	}
	return tm
}
