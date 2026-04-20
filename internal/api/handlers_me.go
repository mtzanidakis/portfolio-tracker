package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func meHandler(*db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		writeJSON(w, http.StatusOK, u)
	}
}

type updateMeRequest struct {
	Name         string          `json:"name,omitempty"`
	Email        string          `json:"email,omitempty"`
	BaseCurrency domain.Currency `json:"base_currency,omitempty"`
}

func updateMeHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var req updateMeRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}

		// Email uniqueness pre-check (409 if taken by a different user).
		newEmail := strings.TrimSpace(req.Email)
		if newEmail != "" && newEmail != u.Email {
			existing, err := d.GetUserByEmail(r.Context(), newEmail)
			if err == nil && existing.ID != u.ID {
				writeError(w, http.StatusConflict, "email already in use")
				return
			}
			if err != nil && !errors.Is(err, db.ErrNotFound) {
				writeDBError(w, err)
				return
			}
		}

		if req.BaseCurrency != "" {
			if !req.BaseCurrency.Valid() {
				writeError(w, http.StatusBadRequest, "invalid base_currency")
				return
			}
			if err := d.UpdateUserBaseCurrency(r.Context(), u.ID, req.BaseCurrency); err != nil {
				writeDBError(w, err)
				return
			}
		}

		name := strings.TrimSpace(req.Name)
		if name != "" || newEmail != "" {
			if err := d.UpdateUserProfile(r.Context(), u.ID, name, newEmail); err != nil {
				writeDBError(w, err)
				return
			}
		}

		updated, err := d.GetUser(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	}
}
