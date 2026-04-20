package api

import (
	"net/http/cookiejar"
)

// newJar returns a permissive cookiejar for tests.
func newJar() (*cookiejar.Jar, error) {
	return cookiejar.New(nil)
}
