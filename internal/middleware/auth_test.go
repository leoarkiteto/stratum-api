package middleware_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum-backend/internal/auth"
	"github.com/leoarkiteto/stratum-backend/internal/middleware"
	"github.com/leoarkiteto/stratum-backend/internal/model"
)

func TestAuthenticateSetsPrincipal(t *testing.T) {
	jwtm := auth.NewJWTManager("test-secret", time.Hour, "stratum-test")

	var got auth.Principal
	h := middleware.Authenticate(jwtm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := auth.PrincipalFrom(r.Context())
		if !ok {
			t.Error("principal missing from context")
		}
		got = p
		w.WriteHeader(http.StatusNoContent)
	}))

	token, err := jwtm.Issue(&model.User{ID: 7, Email: "a@b.com", Role: model.RoleTenant})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
	want := auth.Principal{UserID: 7, Email: "a@b.com", Role: model.RoleTenant}
	if got != want {
		t.Errorf("principal = %+v, want %+v", got, want)
	}
}

func TestAuthenticateMissingHeader(t *testing.T) {
	jwtm := auth.NewJWTManager("test-secret", time.Hour, "stratum-test")
	h := middleware.Authenticate(jwtm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not run without a token")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestAuthenticateInvalidToken(t *testing.T) {
	jwtm := auth.NewJWTManager("test-secret", time.Hour, "stratum-test")
	h := middleware.Authenticate(jwtm)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not run with an invalid token")
	}))
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer invalid.token.value")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}

func TestRequireRoleAllows(t *testing.T) {
	h := middleware.RequireRole(model.RoleOwner, model.RoleTenant)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: 1, Role: model.RoleOwner})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if rr.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want 204", rr.Code)
	}
}

func TestRequireRoleDenies(t *testing.T) {
	h := middleware.RequireRole(model.RoleSyndic)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not run for a disallowed role")
	}))
	ctx := auth.WithPrincipal(context.Background(), auth.Principal{UserID: 1, Role: model.RoleTenant})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil).WithContext(ctx))
	if rr.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rr.Code)
	}
}

func TestRequireRoleWithoutPrincipal(t *testing.T) {
	h := middleware.RequireRole(model.RoleSyndic)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler must not run without a principal")
	}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest("GET", "/", nil))
	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rr.Code)
	}
}
