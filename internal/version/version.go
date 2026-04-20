// Package version exposes the build version of the binary.
//
// The Version variable is overridden at build time via:
//
//	-ldflags "-X github.com/mtzanidakis/portfolio-tracker/internal/version.Version=..."
package version

// Version is the build version string.
// It is set at build time via -ldflags; defaults to "dev".
var Version = "dev"
