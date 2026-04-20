package auth

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

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

func setupUserWithToken(t *testing.T, d *db.DB) (*domain.User, string) {
	t.Helper()
	u := &domain.User{Email: "a@b.io", Name: "A", BaseCurrency: domain.EUR}
	if err := d.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	plain, hash, err := GenerateToken()
	if err != nil {
		t.Fatalf("gen: %v", err)
	}
	tok := &domain.Token{UserID: u.ID, Name: "test", Hash: hash}
	if err := d.CreateToken(t.Context(), tok); err != nil {
		t.Fatalf("create token: %v", err)
	}
	return u, plain
}

func sendGet(t *testing.T, url, bearer string) int {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode
}

func TestMiddleware_ValidToken(t *testing.T) {
	d := newDB(t)
	u, plain := setupUserWithToken(t, d)

	var seenUser *domain.User
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenUser = UserFrom(r.Context())
		w.WriteHeader(http.StatusNoContent)
	})
	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(next))
	defer srv.Close()

	if code := sendGet(t, srv.URL, plain); code != http.StatusNoContent {
		t.Errorf("status: %d", code)
	}
	if seenUser == nil || seenUser.ID != u.ID {
		t.Errorf("user context: %+v", seenUser)
	}
}

func TestMiddleware_MissingHeader(t *testing.T) {
	d := newDB(t)
	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	if code := sendGet(t, srv.URL, ""); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestMiddleware_InvalidToken(t *testing.T) {
	d := newDB(t)
	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	if code := sendGet(t, srv.URL, "not-a-real-token"); code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", code)
	}
}

func TestMiddleware_RevokedToken(t *testing.T) {
	d := newDB(t)
	_, plain := setupUserWithToken(t, d)

	tok, _ := d.GetTokenByHash(t.Context(), HashToken(plain))
	if err := d.RevokeToken(t.Context(), tok.ID); err != nil {
		t.Fatal(err)
	}

	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	if code := sendGet(t, srv.URL, plain); code != http.StatusUnauthorized {
		t.Errorf("revoked token should 401, got %d", code)
	}
}

func TestMiddleware_TouchesToken(t *testing.T) {
	d := newDB(t)
	_, plain := setupUserWithToken(t, d)
	tok, _ := d.GetTokenByHash(t.Context(), HashToken(plain))
	if tok.LastUsedAt != nil {
		t.Fatal("precondition: LastUsedAt should be nil")
	}

	mw := &Middleware{DB: d}
	srv := httptest.NewServer(mw.Handler(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})))
	defer srv.Close()

	_ = sendGet(t, srv.URL, plain)

	again, _ := d.GetToken(t.Context(), tok.ID)
	if again.LastUsedAt == nil {
		t.Error("expected LastUsedAt to be set after auth")
	}
}
