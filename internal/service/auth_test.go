package service

import (
	"context"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum-backend/internal/apierr"
	"github.com/leoarkiteto/stratum-backend/internal/model"
	"github.com/leoarkiteto/stratum-backend/internal/store"
)

// fakeUserStore is an in-memory UserStore for tests (no DB needed).
type fakeUserStore struct {
	users   map[int64]*model.User
	byEmail map[string]*model.User
	nextID  int64
}

func newFakeUserStore() *fakeUserStore {
	return &fakeUserStore{
		users:   map[int64]*model.User{},
		byEmail: map[string]*model.User{},
		nextID:  1,
	}
}

func (f *fakeUserStore) CreateUser(_ context.Context, u *model.User) (int64, error) {
	if _, exists := f.byEmail[u.Email]; exists {
		return 0, store.ErrEmailTaken
	}
	u.ID = f.nextID
	f.nextID++
	now := time.Now()
	u.CreatedAt, u.UpdatedAt = now, now
	f.users[u.ID] = u
	f.byEmail[u.Email] = u
	return u.ID, nil
}

func (f *fakeUserStore) GetUserByEmail(_ context.Context, email string) (*model.User, error) {
	u, ok := f.byEmail[email]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func (f *fakeUserStore) GetUserByID(_ context.Context, id int64) (*model.User, error) {
	u, ok := f.users[id]
	if !ok {
		return nil, store.ErrNotFound
	}
	return u, nil
}

func newTestService() (*Service, *fakeUserStore) {
	fs := newFakeUserStore()
	return New(fs), fs
}

func validRegister() RegisterParams {
	return RegisterParams{Name: "Ana Souza", Email: "ana@example.com", Password: "strong-password", Role: ""}
}

func TestRegisterSuccessDefaultsToOwner(t *testing.T) {
	svc, _ := newTestService()

	u, err := svc.Register(context.Background(), validRegister())
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.ID == 0 {
		t.Error("user should have an id")
	}
	if u.Email != "ana@example.com" {
		t.Errorf("Email = %q, want lowercase normalized", u.Email)
	}
	if u.Role != model.RoleOwner {
		t.Errorf("Role = %q, want default %q", u.Role, model.RoleOwner)
	}
	if u.PasswordHash == "" || u.PasswordHash == "strong-password" {
		t.Error("password must be stored hashed")
	}
}

func TestRegisterExplicitTenantRole(t *testing.T) {
	svc, _ := newTestService()
	p := validRegister()
	p.Role = model.RoleTenant

	u, err := svc.Register(context.Background(), p)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}
	if u.Role != model.RoleTenant {
		t.Errorf("Role = %q, want %q", u.Role, model.RoleTenant)
	}
}

func TestRegisterRejectsSyndicRole(t *testing.T) {
	svc, _ := newTestService()
	p := validRegister()
	p.Role = model.RoleSyndic

	_, err := svc.Register(context.Background(), p)
	ae := apierr.As(err)
	if ae == nil || ae.Status() != 400 {
		t.Fatalf("want 400 invalid_input, got %v", err)
	}
}

func TestRegisterValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*RegisterParams)
	}{
		{"empty name", func(p *RegisterParams) { p.Name = "  " }},
		{"invalid email", func(p *RegisterParams) { p.Email = "not-an-email" }},
		{"display-name email", func(p *RegisterParams) { p.Email = "Ana <ana@example.com>" }},
		{"short password", func(p *RegisterParams) { p.Password = "short" }},
		{"unknown role", func(p *RegisterParams) { p.Role = "root" }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newTestService()
			p := validRegister()
			tc.edit(&p)
			if _, err := svc.Register(context.Background(), p); apierr.As(err) == nil {
				t.Fatalf("expected a 4xx API error, got %v", err)
			}
		})
	}
}

func TestRegisterDuplicateEmail(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register(context.Background(), validRegister()); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	_, err := svc.Register(context.Background(), validRegister())
	ae := apierr.As(err)
	if ae == nil || ae.Code != "email_taken" {
		t.Fatalf("want email_taken conflict, got %v", err)
	}
}

func TestLoginSuccess(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register(context.Background(), validRegister()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	u, err := svc.Login(context.Background(), "ANA@example.com", "strong-password")
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	if u.Email != "ana@example.com" {
		t.Errorf("Email = %q", u.Email)
	}
}

func TestLoginWrongPassword(t *testing.T) {
	svc, _ := newTestService()
	if _, err := svc.Register(context.Background(), validRegister()); err != nil {
		t.Fatalf("Register: %v", err)
	}
	_, err := svc.Login(context.Background(), "ana@example.com", "wrong-password")
	ae := apierr.As(err)
	if ae == nil || ae.Code != "invalid_credentials" || ae.Status() != 401 {
		t.Fatalf("want 401 invalid_credentials, got %v", err)
	}
}

func TestLoginUnknownEmailHasSameError(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.Login(context.Background(), "ghost@example.com", "strong-password")
	ae := apierr.As(err)
	if ae == nil || ae.Code != "invalid_credentials" {
		t.Fatalf("want invalid_credentials (no user enumeration), got %v", err)
	}
}

func TestGetUserByIDNotFound(t *testing.T) {
	svc, _ := newTestService()
	_, err := svc.GetUserByID(context.Background(), 999)
	ae := apierr.As(err)
	if ae == nil || ae.Code != "user_not_found" || ae.Status() != 404 {
		t.Fatalf("want 404 user_not_found, got %v", err)
	}
}
