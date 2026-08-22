// Package session implements server-side HTTP sessions backed by Postgres.
// The browser holds a random token in an httpOnly cookie; the database stores
// a SHA-256 hash of that token (defense in depth), the optional user id and a
// CSRF token bound to the session.
package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"
)

// ErrNotFound means the session token is unknown or expired.
var ErrNotFound = errors.New("session: not found")

// Session is a server-side session (anonymous when UserID is 0).
type Session struct {
	UserID int64
	CSRF   string
}

// Store is the persistence contract the rest of the app depends on.
// *Manager implements it; tests use an in-memory fake.
type Store interface {
	Create(ctx context.Context) (token, csrf string, err error)
	Get(ctx context.Context, token string) (*Session, error)
	BindUser(ctx context.Context, token string, userID int64) (newToken string, err error)
	Delete(ctx context.Context, token string) error
}

// Manager persists sessions in Postgres.
type Manager struct {
	db  *sql.DB
	ttl time.Duration
}

// New builds a session manager with the given lifetime.
func New(db *sql.DB, ttl time.Duration) *Manager {
	return &Manager{db: db, ttl: ttl}
}

// Create inserts a new anonymous session and returns its raw token and CSRF token.
func (m *Manager) Create(ctx context.Context) (string, string, error) {
	token, csrf := newToken(), newToken()
	_, err := m.db.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, csrf, expires_at) VALUES ($1, NULL, $2, $3)`,
		hash(token), csrf, time.Now().Add(m.ttl),
	)
	if err != nil {
		return "", "", err
	}
	return token, csrf, nil
}

// Get loads the session for a raw token, deleting it if expired.
func (m *Manager) Get(ctx context.Context, token string) (*Session, error) {
	var s Session
	var userID sql.NullInt64
	var expiresAt time.Time
	err := m.db.QueryRowContext(ctx,
		`SELECT user_id, csrf, expires_at FROM sessions WHERE token_hash = $1`, hash(token),
	).Scan(&userID, &s.CSRF, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if !expiresAt.After(time.Now()) {
		_, _ = m.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash(token))
		return nil, ErrNotFound
	}
	if userID.Valid {
		s.UserID = userID.Int64
	}
	return &s, nil
}

// BindUser rotates the session token and attaches the user id. Rotating the
// token prevents session fixation on login.
func (m *Manager) BindUser(ctx context.Context, token string, userID int64) (string, error) {
	s, err := m.Get(ctx, token)
	if err != nil {
		return "", err
	}

	newToken := newToken()
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash(token)); err != nil {
		return "", err
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO sessions (token_hash, user_id, csrf, expires_at) VALUES ($1, $2, $3, $4)`,
		hash(newToken), userID, s.CSRF, time.Now().Add(m.ttl)); err != nil {
		return "", err
	}
	if err := tx.Commit(); err != nil {
		return "", err
	}
	return newToken, nil
}

// Delete removes the session (logout).
func (m *Manager) Delete(ctx context.Context, token string) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, hash(token))
	return err
}

// newToken returns a cryptographically random URL-safe token.
func newToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand failing is unrecoverable.
		panic("session: crypto/rand read failed: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// hash returns the hex SHA-256 of a token for storage.
func hash(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
