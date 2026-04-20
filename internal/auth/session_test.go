package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewSessionID_UniqueAndB64(t *testing.T) {
	a, err := NewSessionID()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := NewSessionID()
	if a == b {
		t.Error("session ids should differ")
	}
	if len(a) < 40 { // 32 bytes → 43 chars b64url (no padding)
		t.Errorf("id length %d", len(a))
	}
}

func TestSetAuthCookies_Structure(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example/", nil)
	SetAuthCookies(w, r, "sess-1", "csrf-1", time.Now().Add(time.Hour))

	found := map[string]*http.Cookie{}
	for _, c := range w.Result().Cookies() {
		found[c.Name] = c
	}
	s := found[SessionCookieName]
	if s == nil || s.Value != "sess-1" {
		t.Fatalf("session cookie: %+v", s)
	}
	if !s.HttpOnly {
		t.Error("session cookie must be HttpOnly")
	}
	if s.SameSite != http.SameSiteLaxMode {
		t.Errorf("session SameSite: %v", s.SameSite)
	}

	c := found[CSRFCookieName]
	if c == nil || c.Value != "csrf-1" {
		t.Fatalf("csrf cookie: %+v", c)
	}
	if c.HttpOnly {
		t.Error("csrf cookie must NOT be HttpOnly (JS reads it)")
	}
}

func TestClearAuthCookies_UsesMaxAgeMinus1(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example/", nil)
	ClearAuthCookies(w, r)

	for _, c := range w.Result().Cookies() {
		if c.MaxAge != -1 {
			t.Errorf("%s MaxAge=%d, want -1", c.Name, c.MaxAge)
		}
	}
}

func TestSessionID_ContextRoundtrip(t *testing.T) {
	ctx := WithSessionID(context.Background(), "xyz")
	if got := SessionIDFrom(ctx); got != "xyz" {
		t.Errorf("got %q", got)
	}
	if SessionIDFrom(context.Background()) != "" {
		t.Error("empty context should yield empty id")
	}
}

func TestIsSecureRequest(t *testing.T) {
	r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "http://example/", nil)
	if isSecureRequest(r) {
		t.Error("plain http should not be secure")
	}
	r.Header.Set("X-Forwarded-Proto", "https")
	if !isSecureRequest(r) {
		t.Error("X-Forwarded-Proto=https should flag secure")
	}
}
