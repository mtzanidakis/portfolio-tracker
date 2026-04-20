package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestTokenLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "tok@test.io")

	tok := &domain.Token{UserID: u.ID, Name: "cli", Hash: "deadbeef"}
	if err := db.CreateToken(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}
	if tok.ID == 0 {
		t.Fatal("ID not set")
	}

	got, err := db.GetTokenByHash(ctx, "deadbeef")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("user mismatch: %d", got.UserID)
	}

	if err := db.TouchToken(ctx, tok.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}
	got, _ = db.GetToken(ctx, tok.ID)
	if got.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after touch")
	}

	toks, err := db.ListTokens(ctx, u.ID)
	if err != nil || len(toks) != 1 {
		t.Errorf("list: got %d, err=%v", len(toks), err)
	}

	if err := db.RevokeToken(ctx, tok.ID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// Revoked tokens disappear from GetTokenByHash.
	if _, err := db.GetTokenByHash(ctx, "deadbeef"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound post-revoke, got %v", err)
	}
	// But GetToken by id still works.
	got, _ = db.GetToken(ctx, tok.ID)
	if got.RevokedAt == nil {
		t.Error("RevokedAt should be set")
	}
}

func TestRevokeToken_NotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.RevokeToken(t.Context(), 9999); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateToken_DuplicateHash(t *testing.T) {
	db := newTestDB(t)
	u := mustCreateUser(t, db, "dup@test.io")

	_ = db.CreateToken(t.Context(), &domain.Token{UserID: u.ID, Name: "a", Hash: "xyz"})
	err := db.CreateToken(t.Context(), &domain.Token{UserID: u.ID, Name: "b", Hash: "xyz"})
	if err == nil {
		t.Fatal("expected unique constraint error")
	}
}
