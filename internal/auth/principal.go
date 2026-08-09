package auth

import (
	"context"

	"github.com/leoarkiteto/stratum-api/internal/model"
)

// Principal is the authenticated identity attached to the request context
// by the auth middleware.
type Principal struct {
	Email  string
	Role   model.Role
	UserID int64
}

type principalCtxKey struct{}

// WithPrincipal returns a context carrying the authenticated principal.
func WithPrincipal(ctx context.Context, p Principal) context.Context {
	return context.WithValue(ctx, principalCtxKey{}, p)
}

// PrincipalFrom extracts the principal from ctx, if present.
func PrincipalFrom(ctx context.Context) (Principal, bool) {
	p, ok := ctx.Value(principalCtxKey{}).(Principal)
	return p, ok
}

// ParseRole parses a role name, reporting whether it is known.
func ParseRole(s string) (model.Role, bool) {
	r := model.Role(s)
	return r, r.IsValid()
}

// RegistrableRoles are the roles a user may self-select at registration.
// Syndic is intentionally excluded: that role is granted by election or
// by an administrator flow, not by self-service signup.
var RegistrableRoles = []model.Role{model.RoleOwner, model.RoleTenant}
