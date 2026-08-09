package auth

import (
	"context"
	"testing"

	"github.com/leoarkiteto/stratum-backend/internal/model"
)

func TestRoleValidity(t *testing.T) {
	for _, role := range []model.Role{model.RoleSyndic, model.RoleOwner, model.RoleTenant} {
		if !role.IsValid() {
			t.Errorf("%q should be valid", role)
		}
	}
	for _, role := range []model.Role{"admin", "guest", ""} {
		if role.IsValid() {
			t.Errorf("%q should not be valid", role)
		}
	}
}

func TestParseRole(t *testing.T) {
	if r, ok := ParseRole("owner"); !ok || r != model.RoleOwner {
		t.Errorf(`ParseRole("owner") = %q, %v; want owner, true`, r, ok)
	}
	if _, ok := ParseRole("root"); ok {
		t.Error(`ParseRole("root") should fail`)
	}
}

func TestRegistrableRolesExcludeSyndic(t *testing.T) {
	for _, r := range RegistrableRoles {
		if r == model.RoleSyndic {
			t.Error("syndic must not be self-selectable at registration")
		}
	}
}

func TestPrincipalContextRoundtrip(t *testing.T) {
	want := Principal{UserID: 7, Email: "a@b.com", Role: model.RoleTenant}
	ctx := WithPrincipal(context.Background(), want)

	got, ok := PrincipalFrom(ctx)
	if !ok {
		t.Fatal("PrincipalFrom should find the principal")
	}
	if got != want {
		t.Errorf("PrincipalFrom = %+v, want %+v", got, want)
	}
}

func TestPrincipalFromEmptyContext(t *testing.T) {
	if _, ok := PrincipalFrom(context.Background()); ok {
		t.Fatal("PrincipalFrom on an empty context must return ok=false")
	}
}
