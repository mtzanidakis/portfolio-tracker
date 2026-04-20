// Package web embeds the built Preact frontend (web/dist, copied into
// ./dist by the Dockerfile web stage) and exposes an http.Handler that
// serves it as static files.
package web

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// FS returns a sub-FS rooted at the embedded dist/ directory. Exposed
// primarily for tests.
func FS() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}

// Handler returns an http.Handler that serves the given filesystem as
// static content. Missing files yield 404.
func Handler(fsys fs.FS) http.Handler {
	return http.FileServer(http.FS(fsys))
}

// DefaultHandler serves the embedded dist/.
func DefaultHandler() http.Handler {
	sub, err := FS()
	if err != nil {
		// Should not happen: dist/ is guaranteed by the build.
		panic(err)
	}
	return Handler(sub)
}
