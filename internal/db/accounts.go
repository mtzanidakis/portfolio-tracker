package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// CreateAccount inserts acc and sets its ID + CreatedAt.
func (db *DB) CreateAccount(ctx context.Context, acc *domain.Account) error {
	if !acc.Currency.Valid() {
		return fmt.Errorf("invalid currency %q", acc.Currency)
	}
	res, err := db.ExecContext(ctx, `
        INSERT INTO accounts(user_id, name, type, short, color, currency)
        VALUES (?, ?, ?, ?, ?, ?)`,
		acc.UserID, acc.Name, acc.Type, acc.Short, acc.Color,
		string(acc.Currency),
	)
	if err != nil {
		return fmt.Errorf("insert account: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	acc.ID = id
	return db.QueryRowContext(ctx,
		"SELECT created_at FROM accounts WHERE id = ?", id,
	).Scan(&acc.CreatedAt)
}

// GetAccount returns the account by id or ErrNotFound.
func (db *DB) GetAccount(ctx context.Context, id int64) (*domain.Account, error) {
	return db.scanAccount(ctx, `
        SELECT id, user_id, name, type, short, color, currency, created_at
          FROM accounts WHERE id = ?`, id)
}

// ListAccounts returns every account owned by the user, ordered by id.
func (db *DB) ListAccounts(ctx context.Context, userID int64) ([]*domain.Account, error) {
	rows, err := db.QueryContext(ctx, `
        SELECT id, user_id, name, type, short, color, currency, created_at
          FROM accounts WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("query accounts: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*domain.Account
	for rows.Next() {
		acc, err := scanAccountRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, acc)
	}
	return out, rows.Err()
}

// UpdateAccount replaces all mutable fields of an account.
func (db *DB) UpdateAccount(ctx context.Context, acc *domain.Account) error {
	if !acc.Currency.Valid() {
		return fmt.Errorf("invalid currency %q", acc.Currency)
	}
	res, err := db.ExecContext(ctx, `
        UPDATE accounts
           SET name = ?, type = ?, short = ?, color = ?, currency = ?
         WHERE id = ?`,
		acc.Name, acc.Type, acc.Short, acc.Color,
		string(acc.Currency), acc.ID,
	)
	if err != nil {
		return fmt.Errorf("update account: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAccount removes the account (fails if transactions still reference it).
func (db *DB) DeleteAccount(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx, `DELETE FROM accounts WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete account: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanAccountRow(r rowScanner) (*domain.Account, error) {
	var (
		acc domain.Account
		cur string
	)
	if err := r.Scan(&acc.ID, &acc.UserID, &acc.Name, &acc.Type, &acc.Short,
		&acc.Color, &cur, &acc.CreatedAt); err != nil {
		return nil, fmt.Errorf("scan account: %w", err)
	}
	acc.Currency = domain.Currency(cur)
	return &acc, nil
}

func (db *DB) scanAccount(ctx context.Context, query string, arg any) (*domain.Account, error) {
	row := db.QueryRowContext(ctx, query, arg)
	acc, err := scanAccountRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return acc, err
}
