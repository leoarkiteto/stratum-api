// Package httputil provides shared HTTP helpers for the server-rendered app:
// templ rendering, session cookies, CSRF, and auth middleware.
package httputil

import (
	"context"

	"github.com/leoarkiteto/stratum/internal/shared/model"
	"github.com/leoarkiteto/stratum/internal/shared/session"
)

type ctxKey struct{ name string }

var (
	userCtxKey    = ctxKey{"user"}
	sessionCtxKey = ctxKey{"session"}
)

// WithUser returns a context carrying the current user.
func WithUser(ctx context.Context, u *model.User) context.Context {
	return context.WithValue(ctx, userCtxKey, u)
}

// UserFrom extracts the current user from ctx, if present.
func UserFrom(ctx context.Context) (*model.User, bool) {
	u, ok := ctx.Value(userCtxKey).(*model.User)
	return u, ok
}

// WithSession returns a context carrying the current session.
func WithSession(ctx context.Context, s *session.Session) context.Context {
	return context.WithValue(ctx, sessionCtxKey, s)
}

// SessionFrom extracts the session from ctx, if present.
func SessionFrom(ctx context.Context) (*session.Session, bool) {
	s, ok := ctx.Value(sessionCtxKey).(*session.Session)
	return s, ok
}
