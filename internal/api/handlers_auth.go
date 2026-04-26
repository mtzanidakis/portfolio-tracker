package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// MinPasswordLen matches the client-side validation in the Preact
// LoginForm / password-change modal. Kept small here; the guidance is
// "length > complexity" per NIST SP 800-63B.
const MinPasswordLen = 8

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// loginHandler authenticates with email + password, issues a session,
// and sets the pt_session + pt_csrf cookies. Any failure (no such user,
// empty password_hash, wrong password) yields the same generic error to
// avoid leaking user existence.
func loginHandler(d *db.DB, lifetime time.Duration, secret []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		req.Email = strings.TrimSpace(req.Email)
		if req.Email == "" || req.Password == "" {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		u, err := d.GetUserByEmail(r.Context(), req.Email)
		if errors.Is(err, db.ErrNotFound) || err != nil {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		if !auth.VerifyPassword(req.Password, u.PasswordHash) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}

		sid, err := auth.NewSessionID()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "session error")
			return
		}
		csrf, err := auth.NewCSRFToken()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "session error")
			return
		}
		expires := time.Now().Add(lifetime)
		if err := d.CreateSession(r.Context(), &domain.Session{
			ID: sid, UserID: u.ID, ExpiresAt: expires,
		}); err != nil {
			writeError(w, http.StatusInternalServerError, "session error")
			return
		}
		auth.SetAuthCookies(w, r, secret, sid, csrf, expires)
		writeJSON(w, http.StatusOK, u)
	}
}

// logoutHandler deletes the current session (cookie path only) and
// clears both cookies. Bearer-authenticated callers simply get their
// cookies cleared (if any) — tokens are not affected.
func logoutHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sid := auth.SessionIDFrom(r.Context()); sid != "" {
			_ = d.DeleteSession(r.Context(), sid)
		}
		auth.ClearAuthCookies(w, r)
		w.WriteHeader(http.StatusNoContent)
	}
}

type changePasswordRequest struct {
	Current string `json:"current"`
	New     string `json:"new"`
}

// changePasswordHandler verifies the current password, re-hashes the
// new one, and (for session-authenticated callers) logs out every other
// session belonging to the user, keeping only the current one.
func changePasswordHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var req changePasswordRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body")
			return
		}
		if len(req.New) < MinPasswordLen {
			writeError(w, http.StatusBadRequest, "new password too short")
			return
		}
		if !auth.VerifyPassword(req.Current, u.PasswordHash) {
			writeError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		hash, err := auth.HashPassword(req.New)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "hash error")
			return
		}
		if err := d.UpdateUserPassword(r.Context(), u.ID, hash); err != nil {
			writeDBError(w, err)
			return
		}
		// Kill other sessions but keep the current one.
		if sid := auth.SessionIDFrom(r.Context()); sid != "" {
			_ = d.DeleteUserSessionsExcept(r.Context(), u.ID, sid)
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
