package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leoarkiteto/stratum-backend/internal/model"
)

const userColumns = "id, email, name, role, password_hash, created_at, updated_at"

// CreateUser inserts a user and fills in the generated id and timestamps.
func (s *Store) CreateUser(ctx context.Context, u *model.User) (int64, error) {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, name, role, password_hash)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at`,
		u.Email, u.Name, string(u.Role), u.PasswordHash,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if isUniqueViolation(err) {
			return 0, ErrEmailTaken
		}
		return 0, err
	}
	return u.ID, nil
}

// GetUserByEmail returns the user with the given email, or ErrNotFound.
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*model.User, error) {
	u := &model.User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE email = $1", email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// GetUserByID returns the user with the given id, or ErrNotFound.
func (s *Store) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	u := &model.User{}
	err := s.db.QueryRowContext(ctx,
		"SELECT "+userColumns+" FROM users WHERE id = $1", id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.Role, &u.PasswordHash, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

// isUniqueViolation reports whether err is a Postgres unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505" && strings.Contains(pgErr.ConstraintName, "users_email_key")
}
