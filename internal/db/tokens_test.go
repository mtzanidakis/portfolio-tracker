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

func TestSoftDeleteToken_HidesFromReads(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "del@test.io")

	tok := &domain.Token{UserID: u.ID, Name: "del", Hash: "tobedeleted"}
	if err := db.CreateToken(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}

	if err := db.SoftDeleteToken(ctx, tok.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	// Hash lookup is dead — the credential can no longer authenticate.
	if _, err := db.GetTokenByHash(ctx, "tobedeleted"); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTokenByHash post-delete: got %v", err)
	}
	// Direct id lookup also hides deleted rows.
	if _, err := db.GetToken(ctx, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetToken post-delete: got %v", err)
	}
	// And it's gone from the user's list.
	toks, err := db.ListTokens(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(toks) != 0 {
		t.Errorf("ListTokens should hide deleted rows, got %d", len(toks))
	}

	// Re-deleting signals "already gone" with ErrNotFound.
	if err := db.SoftDeleteToken(ctx, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("re-delete: got %v", err)
	}
}

func TestRevokeToken_DeletedIsNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "rev-del@test.io")
	tok := &domain.Token{UserID: u.ID, Name: "x", Hash: "h"}
	if err := db.CreateToken(ctx, tok); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := db.SoftDeleteToken(ctx, tok.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := db.RevokeToken(ctx, tok.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke of deleted: got %v", err)
	}
}
