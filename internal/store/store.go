// Package store implements Postgres persistence via database/sql + pgx.
package store

import (
	"database/sql"
	"errors"
)

// Sentinel domain errors returned by store methods. Services/handlers map
// these to API errors; callers must compare with errors.Is.
var (
	// ErrNotFound means the requested row does not exist.
	ErrNotFound = errors.New("store: not found")
	// ErrEmailTaken means a unique constraint on email was violated.
	ErrEmailTaken = errors.New("store: email already taken")
)

// Store provides access to all aggregates. One Store per process.
type Store struct {
	db *sql.DB
}

// New builds a Store on the given database handle.
func New(db *sql.DB) *Store {
	return &Store{db: db}
}
