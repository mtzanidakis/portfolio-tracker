package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// authEnv wires a real DB + router and a user with a known password.
type authEnv struct {
	t    *testing.T
	db   *db.DB
	srv  *httptest.Server
	user *domain.User
	pw   string
}

func authSetup(t *testing.T) *authEnv {
	t.Helper()
	d, err := db.Open(t.Context(), filepath.Join(t.TempDir(), "auth-api.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Migrate(t.Context()); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	pw := "correct horse battery staple"
	hash, _ := auth.HashPassword(pw)
	u := &domain.User{
		Email: "browser@test.io", Name: "Browser",
		BaseCurrency: domain.EUR, PasswordHash: hash,
	}
	if err := d.CreateUser(t.Context(), u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	srv := httptest.NewServer(NewRouter(d, time.Hour))
	t.Cleanup(srv.Close)
	return &authEnv{t: t, db: d, srv: srv, user: u, pw: pw}
}

func postJSON(t *testing.T, client *http.Client, url string, body any, csrf string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrf != "" {
		req.Header.Set(auth.CSRFHeaderName, csrf)
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func csrfFromJar(t *testing.T, jar http.CookieJar, srvURL string) string {
	t.Helper()
	u, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srvURL, nil)
	for _, c := range jar.Cookies(u.URL) {
		if c.Name == auth.CSRFCookieName {
			return c.Value
		}
	}
	t.Fatal("no csrf cookie in jar")
	return ""
}

func newCookieClient(t *testing.T) *http.Client {
	t.Helper()
	jar, err := newJar()
	if err != nil {
		t.Fatal(err)
	}
	return &http.Client{Jar: jar}
}

func TestLogin_Success(t *testing.T) {
	e := authSetup(t)
	client := newCookieClient(t)

	resp := postJSON(t, client, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d body=%s", resp.StatusCode, body)
	}

	// Cookies were set.
	csrf := csrfFromJar(t, client.Jar, e.srv.URL)
	if csrf == "" {
		t.Error("expected pt_csrf cookie")
	}

	// Subsequent GET /me works.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, e.srv.URL+"/api/v1/me", nil)
	resp, _ = client.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("me status: %d", resp.StatusCode)
	}
}

func TestLogin_WrongPassword_Generic401(t *testing.T) {
	e := authSetup(t)
	client := newCookieClient(t)

	for _, body := range []map[string]string{
		{"email": e.user.Email, "password": "wrong"},
		{"email": "nope@nowhere.io", "password": "x"}, // user does not exist
		{"email": e.user.Email, "password": ""},       // empty
	} {
		resp := postJSON(t, client, e.srv.URL+"/api/v1/login", body, "")
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("body=%+v: expected 401, got %d", body, resp.StatusCode)
		}
	}
}

func TestLogin_EmptyPasswordHashUserCannotLogIn(t *testing.T) {
	e := authSetup(t)
	// Create a user without a password (the default for ptadmin user add
	// if admin skips the prompt).
	other := &domain.User{
		Email: "empty@test.io", Name: "E", BaseCurrency: domain.EUR,
	}
	_ = e.db.CreateUser(t.Context(), other)

	client := newCookieClient(t)
	resp := postJSON(t, client, e.srv.URL+"/api/v1/login",
		map[string]string{"email": other.Email, "password": "anything"}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLogout_DeletesSessionAndClearsCookies(t *testing.T) {
	e := authSetup(t)
	client := newCookieClient(t)

	// Login.
	_ = postJSON(t, client, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	csrf := csrfFromJar(t, client.Jar, e.srv.URL)

	// Logout (POST — needs CSRF header).
	resp := postJSON(t, client, e.srv.URL+"/api/v1/logout", nil, csrf)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("logout status: %d", resp.StatusCode)
	}

	// Subsequent /me should 401.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, e.srv.URL+"/api/v1/me", nil)
	resp, _ = client.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("post-logout /me: expected 401, got %d", resp.StatusCode)
	}
}

func TestChangePassword_VerifiesCurrent(t *testing.T) {
	e := authSetup(t)
	client := newCookieClient(t)
	_ = postJSON(t, client, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	csrf := csrfFromJar(t, client.Jar, e.srv.URL)

	// Wrong current → 401.
	resp := postJSON(t, client, e.srv.URL+"/api/v1/password",
		map[string]string{"current": "wrong", "new": "new-passw0rd"}, csrf)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("wrong current: expected 401, got %d", resp.StatusCode)
	}

	// Too-short new → 400.
	resp = postJSON(t, client, e.srv.URL+"/api/v1/password",
		map[string]string{"current": e.pw, "new": "short"}, csrf)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("short pw: expected 400, got %d", resp.StatusCode)
	}

	// Correct → 204.
	resp = postJSON(t, client, e.srv.URL+"/api/v1/password",
		map[string]string{"current": e.pw, "new": "a-new-passw0rd"}, csrf)
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("good change: expected 204, got %d", resp.StatusCode)
	}

	// Re-login with old → 401.
	resp = postJSON(t, newCookieClient(t), e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("old pw after change: expected 401, got %d", resp.StatusCode)
	}
}

func TestChangePassword_KillsOtherSessions(t *testing.T) {
	e := authSetup(t)

	// Session A.
	clientA := newCookieClient(t)
	_ = postJSON(t, clientA, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")

	// Session B.
	clientB := newCookieClient(t)
	_ = postJSON(t, clientB, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	csrfB := csrfFromJar(t, clientB.Jar, e.srv.URL)

	// Change password from session B.
	resp := postJSON(t, clientB, e.srv.URL+"/api/v1/password",
		map[string]string{"current": e.pw, "new": "another-passw0rd"}, csrfB)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("change pw: %d", resp.StatusCode)
	}

	// Session A must be dead.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, e.srv.URL+"/api/v1/me", nil)
	resp, _ = clientA.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("session A should be invalidated, got %d", resp.StatusCode)
	}

	// Session B still works.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodGet, e.srv.URL+"/api/v1/me", nil)
	resp, _ = clientB.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("session B should still be valid, got %d", resp.StatusCode)
	}
}

func TestPatchMe_EmailUniqueness(t *testing.T) {
	e := authSetup(t)
	// Second user.
	_ = e.db.CreateUser(t.Context(), &domain.User{
		Email: "other@test.io", Name: "O", BaseCurrency: domain.EUR,
	})
	client := newCookieClient(t)
	_ = postJSON(t, client, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	csrf := csrfFromJar(t, client.Jar, e.srv.URL)

	// Attempt to steal other user's email.
	b, _ := json.Marshal(map[string]string{"email": "other@test.io"})
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPatch,
		e.srv.URL+"/api/v1/me", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(auth.CSRFHeaderName, csrf)
	resp, _ := client.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409, got %d", resp.StatusCode)
	}
}

func TestMeTokens_SelfServiceCRUD(t *testing.T) {
	e := authSetup(t)
	client := newCookieClient(t)
	_ = postJSON(t, client, e.srv.URL+"/api/v1/login",
		map[string]string{"email": e.user.Email, "password": e.pw}, "")
	csrf := csrfFromJar(t, client.Jar, e.srv.URL)

	// Create.
	resp := postJSON(t, client, e.srv.URL+"/api/v1/me/tokens",
		map[string]string{"name": "cli"}, csrf)
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("create: %d body=%s", resp.StatusCode, body)
	}
	var created createTokenResponse
	_ = json.NewDecoder(resp.Body).Decode(&created)
	if created.Token == "" {
		t.Error("plaintext token missing from create response")
	}
	if !strings.Contains(created.Token, "") || len(created.Token) < 40 {
		t.Errorf("token looks wrong: %q", created.Token)
	}

	// List includes it.
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet,
		e.srv.URL+"/api/v1/me/tokens", nil)
	resp, _ = client.Do(req)
	defer func() { _ = resp.Body.Close() }()
	var list []tokenListRow
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 1 || list[0].Name != "cli" {
		t.Errorf("list: %+v", list)
	}

	// Delete.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodDelete,
		e.srv.URL+"/api/v1/me/tokens/"+strconv.FormatInt(created.ID, 10), nil)
	req.Header.Set(auth.CSRFHeaderName, csrf)
	resp, _ = client.Do(req)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("revoke: %d", resp.StatusCode)
	}

	// Token now appears revoked.
	req, _ = http.NewRequestWithContext(t.Context(), http.MethodGet,
		e.srv.URL+"/api/v1/me/tokens", nil)
	resp, _ = client.Do(req)
	defer func() { _ = resp.Body.Close() }()
	_ = json.NewDecoder(resp.Body).Decode(&list)
	if list[0].RevokedAt == nil {
		t.Error("token should be marked revoked")
	}
}
