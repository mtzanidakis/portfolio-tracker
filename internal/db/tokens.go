package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// CreateToken inserts a token row. The caller supplies the already-computed
// hash; the plaintext token is never stored.
func (db *DB) CreateToken(ctx context.Context, tok *domain.Token) error {
	res, err := db.ExecContext(ctx,
		`INSERT INTO tokens(user_id, name, hash) VALUES (?, ?, ?)`,
		tok.UserID, tok.Name, tok.Hash,
	)
	if err != nil {
		return fmt.Errorf("insert token: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	tok.ID = id
	return db.QueryRowContext(ctx,
		"SELECT created_at FROM tokens WHERE id = ?", id,
	).Scan(&tok.CreatedAt)
}

// GetTokenByHash returns the (non-revoked) token matching the given hash, or
// ErrNotFound if no active token exists.
func (db *DB) GetTokenByHash(ctx context.Context, hash string) (*domain.Token, error) {
	return db.scanToken(ctx,
		`SELECT id, user_id, name, hash, created_at, last_used_at, revoked_at
		   FROM tokens WHERE hash = ? AND revoked_at IS NULL`, hash)
}

// GetToken returns a token by id regardless of revocation.
func (db *DB) GetToken(ctx context.Context, id int64) (*domain.Token, error) {
	return db.scanToken(ctx,
		`SELECT id, user_id, name, hash, created_at, last_used_at, revoked_at
		   FROM tokens WHERE id = ?`, id)
}

// ListTokens returns all tokens for a user (including revoked).
func (db *DB) ListTokens(ctx context.Context, userID int64) ([]*domain.Token, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT id, user_id, name, hash, created_at, last_used_at, revoked_at
		   FROM tokens WHERE user_id = ? ORDER BY id`, userID)
	if err != nil {
		return nil, fmt.Errorf("query tokens: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var tokens []*domain.Token
	for rows.Next() {
		tok, err := scanTokenRow(rows)
		if err != nil {
			return nil, err
		}
		tokens = append(tokens, tok)
	}
	return tokens, rows.Err()
}

// TouchToken sets last_used_at = now for the given id. Does not error if the
// token is missing (best-effort).
func (db *DB) TouchToken(ctx context.Context, id int64) error {
	_, err := db.ExecContext(ctx,
		`UPDATE tokens SET last_used_at = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// RevokeToken marks a token as revoked. Re-revoking a revoked token is a no-op.
func (db *DB) RevokeToken(ctx context.Context, id int64) error {
	res, err := db.ExecContext(ctx,
		`UPDATE tokens SET revoked_at = CURRENT_TIMESTAMP
		   WHERE id = ? AND revoked_at IS NULL`, id)
	if err != nil {
		return fmt.Errorf("revoke token: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		// Distinguish between "not found" and "already revoked".
		var exists int
		if err := db.QueryRowContext(ctx,
			"SELECT 1 FROM tokens WHERE id = ?", id).Scan(&exists); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
	}
	return nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanTokenRow(r rowScanner) (*domain.Token, error) {
	var (
		tok       domain.Token
		lastUsed  sql.NullTime
		revokedAt sql.NullTime
	)
	if err := r.Scan(&tok.ID, &tok.UserID, &tok.Name, &tok.Hash, &tok.CreatedAt,
		&lastUsed, &revokedAt); err != nil {
		return nil, fmt.Errorf("scan token: %w", err)
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		tok.LastUsedAt = &t
	}
	if revokedAt.Valid {
		t := revokedAt.Time
		tok.RevokedAt = &t
	}
	return &tok, nil
}

func (db *DB) scanToken(ctx context.Context, query string, arg any) (*domain.Token, error) {
	row := db.QueryRowContext(ctx, query, arg)
	tok, err := scanTokenRow(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		// Disambiguate sql.ErrNoRows that's already wrapped by scanTokenRow.
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return tok, nil
}
