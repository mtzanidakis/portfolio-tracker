package db

import (
	"errors"
	"testing"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func TestAccountCRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := t.Context()
	u := mustCreateUser(t, db, "acc@test.io")

	acc := &domain.Account{
		UserID: u.ID, Name: "Ember", Type: "Brokerage", Short: "EB",
		Color: "#c8502a", Currency: domain.USD,
	}
	if err := db.CreateAccount(ctx, acc); err != nil {
		t.Fatalf("create: %v", err)
	}

	got, err := db.GetAccount(ctx, acc.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Currency != domain.USD {
		t.Errorf("mismatch: %+v", got)
	}

	got.Name = "Ember Brokerage"
	if err := db.UpdateAccount(ctx, got); err != nil {
		t.Fatalf("update: %v", err)
	}
	again, _ := db.GetAccount(ctx, acc.ID)
	if again.Name != "Ember Brokerage" {
		t.Errorf("update not persisted: %+v", again)
	}

	list, err := db.ListAccounts(ctx, u.ID)
	if err != nil || len(list) != 1 {
		t.Errorf("list: %d, err=%v", len(list), err)
	}

	if err := db.DeleteAccount(ctx, acc.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := db.GetAccount(ctx, acc.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateAccount_InvalidCurrency(t *testing.T) {
	db := newTestDB(t)
	u := mustCreateUser(t, db, "bad@test.io")
	acc := &domain.Account{
		UserID: u.ID, Name: "X", Type: "T", Short: "X",
		Color: "#000", Currency: domain.Currency("XYZ"),
	}
	if err := db.CreateAccount(t.Context(), acc); err == nil {
		t.Fatal("expected error")
	}
}
