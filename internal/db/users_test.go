package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestUserCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()

	u := &domain.User{Email: "a@b.io", Name: "Alice", BaseCurrency: domain.EUR}
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
