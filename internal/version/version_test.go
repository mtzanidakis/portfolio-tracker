package version

import "testing"

func TestVersionHasDefault(t *testing.T) {
	if Version == "" {
		t.Fatal("Version must not be empty")
	}
}
