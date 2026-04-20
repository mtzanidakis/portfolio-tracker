// Package api exposes the v1 HTTP handlers backing the portfolio-tracker
// server. Handlers are pure adapters over internal/db + internal/portfolio
// + internal/auth; they perform no I/O beyond DB reads and writes.
package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
)

// writeJSON encodes v as JSON and writes it with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes an {"error": msg} body with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// writeDBError maps common DB errors to HTTP status codes.
func writeDBError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, db.ErrNotFound):
		writeError(w, http.StatusNotFound, "not found")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}

// decodeJSON reads r.Body into v; returns a 400 error helper on failure.
func decodeJSON(r *http.Request, v any) error {
	return json.NewDecoder(r.Body).Decode(v)
}
