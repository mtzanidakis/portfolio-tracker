package db

import (
	"errors"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestSessionLifecycle(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "sess@test.io")

	s := &domain.Session{
		ID:        "session-abc",
		UserID:    u.ID,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := db.CreateSession(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.CreatedAt.IsZero() {
		t.Fatal("CreatedAt not populated")
	}

	got, err := db.GetSession(ctx, s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.UserID != u.ID {
		t.Errorf("user id mismatch: %d", got.UserID)
	}

	// Touch extends expiry.
	newExp := time.Now().Add(48 * time.Hour)
	if err := db.TouchSession(ctx, s.ID, newExp); err != nil {
		t.Fatalf("touch: %v", err)
	}
	got, _ = db.GetSession(ctx, s.ID)
	if got.LastUsedAt == nil {
		t.Error("LastUsedAt should be set after touch")
	}
	if got.ExpiresAt.Before(newExp.Add(-time.Second)) {
		t.Errorf("expires_at not extended: %v", got.ExpiresAt)
	}

	// Delete.
	if err := db.DeleteSession(ctx, s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetSession(ctx, s.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestGetSession_ExpiredIsNotFound(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "exp@test.io")

	s := &domain.Session{
		ID:        "expired",
		UserID:    u.ID,
		ExpiresAt: time.Now().Add(-time.Minute),
	}
	if err := db.CreateSession(ctx, s); err != nil {
		t.Fatalf("create: %v", err)
	}
	if _, err := db.GetSession(ctx, s.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected expired session to be hidden, got %v", err)
	}
}

func TestDeleteUserSessionsExcept(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "multi@test.io")

	future := time.Now().Add(time.Hour)
	for _, id := range []string{"keep", "drop1", "drop2"} {
		if err := db.CreateSession(ctx, &domain.Session{
			ID: id, UserID: u.ID, ExpiresAt: future,
		}); err != nil {
			t.Fatalf("seed %s: %v", id, err)
		}
	}
	if err := db.DeleteUserSessionsExcept(ctx, u.ID, "keep"); err != nil {
		t.Fatalf("delete except: %v", err)
	}
	if _, err := db.GetSession(ctx, "keep"); err != nil {
		t.Errorf("keep removed: %v", err)
	}
	for _, id := range []string{"drop1", "drop2"} {
		if _, err := db.GetSession(ctx, id); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s not removed: %v", id, err)
		}
	}
}

func TestPurgeExpiredSessions(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "purge@test.io")

	_ = db.CreateSession(ctx, &domain.Session{
		ID: "alive", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour),
	})
	_ = db.CreateSession(ctx, &domain.Session{
		ID: "dead1", UserID: u.ID, ExpiresAt: time.Now().Add(-time.Minute),
	})
	_ = db.CreateSession(ctx, &domain.Session{
		ID: "dead2", UserID: u.ID, ExpiresAt: time.Now().Add(-time.Hour),
	})

	n, err := db.PurgeExpiredSessions(ctx)
	if err != nil {
		t.Fatalf("purge: %v", err)
	}
	if n != 2 {
		t.Errorf("purged %d, want 2", n)
	}
}
