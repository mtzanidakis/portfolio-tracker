package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"net/http"
	"strings"
	"time"
)

// Cookie and header names used for browser authentication.
const (
	SessionCookieName = "pt_session"
	CSRFCookieName    = "pt_csrf"
	CSRFHeaderName    = "X-CSRF-Token"
)

// NewSessionID produces a 32-byte random identifier, base64url-encoded.
func NewSessionID() (string, error) { return randomBase64URL(32) }

// NewCSRFToken produces a 32-byte random CSRF token, base64url-encoded.
func NewCSRFToken() (string, error) { return randomBase64URL(32) }

func randomBase64URL(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SetAuthCookies sets the session (HttpOnly, HMAC-signed with secret)
// and CSRF (JS-readable, unsigned — only its presence matters for the
// double-submit check) cookies on w. The Secure flag is auto-detected
// from the request.
func SetAuthCookies(w http.ResponseWriter, r *http.Request, secret []byte, sessionID, csrf string, expires time.Time) {
	secure := isSecureRequest(r)
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    SignCookie(secret, sessionID),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  expires,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     CSRFCookieName,
		Value:    csrf,
		Path:     "/",
		HttpOnly: false, // JS needs to read this
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
		Expires:  expires,
	})
}

// ClearAuthCookies invalidates both auth cookies.
func ClearAuthCookies(w http.ResponseWriter, r *http.Request) {
	secure := isSecureRequest(r)
	for _, c := range []struct {
		name     string
		httpOnly bool
	}{
		{SessionCookieName, true},
		{CSRFCookieName, false},
	} {
		http.SetCookie(w, &http.Cookie{
			Name:     c.name,
			Value:    "",
			Path:     "/",
			HttpOnly: c.httpOnly,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
			Expires:  time.Unix(0, 0),
			MaxAge:   -1,
		})
	}
}

func isSecureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		return true
	}
	return false
}

// --- session-id context plumbing (used by handlers that need to know
// which session to touch or delete, e.g., logout). ---

type sessionCtxKey struct{}

var sessionIDKey = sessionCtxKey{}

// WithSessionID attaches a session id to the context.
func WithSessionID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, sessionIDKey, id)
}

// SessionIDFrom returns the session id attached by Middleware (empty for
// Bearer-authenticated requests).
func SessionIDFrom(ctx context.Context) string {
	s, _ := ctx.Value(sessionIDKey).(string)
	return s
}
