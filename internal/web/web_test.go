package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func TestHandler_ServesFiles(t *testing.T) {
	fsys := fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("<!doctype html><title>PT</title>")},
		"app.js":     &fstest.MapFile{Data: []byte("console.log('pt')")},
	}
	srv := httptest.NewServer(Handler(fsys))
	defer srv.Close()

	cases := []struct {
		path        string
		wantPrefix  string
		contentType string
	}{
		{"/", "<!doctype html>", "text/html"},
		{"/index.html", "<!doctype html>", "text/html"},
		{"/app.js", "console.log", "text/javascript"},
	}
	for _, tc := range cases {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+tc.path, nil)
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("do %s: %v", tc.path, err)
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: status %d", tc.path, resp.StatusCode)
		}
		if len(body) < len(tc.wantPrefix) || string(body[:len(tc.wantPrefix)]) != tc.wantPrefix {
			t.Errorf("%s body: %q", tc.path, string(body))
		}
	}
}

func TestHandler_Missing404(t *testing.T) {
	fsys := fstest.MapFS{}
	srv := httptest.NewServer(Handler(fsys))
	defer srv.Close()

	req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, srv.URL+"/nope.html", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status: %d", resp.StatusCode)
	}
}

func TestFS_Embeds(t *testing.T) {
	// The compile should have succeeded with dist/ committed as a
	// placeholder; FS() must be callable without error.
	if _, err := FS(); err != nil {
		t.Fatalf("FS: %v", err)
	}
}
