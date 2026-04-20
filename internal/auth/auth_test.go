package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

func newDB(t *testing.T) *db.DB {
	t.Helper()
	d, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "auth.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(t.Context()); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return d
}

func TestGenerateTokenIsDeterministicHash(t *testing.T) {
	p, h, err := GenerateToken()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if p == "" || h == "" {
		t.Fatal("empty token/hash")
	}
	if HashToken(p) != h {
		t.Error("HashToken disagreed with GenerateToken's hash")
	}
}

func TestGenerateTokensAreUnique(t *testing.T) {
	seen := make(map[string]bool)
	for range 100 {
		p, _, err := GenerateToken()
		if err != nil {
			t.Fatal(err)
		}
		if seen[p] {
			t.Fatalf("duplicate token: %s", p)
		}
		seen[p] = true
	}
}

func seedUserToken(t *testing.T, d *db.DB) (*domain.User, string) {
	t.Helper()
	u := &domain.User{Email: "a@b.io", Name: "A", BaseCurrency: domain.EUR}
	if err := d.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	plain, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	if err := d.CreateToken(t.Context(), &domain.Token{
		UserID: u.ID, Name: "test", Hash: hash,
	}); err != nil {
		t.Fatalf("create token: %v", err)
	}
	return u, plain
}

func seedUserSession(t *testing.T, d *db.DB, lifetime time.Duration) (*domain.User, string, string) {
	t.Helper()
	u := &domain.User{Email: "browser@test.io", Name: "B", BaseCurrency: domain.EUR}
	if err := d.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	sid, _ := NewSessionID()
	csrf, _ := NewCSRFToken()
	if err := d.CreateSession(t.Context(), &domain.Session{
		ID: sid, UserID: u.ID, ExpiresAt: time.Now().Add(lifetime),
	}); err != nil {
		t.Fatalf("create session: %v", err)
	}
	return u, sid, csrf
}

func newAuthedRequest(t *testing.T, url string) *http.Request {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	return req
}

func do(t *testing.T, req *http.Request) int {
	t.Helper()
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

// --- Bearer-token paths (pre-existing behaviour) ---

func TestMiddleware_Bearer_Valid(t *testing.T) {
	d := newDB(t)
	u, plain := seedUserToken(t, d)

	var seenUser *domain.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser = UserFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(next))
	defer srv.Close()

	req := newAuthedRequest(t, srv.URL)
	req.Header.Set("Authorization", "Bearer "+plain)
	if code := do(t, req); code != http.StatusNoContent {
		t.Errorf("status: %d", code)
	}
	if seenUser == nil || seenUser.ID != u.ID {
		t.Errorf("user: %+v", seenUser)
	}
}

func TestMiddleware_Bearer_Invalid(t *testing.T) {
	d := newDB(t)
	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	req := newAuthedRequest(t, srv.URL)
	req.Header.Set("Authorization", "Bearer nope")
	if code := do(t, req); code != http.StatusUnauthorized {
		t.Errorf("status: %d", code)
	}
}

func TestMiddleware_NoCredentials(t *testing.T) {
	d := newDB(t)
	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	if code := do(t, newAuthedRequest(t, srv.URL)); code != http.StatusUnauthorized {
		t.Errorf("status: %d", code)
	}
}

// --- Cookie + CSRF paths ---

func TestMiddleware_Cookie_SafeMethodOK(t *testing.T) {
	d := newDB(t)
	u, sid, csrf := seedUserSession(t, d, time.Hour)

	var seenUser *domain.User
	var seenSession string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser = UserFrom(r.Context())
		seenSession = SessionIDFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	mw := &Middleware{DB: d, SessionLifetime: time.Hour}
	srv := httptest.NewServer(mw.Handler(next))
	defer srv.Close()

	req := newAuthedRequest(t, srv.URL)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	if code := do(t, req); code != http.StatusNoContent {
		t.Errorf("status: %d", code)
	}
	if seenUser == nil || seenUser.ID != u.ID {
		t.Errorf("user: %+v", seenUser)
	}
	if seenSession != sid {
		t.Errorf("session id: %q", seenSession)
	}
}

func TestMiddleware_Cookie_UnsafeRequiresCSRF(t *testing.T) {
	d := newDB(t)
	_, sid, csrf := seedUserSession(t, d, time.Hour)

	mw := &Middleware{DB: d, SessionLifetime: time.Hour}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})))
	defer srv.Close()

	// Missing header → 403.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	if code := do(t, req); code != http.StatusForbidden {
		t.Errorf("missing csrf header: expected 403, got %d", code)
	}

	// Mismatched header → 403.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	req.Header.Set(CSRFHeaderName, "bogus")
	if code := do(t, req); code != http.StatusForbidden {
		t.Errorf("wrong csrf: expected 403, got %d", code)
	}

	// Matching header → 204.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodPost, srv.URL, nil)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	req.Header.Set(CSRFHeaderName, csrf)
	if code := do(t, req); code != http.StatusNoContent {
		t.Errorf("matching csrf: expected 204, got %d", code)
	}
}

func TestMiddleware_Cookie_BearerWinsOverCookie(t *testing.T) {
	d := newDB(t)
	// seed a browser session for user A
	_, sid, _ := seedUserSession(t, d, time.Hour)
	// seed a bearer token for user B
	userB, plainB := seedUserToken(t, d)

	var seen *domain.User
	mw := &Middleware{DB: d, SessionLifetime: time.Hour}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = UserFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})))
	defer srv.Close()

	req := newAuthedRequest(t, srv.URL)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	req.Header.Set("Authorization", "Bearer "+plainB)
	_ = do(t, req)
	if seen == nil || seen.ID != userB.ID {
		t.Errorf("Bearer should win; saw %+v (expected userB id=%d)", seen, userB.ID)
	}
}

func TestMiddleware_Cookie_Expired(t *testing.T) {
	d := newDB(t)
	// negative lifetime → already expired
	_, sid, csrf := seedUserSession(t, d, -time.Minute)

	mw := &Middleware{DB: d, SessionLifetime: time.Hour}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	req := newAuthedRequest(t, srv.URL)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: sid})
	req.AddCookie(&http.Cookie{Name: CSRFCookieName, Value: csrf})
	if code := do(t, req); code != http.StatusUnauthorized {
		t.Errorf("expired session: expected 401, got %d", code)
	}
}
