package api

import (
	"net/http"
	"strings"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/prices"
)

func listAssetsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		assets, err := d.ListAssets(r.Context())
		if err != nil {
			writeDBError(w, err)
			return
		}
		q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
		if q != "" {
			filtered := make([]*domain.Asset, 0, len(assets))
			for _, a := range assets {
				if strings.Contains(strings.ToLower(a.Symbol), q) ||
					strings.Contains(strings.ToLower(a.Name), q) {
					filtered = append(filtered, a)
				}
			}
			assets = filtered
		}
		writeJSON(w, http.StatusOK, assets)
	}
}

func getAssetHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		asset, err := d.GetAsset(r.Context(), r.PathValue("symbol"))
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, asset)
	}
}

type assetRequest struct {
	Symbol     string           `json:"symbol"`
	Name       string           `json:"name"`
	Type       domain.AssetType `json:"type"`
	Currency   domain.Currency  `json:"currency"`
	Color      string           `json:"color"`
	Provider   string           `json:"provider"`
	ProviderID string           `json:"provider_id"`
	LogoURL    string           `json:"logo_url"`
}

func upsertAssetHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req assetRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if req.Symbol == "" || req.Name == "" {
			writeError(w, http.StatusBadRequest, "symbol and name are required")
			return
		}
		if !req.Type.Valid() {
			writeError(w, http.StatusBadRequest, "invalid type")
			return
		}
		if !req.Currency.Valid() {
			writeError(w, http.StatusBadRequest, "invalid currency")
			return
		}
		a := &domain.Asset{
			Symbol:     req.Symbol,
			Name:       req.Name,
			Type:       req.Type,
			Currency:   req.Currency,
			Color:      req.Color,
			Provider:   req.Provider,
			ProviderID: req.ProviderID,
			LogoURL:    req.LogoURL,
		}
		if err := d.UpsertAsset(r.Context(), a); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, a)
	}
}

type assetLookupResponse struct {
	Symbol     string `json:"symbol"`
	Name       string `json:"name"`
	Currency   string `json:"currency,omitempty"`
	Type       string `json:"type,omitempty"`
	Provider   string `json:"provider"`
	ProviderID string `json:"provider_id,omitempty"`
	LogoURL    string `json:"logo_url,omitempty"`
}

// lookupAssetHandler resolves a user-typed symbol via the given
// provider, used by the "add asset" form to auto-fill the name / native
// currency / type. Returns 404 when the provider finds no match so the
// UI can keep the manually-typed values.
func lookupAssetHandler(yahoo, coingecko prices.SymbolLookup) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		symbol := strings.TrimSpace(q.Get("symbol"))
		if symbol == "" {
			writeError(w, http.StatusBadRequest, "symbol is required")
			return
		}
		provider := strings.ToLower(strings.TrimSpace(q.Get("provider")))
		if provider == "" {
			provider = "yahoo"
		}
		var lookup prices.SymbolLookup
		switch provider {
		case "yahoo":
			lookup = yahoo
		case "coingecko":
			lookup = coingecko
		default:
			writeError(w, http.StatusBadRequest, "unknown provider")
			return
		}
		if lookup == nil {
			writeError(w, http.StatusServiceUnavailable, "provider not configured")
			return
		}
		info, err := lookup.LookupSymbol(r.Context(), symbol)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		if info == nil {
			writeError(w, http.StatusNotFound, "symbol not found")
			return
		}
		writeJSON(w, http.StatusOK, assetLookupResponse{
			Symbol:     info.Symbol,
			Name:       info.Name,
			Currency:   string(info.Currency),
			Type:       string(info.AssetType),
			Provider:   provider,
			ProviderID: info.ProviderID,
			LogoURL:    info.LogoURL,
		})
	}
}

func deleteAssetHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := d.DeleteAsset(r.Context(), r.PathValue("symbol")); err != nil {
			writeDBError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
