package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	u := &domain.User{
		Email: "a@b.io", Name: "Alice",
		PasswordHash: "hash-abc", BaseCurrency: domain.EUR,
	}
	if err := db.CreateUser(ctx, u); err != nil {
		t.Fatalf("create: %v", err)
	}
	if u.ID == 0 || u.CreatedAt.IsZero() {
		t.Fatal("expected ID and CreatedAt to be populated")
	}

	got, err := db.GetUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Email != u.Email || got.BaseCurrency != domain.EUR {
		t.Errorf("mismatch: %+v", got)
	}
	if got.PasswordHash != "hash-abc" {
		t.Errorf("password hash lost: %q", got.PasswordHash)
	}

	byEmail, err := db.GetUserByEmail(ctx, u.Email)
	if err != nil || byEmail.ID != u.ID {
		t.Errorf("GetUserByEmail: %+v, err=%v", byEmail, err)
	}

	if err := db.UpdateUserBaseCurrency(ctx, u.ID, domain.USD); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ = db.GetUser(ctx, u.ID)
	if got.BaseCurrency != domain.USD {
		t.Errorf("base_currency not updated: %s", got.BaseCurrency)
	}

	users, err := db.ListUsers(ctx)
	if err != nil || len(users) != 1 {
		t.Errorf("list: %d users, err=%v", len(users), err)
	}

	if err := db.DeleteUser(ctx, u.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetUser(ctx, u.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateUser_InvalidCurrency(t *testing.T) {
	db := newTestDB(t)
	u := &domain.User{Email: "x@y.io", Name: "X", BaseCurrency: domain.Currency("XYZ")}
	if err := db.CreateUser(t.Context(), u); err == nil {
		t.Fatal("expected error for invalid currency")
	}
}

func TestGetUser_NotFound(t *testing.T) {
	db := newTestDB(t)
	if _, err := db.GetUser(t.Context(), 999); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateBaseCurrency_NotFound(t *testing.T) {
	db := newTestDB(t)
	err := db.UpdateUserBaseCurrency(t.Context(), 42, domain.USD)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestUpdateUserProfile(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "prof@test.io")

	if err := db.UpdateUserProfile(ctx, u.ID, "New Name", "new@test.io"); err != nil {
		t.Fatalf("both: %v", err)
	}
	got, _ := db.GetUser(ctx, u.ID)
	if got.Name != "New Name" || got.Email != "new@test.io" {
		t.Errorf("both not updated: %+v", got)
	}

	// Partial updates.
	_ = db.UpdateUserProfile(ctx, u.ID, "Solo Name", "")
	got, _ = db.GetUser(ctx, u.ID)
	if got.Name != "Solo Name" || got.Email != "new@test.io" {
		t.Errorf("name-only: %+v", got)
	}
	_ = db.UpdateUserProfile(ctx, u.ID, "", "solo@test.io")
	got, _ = db.GetUser(ctx, u.ID)
	if got.Name != "Solo Name" || got.Email != "solo@test.io" {
		t.Errorf("email-only: %+v", got)
	}

	// No-op with both empty.
	if err := db.UpdateUserProfile(ctx, u.ID, "", ""); err != nil {
		t.Errorf("noop error: %v", err)
	}
}

func TestUpdateUserPassword(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "pw@test.io")

	if err := db.UpdateUserPassword(ctx, u.ID, "new-hash"); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := db.GetUser(ctx, u.ID)
	if got.PasswordHash != "new-hash" {
		t.Errorf("hash: %q", got.PasswordHash)
	}
}

func TestUpdateUserPassword_NotFound(t *testing.T) {
	db := newTestDB(t)
	if err := db.UpdateUserPassword(t.Context(), 999, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}
