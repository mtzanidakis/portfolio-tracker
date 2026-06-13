package main

import (
	"strings"
	"testing"
)

func TestArchiveName(t *testing.T) {
	got := archiveName("ptagent", "0.2.8", "linux", "amd64")
	want := "ptagent_0.2.8_linux_amd64.tar.gz"
	if got != want {
		t.Fatalf("archiveName = %q, want %q", got, want)
	}
}

func TestParseChecksums(t *testing.T) {
	raw := `abc123  ptagent_0.2.8_linux_amd64.tar.gz
def456  ptagent_0.2.8_darwin_arm64.tar.gz

ff00  checksums.txt
`
	m, err := parseChecksums(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("parseChecksums: %v", err)
	}
	if m["ptagent_0.2.8_linux_amd64.tar.gz"] != "abc123" {
		t.Errorf("linux amd64 = %q, want abc123", m["ptagent_0.2.8_linux_amd64.tar.gz"])
	}
	if m["ptagent_0.2.8_darwin_arm64.tar.gz"] != "def456" {
		t.Errorf("darwin arm64 = %q, want def456", m["ptagent_0.2.8_darwin_arm64.tar.gz"])
	}
	if m["checksums.txt"] != "ff00" {
		t.Errorf("checksums.txt = %q, want ff00", m["checksums.txt"])
	}
	if len(m) != 3 {
		t.Errorf("got %d entries, want 3 (blanks must be skipped)", len(m))
	}
}
