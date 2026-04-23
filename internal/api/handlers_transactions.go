package api

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/auth"
	"github.com/mtzanidakis/portfolio-tracker/internal/db"
	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

type txRequest struct {
	AccountID   int64     `json:"account_id"`
	AssetSymbol string    `json:"asset_symbol"`
	Side        string    `json:"side"`
	Qty         float64   `json:"qty"`
	Price       float64   `json:"price"`
	Fee         float64   `json:"fee"`
	FxToBase    float64   `json:"fx_to_base"`
	OccurredAt  time.Time `json:"occurred_at"`
	Note        string    `json:"note"`
}

func listTransactionsHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		q := r.URL.Query()

		f := db.TxFilter{UserID: u.ID}
		if v := q.Get("account_id"); v != "" {
			if id, err := strconv.ParseInt(v, 10, 64); err == nil {
				f.AccountID = id
			}
		}
		if v := q.Get("symbol"); v != "" {
			f.AssetSymbol = v
		}
		if v := q.Get("side"); v != "" {
			// Comma-separated → multi-side IN filter (UI group tabs).
			// Single value → stays on the backward-compat Side field so
			// ptagent's --side flag keeps working untouched.
			if strings.Contains(v, ",") {
				for s := range strings.SplitSeq(v, ",") {
					s = strings.TrimSpace(s)
					if s != "" {
						f.Sides = append(f.Sides, domain.TxSide(s))
					}
				}
			} else {
				f.Side = domain.TxSide(v)
			}
		}
		if v := q.Get("q"); v != "" {
			f.Q = strings.TrimSpace(v)
		}
		if v := q.Get("from"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				f.From = t
			}
		}
		if v := q.Get("to"); v != "" {
			if t, err := time.Parse(time.RFC3339, v); err == nil {
				f.To = t
			}
		}
		if v := q.Get("limit"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				f.Limit = n
			}
		}
		if v := q.Get("sort"); v != "" {
			f.Sort = v
		}
		if v := q.Get("order"); v != "" {
			f.Order = v
		}
		if v := q.Get("cursor"); v != "" {
			sort, sortVal, id, err := decodeTxCursor(v)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid cursor")
				return
			}
			f.CursorSort = sort
			f.CursorSortVal = sortVal
			f.CursorID = id
		}

		txs, err := d.ListTransactions(r.Context(), f)
		if err != nil {
			writeDBError(w, err)
			return
		}
		// Emit the next-page cursor as a header so the response body
		// stays a plain array (keeps ptagent + any other array-shaped
		// consumer working). Only set when we filled the page — if
		// fewer rows came back than requested, we're at the end.
		if f.Limit > 0 && len(txs) == f.Limit {
			last := txs[len(txs)-1]
			sortKey := f.Sort
			if sortKey == "" {
				sortKey = "date"
			}
			w.Header().Set("X-Next-Cursor",
				encodeTxCursor(sortKey, db.FormatTxCursorValue(last, sortKey), last.ID))
		}
		writeJSON(w, http.StatusOK, txs)
	}
}

// encodeTxCursor packs (sort, sortValue, id) into an opaque base64
// token of the form "<sort>|<sortvalue>|<id>". Clients are expected
// to treat it as a blob and echo it back verbatim.
func encodeTxCursor(sort, sortVal string, id int64) string {
	raw := fmt.Sprintf("%s|%s|%d", sort, sortVal, id)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeTxCursor(s string) (sort string, sortVal string, id int64, err error) {
	b, derr := base64.RawURLEncoding.DecodeString(s)
	if derr != nil {
		return "", "", 0, derr
	}
	parts := strings.SplitN(string(b), "|", 3)
	if len(parts) != 3 {
		return "", "", 0, fmt.Errorf("cursor: bad format")
	}
	idV, perr := strconv.ParseInt(parts[2], 10, 64)
	if perr != nil {
		return "", "", 0, perr
	}
	return parts[0], parts[1], idV, nil
}

func createTransactionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		var req txRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if err := validateTx(req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		// Ownership: account must belong to user.
		acc, err := d.GetAccount(r.Context(), req.AccountID)
		if err != nil || acc.UserID != u.ID {
			writeError(w, http.StatusBadRequest, "account not found")
			return
		}
		// Cross-validation: asset type must match the side. Cash sides
		// only apply to cash assets; buy/sell must target a non-cash
		// asset. Cash operations are always at unit price 1 by
		// definition, so we normalise that here.
		asset, err := d.GetAsset(r.Context(), req.AssetSymbol)
		if err != nil {
			writeError(w, http.StatusBadRequest, "asset not found")
			return
		}
		if err := validateSideVsAsset(domain.TxSide(req.Side), asset.Type); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		price := req.Price
		if domain.TxSide(req.Side).IsCash() {
			price = 1
		}
		tx := &domain.Transaction{
			UserID:      u.ID,
			AccountID:   req.AccountID,
			AssetSymbol: req.AssetSymbol,
			Side:        domain.TxSide(req.Side),
			Qty:         req.Qty,
			Price:       price,
			Fee:         req.Fee,
			FxToBase:    req.FxToBase,
			OccurredAt:  req.OccurredAt,
			Note:        req.Note,
		}
		if err := d.CreateTransaction(r.Context(), tx); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, tx)
	}
}

// transactionSummaryHandler returns one-shot aggregates for the
// signed-in user — used by the Activities hero so the page doesn't
// have to paginate through the whole history just to draw totals.
func transactionSummaryHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		s, err := d.TransactionSummary(r.Context(), u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, s)
	}
}

func getTransactionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		tx, err := loadOwnedTx(r, d, u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tx)
	}
}

func updateTransactionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		tx, err := loadOwnedTx(r, d, u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		var req txRequest
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid body: "+err.Error())
			return
		}
		if req.AccountID != 0 {
			tx.AccountID = req.AccountID
		}
		if req.AssetSymbol != "" {
			tx.AssetSymbol = req.AssetSymbol
		}
		if req.Side != "" {
			tx.Side = domain.TxSide(req.Side)
		}
		if req.Qty > 0 {
			tx.Qty = req.Qty
		}
		if req.Price >= 0 && req.Price != 0 {
			tx.Price = req.Price
		}
		if req.Fee > 0 {
			tx.Fee = req.Fee
		}
		if req.FxToBase > 0 {
			tx.FxToBase = req.FxToBase
		}
		if !req.OccurredAt.IsZero() {
			tx.OccurredAt = req.OccurredAt
		}
		tx.Note = req.Note
		// Re-validate the (possibly patched) side against the asset so
		// we can't smuggle in a mismatched combination on PATCH either.
		asset, aerr := d.GetAsset(r.Context(), tx.AssetSymbol)
		if aerr != nil {
			writeError(w, http.StatusBadRequest, "asset not found")
			return
		}
		if err := validateSideVsAsset(tx.Side, asset.Type); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		if tx.Side.IsCash() {
			tx.Price = 1
		}
		if err := d.UpdateTransaction(r.Context(), tx); err != nil {
			writeDBError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, tx)
	}
}

func deleteTransactionHandler(d *db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		u := auth.UserFrom(r.Context())
		tx, err := loadOwnedTx(r, d, u.ID)
		if err != nil {
			writeDBError(w, err)
			return
		}
		if err := d.DeleteTransaction(r.Context(), tx.ID); err != nil {
			writeDBError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func validateTx(req txRequest) error {
	switch {
	case req.AccountID == 0:
		return errBadReq("account_id is required")
	case req.AssetSymbol == "":
		return errBadReq("asset_symbol is required")
	case !domain.TxSide(req.Side).Valid():
		return errBadReq("invalid side")
	case req.Qty <= 0:
		return errBadReq("qty must be positive")
	case req.Price < 0:
		return errBadReq("price must be non-negative")
	case req.FxToBase <= 0:
		return errBadReq("fx_to_base must be positive")
	case req.OccurredAt.IsZero():
		return errBadReq("occurred_at is required")
	}
	return nil
}

// validateSideVsAsset enforces the side/asset-type compatibility:
// deposit/withdraw/interest only apply to cash assets; buy/sell only
// apply to non-cash assets. Called after the request passes the
// request-level validateTx check.
func validateSideVsAsset(side domain.TxSide, t domain.AssetType) error {
	if side.IsCash() && t != domain.AssetCash {
		return errBadReq("deposit/withdraw/interest require a cash asset")
	}
	if !side.IsCash() && t == domain.AssetCash {
		return errBadReq("buy/sell cannot target a cash asset")
	}
	return nil
}

type badReqError string

func (e badReqError) Error() string { return string(e) }
func errBadReq(s string) error      { return badReqError(s) }

func loadOwnedTx(r *http.Request, d *db.DB, userID int64) (*domain.Transaction, error) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		return nil, db.ErrNotFound
	}
	tx, err := d.GetTransaction(r.Context(), id)
	if err != nil {
		return nil, err
	}
	if tx.UserID != userID {
		return nil, db.ErrNotFound
	}
	return tx, nil
}
