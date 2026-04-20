package api

import (
	"net/http"

	"github.com/mtzanidakis/portfolio-tracker/internal/version"
)

func versionHandler(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": version.Version})
}
