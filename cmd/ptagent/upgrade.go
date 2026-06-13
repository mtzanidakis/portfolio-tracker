package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/version"
)

const (
	upgradeRepo        = "mtzanidakis/portfolio-tracker"
	upgradeReleaseAPI  = "https://api.github.com/repos/" + upgradeRepo + "/releases/latest"
	upgradeDownloadURL = "https://github.com/" + upgradeRepo + "/releases/download"
)

func cmdUpgrade(args []string) int {
	fs := flag.NewFlagSet("upgrade", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	var (
		check     = fs.Bool("check", false, "only report current and target versions")
		targetVer = fs.String("version", "", "install this exact version instead of latest (e.g. v0.2.7)")
		yes       = fs.Bool("yes", false, "confirm in-place replacement of this binary")
	)
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if runtime.GOOS == "windows" {
		return errf("upgrade is not supported on windows; download a fresh archive from https://github.com/%s/releases", upgradeRepo)
	}

	current := version.Version

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	target := strings.TrimSpace(*targetVer)
	if target == "" {
		latest, err := fetchLatestTag(ctx)
		if err != nil {
			return errf("fetch latest release: %v", err)
		}
		target = latest
	}
	if !strings.HasPrefix(target, "v") {
		target = "v" + target
	}

	if *check {
		fmt.Printf("current=%s target=%s\n", current, target)
		return 0
	}

	if current == "dev" {
		return errf("cannot upgrade a dev build; install a released binary first (https://github.com/%s/releases)", upgradeRepo)
	}
	if current == target {
		fmt.Printf("ptagent %s already up to date\n", current)
		return 0
	}
	if !*yes {
		return errf("would upgrade %s → %s; pass --yes to proceed", current, target)
	}

	bareVer := strings.TrimPrefix(target, "v")
	asset := archiveName("ptagent", bareVer, runtime.GOOS, runtime.GOARCH)
	archiveURL := fmt.Sprintf("%s/%s/%s", upgradeDownloadURL, target, asset)
	checksumsURL := fmt.Sprintf("%s/%s/checksums.txt", upgradeDownloadURL, target)

	archiveBytes, err := downloadAndVerify(ctx, archiveURL, checksumsURL, asset)
	if err != nil {
		return errf("download: %v", err)
	}
	binBytes, err := extractBinary(archiveBytes, "ptagent")
	if err != nil {
		return errf("extract: %v", err)
	}
	if err := replaceSelf(binBytes); err != nil {
		return errf("replace binary: %v", err)
	}
	fmt.Printf("upgraded %s → %s\n", current, target)
	return 0
}

// archiveName mirrors the goreleaser name_template
// "{{ .Binary }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}" (tar.gz on non-windows).
func archiveName(binary, bareVer, goos, goarch string) string {
	return fmt.Sprintf("%s_%s_%s_%s.tar.gz", binary, bareVer, goos, goarch)
}

func fetchLatestTag(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, upgradeReleaseAPI, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("github API %s", resp.Status)
	}
	var rel struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return "", err
	}
	if rel.TagName == "" {
		return "", errors.New("empty tag in release response")
	}
	return rel.TagName, nil
}

func downloadAndVerify(ctx context.Context, archiveURL, checksumsURL, assetName string) ([]byte, error) {
	sumsRaw, err := fetchBytes(ctx, checksumsURL)
	if err != nil {
		return nil, fmt.Errorf("checksums: %w", err)
	}
	sums, err := parseChecksums(bytes.NewReader(sumsRaw))
	if err != nil {
		return nil, fmt.Errorf("checksums: %w", err)
	}
	want, ok := sums[assetName]
	if !ok {
		return nil, fmt.Errorf("no checksum entry for %s", assetName)
	}
	archiveBytes, err := fetchBytes(ctx, archiveURL)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", assetName, err)
	}
	got := sha256.Sum256(archiveBytes)
	if hex.EncodeToString(got[:]) != want {
		return nil, fmt.Errorf("sha256 mismatch for %s", assetName)
	}
	return archiveBytes, nil
}

// parseChecksums reads goreleaser checksums.txt lines (`<hex>  <filename>`).
func parseChecksums(r io.Reader) (map[string]string, error) {
	b, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	out := make(map[string]string)
	for line := range strings.SplitSeq(strings.TrimRight(string(b), "\n"), "\n") {
		if line == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}
		out[f[1]] = f[0]
	}
	return out, nil
}

func fetchBytes(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s", resp.Status)
	}
	// 50 MB cap is paranoia; a real archive is ~5 MB. A truncated read still
	// fails the sha256 check downstream, so this is just a memory safeguard.
	return io.ReadAll(io.LimitReader(resp.Body, 50<<20))
}

func extractBinary(archiveBytes []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archiveBytes))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, fmt.Errorf("entry %q not found in archive", name)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == name {
			return io.ReadAll(tr)
		}
	}
}

// replaceSelf writes newBytes next to the running executable and renames it
// into place. On Linux/macOS the kernel keeps the old inode alive for the
// running process, so the swap is safe mid-run.
func replaceSelf(newBytes []byte) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	dir := filepath.Dir(exe)
	base := filepath.Base(exe)

	tmp, err := os.CreateTemp(dir, base+".new-*")
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("permission denied writing to %s — try sudo", dir)
		}
		return err
	}
	tmpPath := tmp.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := tmp.Write(newBytes); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmpPath, exe); err != nil {
		cleanup()
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("permission denied replacing %s — try sudo", exe)
		}
		return err
	}
	return nil
}
