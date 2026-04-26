// Package auth provides token generation, hashing, and HTTP middleware
// for both Bearer-token and session-cookie authentication.
//
// Browsers authenticate with a password (see password.go) and a server
// session (see session.go), protected against CSRF by the double-submit
// cookie pattern. External CLIs (ptagent) authenticate with Bearer
// tokens. The Middleware accepts either: Bearer wins when present.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

const (
	tokenEntropyBytes = 32
	// TokenPrefix tags Bearer tokens so they're recognisable in logs,
	// secret scanners, and clipboards. The hash is over the full
	// prefixed string, so old un-prefixed tokens stop validating.
	TokenPrefix = "pt_"
)

// GenerateToken returns a new (plaintext, hash) pair for an API token.
// The plaintext must be returned to the user exactly once; the hash is
// what gets persisted.
func GenerateToken() (plaintext, hash string, err error) {
	b := make([]byte, tokenEntropyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plaintext = TokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	hash = HashToken(plaintext)
	return plaintext, hash, nil
}

// HashToken computes the stored form of a plaintext API token.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// --- user context helpers ---

type userCtxKeyType struct{}

var userCtxKey = userCtxKeyType{}

// WithUser attaches a user to the context.
func WithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// UserFrom returns the user previously attached via WithUser, or nil.
func UserFrom(ctx context.Context) *domain.User {
	u, _ := ctx.Value(userCtxKey).(*domain.User)
	return u
}

// --- middleware ---

// Middleware enforces authentication on the wrapped handler. It accepts
// either an Authorization: Bearer token (API clients) or a pt_session
// cookie plus a matching X-CSRF-Token header for state-changing methods
// (browser clients). The session cookie is HMAC-signed with Secret —
// tampered or unsigned cookies are rejected before the DB lookup.
type Middleware struct {
	DB              *db.DB
	SessionLifetime time.Duration
	Secret          []byte
	// LastUsed throttles per-token last_used_at writes so a busy CLI
	// doesn't hammer the database. nil = update on every request
	// (legacy behaviour, fine for tests).
	LastUsed *LastUsedThrottler
}

// touchToken updates last_used_at for the given token id, throttled if
// the middleware has a LastUsedThrottler configured. Throttled writes
// run in a goroutine — the request shouldn't wait on a non-essential
// audit field.
func (m *Middleware) touchToken(ctx context.Context, id int64) {
	if m.LastUsed == nil {
		_ = m.DB.TouchToken(ctx, id)
		return
	}
	if !m.LastUsed.Allow(id, time.Now()) {
		return
	}
	// Detach from the request context so a quick client disconnect
	// doesn't cancel the write mid-flight.
	go func() { _ = m.DB.TouchToken(context.Background(), id) }() //nolint:gosec // intentional fire-and-forget audit write
}

// Handler wraps next with auth enforcement.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Bearer path: API clients.
		if plaintext := extractBearer(r); plaintext != "" {
			tok, err := m.DB.GetTokenByHash(r.Context(), HashToken(plaintext))
			if errors.Is(err, db.ErrNotFound) {
				http.Error(w, "invalid token", http.StatusUnauthorized)
				return
			}
			if err != nil {
				http.Error(w, "auth error", http.StatusInternalServerError)
				return
			}
			u, err := m.DB.GetUser(r.Context(), tok.UserID)
			if err != nil {
				http.Error(w, "auth error", http.StatusInternalServerError)
				return
			}
			m.touchToken(r.Context(), tok.ID)
			next.ServeHTTP(w, r.WithContext(WithUser(r.Context(), u)))
			return
		}

		// 2. Cookie path: browser clients.
		sc, err := r.Cookie(SessionCookieName)
		if err != nil || sc.Value == "" {
			http.Error(w, "missing credentials", http.StatusUnauthorized)
			return
		}
		sid, ok := VerifyCookie(m.Secret, sc.Value)
		if !ok {
			http.Error(w, "invalid session", http.StatusUnauthorized)
			return
		}
		session, err := m.DB.GetSession(r.Context(), sid)
		if errors.Is(err, db.ErrNotFound) {
			http.Error(w, "invalid session", http.StatusUnauthorized)
			return
		}
		if err != nil {
			http.Error(w, "auth error", http.StatusInternalServerError)
			return
		}

		// 3. CSRF double-submit check for unsafe methods.
		if !isSafeMethod(r.Method) && !checkCSRF(r) {
			http.Error(w, "CSRF check failed", http.StatusForbidden)
			return
		}

		// 4. Slide session expiry, attach user + session id.
		if m.SessionLifetime > 0 {
			_ = m.DB.TouchSession(r.Context(), session.ID, time.Now().Add(m.SessionLifetime))
		}
		u, err := m.DB.GetUser(r.Context(), session.UserID)
		if err != nil {
			http.Error(w, "auth error", http.StatusInternalServerError)
			return
		}
		ctx := WithSessionID(WithUser(r.Context(), u), session.ID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func extractBearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if !strings.HasPrefix(h, prefix) {
		return ""
	}
	return strings.TrimPrefix(h, prefix)
}

func isSafeMethod(m string) bool {
	return m == http.MethodGet || m == http.MethodHead || m == http.MethodOptions
}

func checkCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(CSRFCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	header := r.Header.Get(CSRFHeaderName)
	if header == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(header), []byte(cookie.Value)) == 1
}
