package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/mtzanidakis/portfolio-tracker/internal/domain"
)

// CreateSession inserts a new session with the given ID and expiry.
func (db *DB) CreateSession(ctx context.Context, s *domain.Session) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO sessions(id, user_id, expires_at) VALUES (?, ?, ?)`,
		s.ID, s.UserID, s.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("insert session: %w", err)
	}
	return db.QueryRowContext(ctx,
		"SELECT created_at FROM sessions WHERE id = ?", s.ID,
	).Scan(&s.CreatedAt)
}

// GetSession returns the session if it exists and has not expired.
func (db *DB) GetSession(ctx context.Context, id string) (*domain.Session, error) {
	var (
		s        domain.Session
		lastUsed sql.NullTime
	)
	err := db.QueryRowContext(ctx, `
        SELECT id, user_id, created_at, expires_at, last_used_at
          FROM sessions
         WHERE id = ? AND expires_at > CURRENT_TIMESTAMP`, id).
		Scan(&s.ID, &s.UserID, &s.CreatedAt, &s.ExpiresAt, &lastUsed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("scan session: %w", err)
	}
	if lastUsed.Valid {
		t := lastUsed.Time
		s.LastUsedAt = &t
	}
	return &s, nil
}

// TouchSession updates last_used_at and pushes expires_at forward
// (sliding window).
func (db *DB) TouchSession(ctx context.Context, id string, newExpires time.Time) error {
	return db.updateOne(ctx, `
        UPDATE sessions
           SET last_used_at = CURRENT_TIMESTAMP,
               expires_at   = ?
         WHERE id = ?`, newExpires, id)
}

// DeleteSession removes a single session (used by logout).
func (db *DB) DeleteSession(ctx context.Context, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteUserSessionsExcept removes every session belonging to userID
// except keepID. Intended for use after a password change: the caller
// retains the current session while all other devices are logged out.
func (db *DB) DeleteUserSessionsExcept(ctx context.Context, userID int64, keepID string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM sessions WHERE user_id = ? AND id != ?`, userID, keepID)
	if err != nil {
		return fmt.Errorf("delete user sessions: %w", err)
	}
	return nil
}

// PurgeExpiredSessions deletes any session past its expires_at. Returns
// the number of rows removed.
func (db *DB) PurgeExpiredSessions(ctx context.Context) (int64, error) {
	res, err := db.ExecContext(ctx,
		`DELETE FROM sessions WHERE expires_at <= CURRENT_TIMESTAMP`)
	if err != nil {
		return 0, fmt.Errorf("purge sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
