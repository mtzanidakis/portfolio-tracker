package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// TxFilter narrows the ListTransactions result set. All fields are optional.
type TxFilter struct {
	UserID      int64 // required (0 = unscoped, not recommended)
	AccountID   int64
	AssetSymbol string
	Side        domain.TxSide   // single-value (ptagent backward compat)
	Sides       []domain.TxSide // multi-value (UI group filters)
	From        time.Time       // inclusive
	To          time.Time       // inclusive
	Limit       int             // 0 = no limit
	// Q is a free-text needle matched case-insensitively against the
	// tx's asset symbol, the asset's display name (via LEFT JOIN) and
	// the tx note. Empty string disables the filter.
	Q string
	// Sort / Order control ORDER BY. Sort must be one of the keys in
	// txSortColumns; unknown values fall back to "date". Order is
	// "asc" or "desc"; anything else is treated as "desc".
	Sort  string
	Order string
	// Keyset cursor. CursorSort must match Sort (handler verifies);
	// CursorSortVal is the opaque string form of the last row's sort
	// value and CursorID the tiebreaker. Populated together or none.
	CursorSort    string
	CursorSortVal string
	CursorID      int64
}

// txSortColumns maps the public sort keys onto SQL column expressions.
// "total" is a computed column (qty×price) — fine for ORDER BY; cursor
// predicates reuse the same expression so the keyset boundary matches.
var txSortColumns = map[string]string{
	"date":    "t.occurred_at",
	"symbol":  "t.asset_symbol",
	"side":    "t.side",
	"qty":     "t.qty",
	"price":   "t.price",
	"total":   "(t.qty * t.price)",
	"fee":     "t.fee",
	"account": "t.account_id",
}

// parseTxCursorValue decodes a string-encoded sort value (as produced
// by FormatTxCursorValue / emitted by the API layer) back into the Go
// type SQLite expects for the ORDER BY column.
func parseTxCursorValue(sort, s string) (any, error) {
	switch sort {
	case "symbol", "side":
		return s, nil
	case "qty", "price", "total", "fee":
		return strconv.ParseFloat(s, 64)
	case "account":
		return strconv.ParseInt(s, 10, 64)
	default: // "date"
		ns, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		return time.Unix(0, ns).UTC(), nil
	}
}

// FormatTxCursorValue produces the opaque string carried in an
// X-Next-Cursor header for the given row + sort key. Inverse of
// parseTxCursorValue.
func FormatTxCursorValue(t *domain.Transaction, sort string) string {
	switch sort {
	case "symbol":
		return t.AssetSymbol
	case "side":
		return string(t.Side)
	case "qty":
		return strconv.FormatFloat(t.Qty, 'f', -1, 64)
	case "price":
		return strconv.FormatFloat(t.Price, 'f', -1, 64)
	case "total":
		return strconv.FormatFloat(t.Qty*t.Price, 'f', -1, 64)
	case "fee":
		return strconv.FormatFloat(t.Fee, 'f', -1, 64)
	case "account":
		return strconv.FormatInt(t.AccountID, 10)
	default: // "date"
		return strconv.FormatInt(t.OccurredAt.UTC().UnixNano(), 10)
	}
}

// CreateTransaction inserts the given transaction and populates its ID +
// CreatedAt.
func (db *DB) CreateTransaction(ctx context.Context, t *domain.Transaction) error {
	if !t.Side.Valid() {
		return fmt.Errorf("invalid side %q", t.Side)
	}
	res, err := db.ExecContext(ctx, `
        INSERT INTO transactions(user_id, account_id, asset_symbol, side,
                                 qty, price, fee, fx_to_base, occurred_at, note)
        VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.UserID, t.AccountID, t.AssetSymbol, string(t.Side),
		t.Qty, t.Price, t.Fee, t.FxToBase, t.OccurredAt, t.Note,
	)
	if err != nil {
		return fmt.Errorf("insert tx: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	t.ID = id
	return db.QueryRowContext(ctx,
		"SELECT created_at FROM transactions WHERE id = ?", id,
	).Scan(&t.CreatedAt)
}

// GetTransaction returns the transaction by id or ErrNotFound.
func (db *DB) GetTransaction(ctx context.Context, id int64) (*domain.Transaction, error) {
	row := db.QueryRowContext(ctx, `
        SELECT id, user_id, account_id, asset_symbol, side, qty, price, fee,
               fx_to_base, occurred_at, note, created_at
          FROM transactions WHERE id = ?`, id)
	t, err := scanTxRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// ListTransactions returns transactions matching f, ordered newest first.
// Free-text search routes through the tx_fts virtual table so symbol,
// the asset's display name and the tx note are indexed together with
// unicode-aware tokenisation (no LIKE scan, no LEFT JOIN).
func (db *DB) ListTransactions(ctx context.Context, f TxFilter) ([]*domain.Transaction, error) {
	var (
		conds []string
		args  []any
	)
	if f.UserID != 0 {
		conds = append(conds, "t.user_id = ?")
		args = append(args, f.UserID)
	}
	if f.AccountID != 0 {
		conds = append(conds, "t.account_id = ?")
		args = append(args, f.AccountID)
	}
	if f.AssetSymbol != "" {
		conds = append(conds, "t.asset_symbol = ?")
		args = append(args, f.AssetSymbol)
	}
	if len(f.Sides) > 0 {
		placeholders := make([]string, len(f.Sides))
		for i, s := range f.Sides {
			placeholders[i] = "?"
			args = append(args, string(s))
		}
		conds = append(conds, "t.side IN ("+strings.Join(placeholders, ",")+")")
	} else if f.Side != "" {
		conds = append(conds, "t.side = ?")
		args = append(args, string(f.Side))
	}
	if !f.From.IsZero() {
		conds = append(conds, "t.occurred_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		conds = append(conds, "t.occurred_at <= ?")
		args = append(args, f.To)
	}
	if f.Q != "" {
		if pat := ftsPattern(f.Q); pat != "" {
			conds = append(conds, "t.id IN (SELECT rowid FROM tx_fts WHERE tx_fts MATCH ?)")
			args = append(args, pat)
		} else {
			// After stripping FTS-special chars the query collapses to
			// nothing — return an empty result instead of matching all.
			conds = append(conds, "1 = 0")
		}
	}
	// Resolve sort + order with safe fallbacks.
	sortKey := f.Sort
	if _, ok := txSortColumns[sortKey]; !ok {
		sortKey = "date"
	}
	order := strings.ToLower(f.Order)
	if order != "asc" {
		order = "desc"
	}
	sortCol := txSortColumns[sortKey]

	// Keyset cursor — only applied when it was emitted for the same
	// sort we're running now (otherwise we'd compare values from a
	// different column). CursorID is the tiebreaker.
	if f.CursorSort == sortKey && f.CursorID != 0 && f.CursorSortVal != "" {
		val, err := parseTxCursorValue(sortKey, f.CursorSortVal)
		if err != nil {
			return nil, fmt.Errorf("cursor: %w", err)
		}
		cmp := "<"
		if order == "asc" {
			cmp = ">"
		}
		conds = append(conds,
			fmt.Sprintf("(%s %s ? OR (%s = ? AND t.id %s ?))",
				sortCol, cmp, sortCol, cmp))
		args = append(args, val, val, f.CursorID)
	}

	q := `SELECT t.id, t.user_id, t.account_id, t.asset_symbol, t.side, t.qty, t.price, t.fee,
	             t.fx_to_base, t.occurred_at, t.note, t.created_at
	        FROM transactions t`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += fmt.Sprintf(" ORDER BY %s %s, t.id %s", sortCol, order, order)
	if f.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", f.Limit)
	}

	rows, err := db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("query tx: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*domain.Transaction
	for rows.Next() {
		t, err := scanTxRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// UpdateTransaction replaces all mutable fields of the given tx (by ID).
func (db *DB) UpdateTransaction(ctx context.Context, t *domain.Transaction) error {
	if !t.Side.Valid() {
		return fmt.Errorf("invalid side %q", t.Side)
	}
	res, err := db.ExecContext(ctx, `
        UPDATE transactions
           SET account_id = ?, asset_symbol = ?, side = ?, qty = ?, price = ?,
               fee = ?, fx_to_base = ?, occurred_at = ?, note = ?
         WHERE id = ?`,
		t.AccountID, t.AssetSymbol, string(t.Side), t.Qty, t.Price,
		t.Fee, t.FxToBase, t.OccurredAt, t.Note, t.ID)
	if err != nil {
		return fmt.Errorf("update tx: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// TxSummary collapses a user's whole transaction history into the
// aggregates the Activities hero wants. Every monetary field is in the
// user's base currency (tx.fx_to_base is locked at trade time).
type TxSummary struct {
	Count          int     `json:"count"`
	AssetCount     int     `json:"asset_count"`
	AccountCount   int     `json:"account_count"`
	TotalBuys      float64 `json:"total_buys"`
	TotalSells     float64 `json:"total_sells"`
	TotalDeposits  float64 `json:"total_deposits"`
	TotalWithdraws float64 `json:"total_withdraws"`
	TotalInterest  float64 `json:"total_interest"`
	BuyCount       int     `json:"buy_count"`
	SellCount      int     `json:"sell_count"`
}

// TransactionSummary returns the aggregate totals for a user in one
// round trip. Uses the idx_tx_user_date index for a cheap scan; the
// CASE-based sums let us avoid five separate queries.
func (db *DB) TransactionSummary(ctx context.Context, userID int64) (*TxSummary, error) {
	var s TxSummary
	err := db.QueryRowContext(ctx, `
        SELECT COUNT(*),
               COUNT(DISTINCT asset_symbol),
               COUNT(DISTINCT account_id),
               COALESCE(SUM(CASE WHEN side = 'buy'      THEN qty*price*fx_to_base ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN side = 'sell'     THEN qty*price*fx_to_base ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN side = 'deposit'  THEN qty*price*fx_to_base ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN side = 'withdraw' THEN qty*price*fx_to_base ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN side = 'interest' THEN qty*price*fx_to_base ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN side = 'buy'  THEN 1 ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN side = 'sell' THEN 1 ELSE 0 END), 0)
          FROM transactions
         WHERE user_id = ?`, userID).
		Scan(&s.Count, &s.AssetCount, &s.AccountCount,
			&s.TotalBuys, &s.TotalSells,
			&s.TotalDeposits, &s.TotalWithdraws, &s.TotalInterest,
			&s.BuyCount, &s.SellCount)
	if err != nil {
		return nil, fmt.Errorf("tx summary: %w", err)
	}
	return &s, nil
}

// DeleteTransaction removes the transaction by id.
func (db *DB) DeleteTransaction(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete tx: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanTxRow(r rowScanner) (*domain.Transaction, error) {
	var (
		t    domain.Transaction
		side string
	)
	if err := r.Scan(&t.ID, &t.UserID, &t.AccountID, &t.AssetSymbol, &side,
		&t.Qty, &t.Price, &t.Fee, &t.FxToBase, &t.OccurredAt, &t.Note, &t.CreatedAt,
	); err != nil {
		return nil, err
	}
	t.Side = domain.TxSide(side)
	return &t, nil
}
