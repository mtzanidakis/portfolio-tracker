package db

import (
	"errors"
	"testing"
	"time"

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

func TestTokenExpiry_FiltersBearerLookup(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "exp@test.io")

	past := time.Now().Add(-time.Minute)
	future := time.Now().Add(time.Hour)

	expired := &domain.Token{UserID: u.ID, Name: "old", Hash: "h-expired", ExpiresAt: &past}
	if err := db.CreateToken(ctx, expired); err != nil {
		t.Fatalf("create expired: %v", err)
	}
	live := &domain.Token{UserID: u.ID, Name: "live", Hash: "h-live", ExpiresAt: &future}
	if err := db.CreateToken(ctx, live); err != nil {
		t.Fatalf("create live: %v", err)
	}
	never := &domain.Token{UserID: u.ID, Name: "forever", Hash: "h-forever"}
	if err := db.CreateToken(ctx, never); err != nil {
		t.Fatalf("create never: %v", err)
	}

	// Auth path rejects the expired hash but accepts the others.
	if _, err := db.GetTokenByHash(ctx, "h-expired"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired hash should be ErrNotFound, got %v", err)
	}
	got, err := db.GetTokenByHash(ctx, "h-live")
	if err != nil || got == nil || got.ExpiresAt == nil {
		t.Errorf("live: err=%v tok=%+v", err, got)
	}
	got, err = db.GetTokenByHash(ctx, "h-forever")
	if err != nil || got == nil || got.ExpiresAt != nil {
		t.Errorf("never: err=%v tok=%+v", err, got)
	}

	// Management list still returns all three so the UI can render
	// "Expired" status alongside the others.
	all, err := db.ListTokens(ctx, u.ID)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("ListTokens: got %d, want 3", len(all))
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
