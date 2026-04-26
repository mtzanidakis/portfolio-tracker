package api

import (
	"net/http"
	"strconv"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

type tokenListRow struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
}

type createTokenRequest struct {
	Name string `json:"name"`
}

// createTokenResponse carries the plaintext token — exposed exactly once,
// at creation time.
type createTokenResponse struct {
	tokenListRow
	Token string `json:"token"`
}

func listMyTokensHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		rows, err := d.ListTokens(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		out := make([]tokenListRow, 0, len(rows))
		for _, t := range rows {
			out = append(out, tokenListRow{
				ID: t.ID, Name: t.Name,
				CreatedAt:  t.CreatedAt,
				LastUsedAt: t.LastUsedAt,
				RevokedAt:  t.RevokedAt,
			})
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func createMyTokenHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var req createTokenRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if req.Name == "" {
			writeError(w, http.StatusBadRequest, "name is required")
			return
		}
		plain, hash, err := auth.GenerateToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "token error")
			return
		}
		tok := &domain.Token{UserID: u.ID, Name: req.Name, Hash: hash}
		if err := d.CreateToken(r.Context(), tok); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, createTokenResponse{
			tokenListRow: tokenListRow{
				ID: tok.ID, Name: tok.Name, CreatedAt: tok.CreatedAt,
			},
			Token: plain,
		})
	}
}

func revokeMyTokenHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		tok, err := d.GetToken(r.Context(), id)
		if err != nil || tok.UserID != u.ID {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if err := d.RevokeToken(r.Context(), id); err != nil {
			writeDBError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// deleteMyTokenHandler soft-deletes the row so it disappears from the
// user's list. Revoke + delete are separate gestures: revoke kills the
// credential while keeping the audit row; delete hides the row entirely.
func deleteMyTokenHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		tok, err := d.GetToken(r.Context(), id)
		if err != nil || tok.UserID != u.ID {
			writeError(w, http.StatusNotFound, "not found")
			return
		}
		if err := d.SoftDeleteToken(r.Context(), id); err != nil {
			writeDBError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
