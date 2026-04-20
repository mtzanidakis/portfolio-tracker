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
	Side        domain.TxSide
	From        time.Time // inclusive
	To          time.Time // inclusive
	Limit       int       // 0 = no limit
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
func (db *DB) ListTransactions(ctx context.Context, f TxFilter) ([]*domain.Transaction, error) {
	var (
		conds []string
		args  []any
	)
	if f.UserID != 0 {
		conds = append(conds, "user_id = ?")
		args = append(args, f.UserID)
	}
	if f.AccountID != 0 {
		conds = append(conds, "account_id = ?")
		args = append(args, f.AccountID)
	}
	if f.AssetSymbol != "" {
		conds = append(conds, "asset_symbol = ?")
		args = append(args, f.AssetSymbol)
	}
	if f.Side != "" {
		conds = append(conds, "side = ?")
		args = append(args, string(f.Side))
	}
	if !f.From.IsZero() {
		conds = append(conds, "occurred_at >= ?")
		args = append(args, f.From)
	}
	if !f.To.IsZero() {
		conds = append(conds, "occurred_at <= ?")
		args = append(args, f.To)
	}

	q := `SELECT id, user_id, account_id, asset_symbol, side, qty, price, fee,
	             fx_to_base, occurred_at, note, created_at
	        FROM transactions`
	if len(conds) > 0 {
		q += " WHERE " + strings.Join(conds, " AND ")
	}
	q += " ORDER BY occurred_at DESC, id DESC"
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
