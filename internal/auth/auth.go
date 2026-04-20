// Package auth provides token generation, hashing, and HTTP middleware
// for Bearer-token authentication against the tracker's tokens table.
//
// Tokens are 256-bit random values, base64url-encoded for transport.
// Only the SHA-256 hash of the token is stored in the database; the raw
// value is shown to the user exactly once at creation time.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

const tokenEntropyBytes = 32

// GenerateToken returns a new (plaintext, hash) pair. The plaintext must be
// returned to the user exactly once; the hash is what gets persisted.
func GenerateToken() (plaintext, hash string, err error) {
	b := make([]byte, tokenEntropyBytes)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	plaintext = base64.RawURLEncoding.EncodeToString(b)
	hash = HashToken(plaintext)
	return plaintext, hash, nil
}

// HashToken computes the stored form of a plaintext token.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// --- context helpers ---

type ctxKey struct{}

var userCtxKey = ctxKey{}

// WithUser attaches a user to the context (used by Middleware).
func WithUser(ctx context.Context, u *domain.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// UserFrom returns the user previously attached via WithUser, or nil.
func UserFrom(ctx context.Context) *domain.User {
	u, _ := ctx.Value(userCtxKey).(*domain.User)
	return u
}

// --- middleware ---

// Middleware enforces Bearer-token authentication. On success it injects
// the authenticated *domain.User into the request context.
type Middleware struct {
	DB *db.DB
}

// Handler wraps next with auth enforcement.
func (m *Middleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plaintext := extractBearer(r)
		if plaintext == "" {
			http.Error(w, "missing token", http.StatusUnauthorized)
			return
		}
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
		// Best-effort touch; ignore error.
		_ = m.DB.TouchToken(r.Context(), tok.ID)

		r = r.WithContext(WithUser(r.Context(), u))
		next.ServeHTTP(w, r)
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
