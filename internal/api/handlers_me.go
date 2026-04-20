package api

import (
	"net/http"

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
		// Name updates not implemented yet; refuse silently by ignoring.
		updated, err := d.GetUser(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	}
}
