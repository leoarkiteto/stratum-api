package auth

import (
	"context"

	"github.com/leoarkiteto/stratum/internal/shared/model"
)

// UserStore is the persistence contract the auth service needs.
// *PostgresUserRepository implements it; tests use a hand-written fake.
type UserStore interface {
	CreateUser(ctx context.Context, u *model.User) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
}
