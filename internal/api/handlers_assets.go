package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/prices"
)

// maxLogoBytes caps the size of an externally-fetched logo to 2 MB so
// a misbehaving upstream can't OOM the DB. Real logos are well under
// 100 KB.
const maxLogoBytes = 2 << 20

// logoHostAllowlist restricts the hosts we're willing to proxy images
// from. This blocks SSRF via a rogue asset.logo_url and keeps us from
// accidentally acting as a free image-proxy for the world.
var logoHostAllowlist = map[string]struct{}{
	"assets.parqet.com":         {},
	"assets.coingecko.com":      {},
	"coin-images.coingecko.com": {},
}

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

// assetLogoHandler serves /api/v1/assets/{symbol}/logo. Bytes come from
// the asset_logos cache when present; otherwise the handler fetches
// asset.logo_url once, writes the result to the cache, and serves it.
// The endpoint is cookie-authenticated like every other /api/v1 route,
// which is fine for <img> tags on the same origin.
func assetLogoHandler(d *db.DB, client *http.Client) http.HandlerFunc {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		symbol := strings.TrimSpace(r.PathValue("symbol"))
		if symbol == "" {
			http.NotFound(w, r)
			return
		}

		if logo, err := d.GetAssetLogo(r.Context(), symbol); err == nil {
			serveLogo(w, logo)
			return
		} else if !errors.Is(err, db.ErrNotFound) {
			writeDBError(w, err)
			return
		}

		asset, err := d.GetAsset(r.Context(), symbol)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if asset.LogoURL == "" {
			http.NotFound(w, r)
			return
		}

		body, ct, err := fetchLogoBytes(r.Context(), client, asset.LogoURL)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		logo := &db.AssetLogo{
			Symbol:      symbol,
			Bytes:       body,
			ContentType: ct,
			FetchedAt:   time.Now().UTC(),
		}
		if err := d.PutAssetLogo(r.Context(), logo); err != nil {
			writeDBError(w, err)
			return
		}
		serveLogo(w, logo)
	}
}

// fetchLogoBytes pulls the image at rawURL through the provided
// client, capping the response at maxLogoBytes. The URL is validated
// against logoHostAllowlist (https only, known logo hosts) so this
// endpoint can't be turned into an open image-proxy. The effective
// Content-Type is sniffed from the bytes — never trusted from the
// upstream header — to stop an upstream from feeding us a text/html
// payload that the browser would execute.
func fetchLogoBytes(ctx context.Context, client *http.Client, rawURL string) ([]byte, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", fmt.Errorf("invalid logo url: %w", err)
	}
	if u.Scheme != "https" {
		return nil, "", fmt.Errorf("logo url must be https")
	}
	if _, ok := logoHostAllowlist[strings.ToLower(u.Host)]; !ok {
		return nil, "", fmt.Errorf("logo host %q not allowed", u.Host)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil) //nolint:gosec // host validated against logoHostAllowlist above
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Accept", "image/*")
	resp, err := client.Do(req) //nolint:gosec // request URL validated above
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("upstream status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxLogoBytes))
	if err != nil {
		return nil, "", err
	}
	ct := http.DetectContentType(body)
	if !strings.HasPrefix(ct, "image/") {
		return nil, "", fmt.Errorf("unexpected content-type %q", ct)
	}
	return body, ct, nil
}

func serveLogo(w http.ResponseWriter, logo *db.AssetLogo) {
	w.Header().Set("Content-Type", logo.ContentType)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(logo.Bytes)))
	_, _ = w.Write(logo.Bytes) //nolint:gosec // Content-Type sniffed as image/* + nosniff header, not HTML
}
