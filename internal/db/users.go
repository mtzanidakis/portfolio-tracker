package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// CreateUser inserts u and sets u.ID + u.CreatedAt on success.
// u.PasswordHash may be empty; an empty hash means the user cannot log
// in from a browser but can still authenticate with an API token.
func (db *DB) CreateUser(ctx context.Context, u *domain.User) error {
	if !u.BaseCurrency.Valid() {
		return fmt.Errorf("invalid base_currency %q", u.BaseCurrency)
	}
	res, err := db.ExecContext(ctx,
		`INSERT INTO users(email, name, password_hash, base_currency) VALUES (?, ?, ?, ?)`,
		u.Email, u.Name, u.PasswordHash, string(u.BaseCurrency),
	)
	if err != nil {
		return fmt.Errorf("insert user: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	u.ID = id
	return db.QueryRowContext(ctx,
		"SELECT created_at FROM users WHERE id = ?", id,
	).Scan(&u.CreatedAt)
}

// GetUser returns the user with the given ID or ErrNotFound.
func (db *DB) GetUser(ctx context.Context, id int64) (*domain.User, error) {
	return db.scanUser(ctx,
		`SELECT id, email, name, password_hash, base_currency, created_at
		   FROM users WHERE id = ?`,
		id,
	)
}

// GetUserByEmail returns the user with the given email or ErrNotFound.
func (db *DB) GetUserByEmail(ctx context.Context, email string) (*domain.User, error) {
	return db.scanUser(ctx,
		`SELECT id, email, name, password_hash, base_currency, created_at
		   FROM users WHERE email = ?`,
		email,
	)
}

// ListUsers returns all users ordered by id.
func (db *DB) ListUsers(ctx context.Context) ([]*domain.User, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, email, name, password_hash, base_currency, created_at
		   FROM users ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var users []*domain.User
	for rows.Next() {
		var (
			u   domain.User
			cur string
		)
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash,
			&cur, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		u.BaseCurrency = domain.Currency(cur)
		users = append(users, &u)
	}
	return users, rows.Err()
}

// UpdateUserBaseCurrency changes a user's reporting currency.
func (db *DB) UpdateUserBaseCurrency(ctx context.Context, id int64, c domain.Currency) error {
	if !c.Valid() {
		return fmt.Errorf("invalid currency %q", c)
	}
	return db.updateOne(ctx,
		`UPDATE users SET base_currency = ? WHERE id = ?`, string(c), id)
}

// UpdateUserProfile changes a user's display name and/or email. Empty
// strings leave the corresponding field untouched.
func (db *DB) UpdateUserProfile(ctx context.Context, id int64, name, email string) error {
	switch {
	case name != "" && email != "":
		return db.updateOne(ctx,
			`UPDATE users SET name = ?, email = ? WHERE id = ?`, name, email, id)
	case name != "":
		return db.updateOne(ctx, `UPDATE users SET name = ? WHERE id = ?`, name, id)
	case email != "":
		return db.updateOne(ctx, `UPDATE users SET email = ? WHERE id = ?`, email, id)
	default:
		return nil
	}
}

// UpdateUserPassword sets a user's password hash.
func (db *DB) UpdateUserPassword(ctx context.Context, id int64, hash string) error {
	return db.updateOne(ctx,
		`UPDATE users SET password_hash = ? WHERE id = ?`, hash, id)
}

// DeleteUser removes the user (and cascades tokens/accounts/transactions/sessions).
func (db *DB) DeleteUser(ctx context.Context, id int64) error {
	return db.updateOne(ctx, `DELETE FROM users WHERE id = ?`, id)
}

func (db *DB) scanUser(ctx context.Context, query string, arg any) (*domain.User, error) {
	var (
		u   domain.User
		cur string
	)
	err := db.QueryRowContext(ctx, query, arg).
		Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &cur, &u.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", err)
	}
	u.BaseCurrency = domain.Currency(cur)
	return &u, nil
}

// updateOne executes a single-row-affecting statement; returns ErrNotFound
// when zero rows were modified.
func (db *DB) updateOne(ctx context.Context, query string, args ...any) error {
	res, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}
