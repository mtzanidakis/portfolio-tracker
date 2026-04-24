package api

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
	"github.com/mtzanidakis/portfolio-tracker/internal/exporters"
	"github.com/mtzanidakis/portfolio-tracker/internal/version"
)

// exportHandler serves a downloadable snapshot of the signed-in
// user's accounts, assets and transactions. ?format=json returns the
// full envelope (suitable as a backup); ?format=csv returns a
// transactions-only CSV for spreadsheet import.
//
// Only the user's own accounts and transactions are included. Assets
// are shared globally in our schema but we export the subset actually
// referenced by the user's transactions to keep the file focused.
func exportHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		format := strings.ToLower(r.URL.Query().Get("format"))
		if format == "" {
			format = "json"
		}

		accounts, err := d.ListAccounts(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		txs, err := d.ListTransactions(r.Context(), db.TxFilter{UserID: u.ID})
		if err != nil {
			writeDBError(w, err)
			return
		}
		assets, err := userAssets(r.Context(), d, txs)
		if err != nil {
			writeDBError(w, err)
			return
		}

		stamp := time.Now().UTC().Format("20060102-150405")
		switch format {
		case "json":
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Content-Disposition",
				`attachment; filename="portfolio-tracker-`+stamp+`.json"`)
			user, uerr := d.GetUser(r.Context(), u.ID)
			if uerr != nil {
				writeDBError(w, uerr)
				return
			}
			if err := exporters.WriteJSON(w, version.Version, user.BaseCurrency,
				accounts, assets, txs); err != nil {
				// Headers already sent — best we can do is stop writing.
				return
			}
		case "csv":
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			w.Header().Set("Content-Disposition",
				`attachment; filename="portfolio-tracker-transactions-`+stamp+`.csv"`)
			accName := map[int64]string{}
			for _, a := range accounts {
				accName[a.ID] = a.Name
			}
			symInfo := map[string]*domain.Asset{}
			for _, a := range assets {
				symInfo[a.Symbol] = a
			}
			_ = exporters.WriteTransactionsCSV(w, txs, accName, symInfo)
		default:
			writeError(w, http.StatusBadRequest, "unknown format")
			return
		}
	}
}

// userAssets returns the assets actually referenced by the given
// transaction list. We walk all assets once (shared table, small) and
// filter by the symbols that appear in the user's txs.
func userAssets(ctx context.Context, d *db.DB, txs []*domain.Transaction) ([]*domain.Asset, error) {
	symbols := map[string]struct{}{}
	for _, t := range txs {
		symbols[t.AssetSymbol] = struct{}{}
	}
	all, err := d.ListAssets(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Asset, 0, len(symbols))
	for _, a := range all {
		if _, ok := symbols[a.Symbol]; ok {
			out = append(out, a)
		}
	}
	return out, nil
}
