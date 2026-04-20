package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// config captures environment-derived settings for ptagent.
type config struct {
	APIURL string
	Token  string
}

func loadConfig() (*config, error) {
	url := os.Getenv("PT_API_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	tok := os.Getenv("PT_TOKEN")
	if tok == "" {
		return nil, fmt.Errorf("PT_TOKEN is required (create one with `ptadmin token create`)")
	}
	return &config{APIURL: url, Token: tok}, nil
}

// apiGET fetches path, decoding JSON into out (unless out is nil).
func apiGET(cfg *config, path string, out any) error {
	return apiDo(cfg, http.MethodGet, path, nil, out)
}

func apiPOST(cfg *config, path string, body, out any) error {
	return apiDo(cfg, http.MethodPost, path, body, out)
}

func apiPATCH(cfg *config, path string, body, out any) error {
	return apiDo(cfg, http.MethodPatch, path, body, out)
}

func apiDELETE(cfg *config, path string) error {
	return apiDo(cfg, http.MethodDelete, path, nil, nil)
}

func apiDo(cfg *config, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(b)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, cfg.APIURL+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		var apiErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(b, &apiErr) == nil && apiErr.Error != "" {
			return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, apiErr.Error)
		}
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, b)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// printJSON writes v as pretty JSON to stdout.
func printJSON(v any) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
