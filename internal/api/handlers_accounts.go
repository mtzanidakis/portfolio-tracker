package api

import (
	"net/http"
	"strconv"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func listAccountsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		accs, err := d.ListAccounts(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, accs)
	}
}

type accountRequest struct {
	Name     string          `json:"name"`
	Type     string          `json:"type"`
	Short    string          `json:"short"`
	Color    string          `json:"color"`
	Currency domain.Currency `json:"currency"`
}

func createAccountHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var req accountRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		if !req.Currency.Valid() {
			writeError(w, http.StatusBadRequest, "invalid currency")
			return
		}
		acc := &domain.Account{
			UserID:   u.ID,
			Name:     req.Name,
			Type:     req.Type,
			Short:    req.Short,
			Color:    req.Color,
			Currency: req.Currency,
		}
		if err := d.CreateAccount(r.Context(), acc); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, acc)
	}
}

func getAccountHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		acc, err := loadOwnedAccount(r, d, u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, acc)
	}
}

func updateAccountHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		acc, err := loadOwnedAccount(r, d, u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		var req accountRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if req.Name != "" {
			acc.Name = req.Name
		}
		if req.Type != "" {
			acc.Type = req.Type
		}
		if req.Short != "" {
			acc.Short = req.Short
		}
		if req.Color != "" {
			acc.Color = req.Color
		}
		if req.Currency != "" {
			if !req.Currency.Valid() {
				writeError(w, http.StatusBadRequest, "invalid currency")
				return
			}
			acc.Currency = req.Currency
		}
		if err := d.UpdateAccount(r.Context(), acc); err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, acc)
	}
}

func deleteAccountHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		acc, err := loadOwnedAccount(r, d, u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if err := d.DeleteAccount(r.Context(), acc.ID); err != nil {
			writeDBError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// loadOwnedAccount fetches the account at {id} and verifies ownership.
func loadOwnedAccount(r *http.Request, d *db.DB, userID int64) (*domain.Account, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, db.ErrNotFound
	}
	acc, err := d.GetAccount(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if acc.UserID != userID {
		return nil, db.ErrNotFound
	}
	return acc, nil
}
