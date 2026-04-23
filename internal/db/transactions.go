package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
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
	// Cursor pins a "continue after" position for keyset pagination
	// using the same (occurred_at, id) DESC ordering as the main query.
	// Both fields must be populated to take effect.
	CursorOccurredAt time.Time
	CursorID         int64
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
	// Keyset cursor — only applied when both fields are set. The
	// tuple comparison keeps us aligned with the ORDER BY so rows on
	// the boundary day don't get skipped or duplicated.
	if !f.CursorOccurredAt.IsZero() && f.CursorID != 0 {
		conds = append(conds,
			"(t.occurred_at < ? OR (t.occurred_at = ? AND t.id < ?))")
		args = append(args, f.CursorOccurredAt, f.CursorOccurredAt, f.CursorID)
	}

	q := `SELECT t.id, t.user_id, t.account_id, t.asset_symbol, t.side, t.qty, t.price, t.fee,
	             t.fx_to_base, t.occurred_at, t.note, t.created_at
	        FROM transactions t`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY t.occurred_at DESC, t.id DESC"
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
