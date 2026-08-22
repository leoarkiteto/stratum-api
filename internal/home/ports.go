package home

import (
	"context"

	"github.com/leoarkiteto/stratum/internal/shared/model"
)

// UserReader is the read-side contract home needs to resolve the current user.
// auth.PostgresUserRepository implements it structurally, wired in internal/app.
type UserReader interface {
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
}
