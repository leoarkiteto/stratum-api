// Package service implements the business logic / use cases of the API.
// It depends on store interfaces, never on HTTP or SQL.
package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/leoarkiteto/stratum-api/internal/apierr"
	"github.com/leoarkiteto/stratum-api/internal/auth"
	"github.com/leoarkiteto/stratum-api/internal/model"
	"github.com/leoarkiteto/stratum-api/internal/store"
)

// UserStore is the persistence interface the auth service needs.
// *store.Store implements it; tests use a hand-written fake.
type UserStore interface {
	CreateUser(ctx context.Context, u *model.User) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
}

// Service is the application service layer for auth use cases.
type Service struct {
	users UserStore
}

// New builds the auth service.
func New(users UserStore) *Service {
	return &Service{users: users}
}

// RegisterParams is the input for Register.
type RegisterParams struct {
	Name     string
	Email    string
	Password string
	Role     model.Role
}

// Register creates a new user and returns it. On success the caller (HTTP
// layer) issues the access token. Role defaults to owner; only owner and
// tenant may be self-selected (syndic is granted by election/admin flow).
func (s *Service) Register(ctx context.Context, p RegisterParams) (*model.User, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	if p.Role == "" {
		p.Role = model.RoleOwner
	}

	if err := validateRegister(p); err != nil {
		return nil, err
	}

	hash, err := auth.HashPassword(p.Password)
	if err != nil {
		return nil, apierr.Internal("hash_failed", "could not hash password", err)
	}

	u := &model.User{
		Email:        p.Email,
		Name:         p.Name,
		Role:         p.Role,
		PasswordHash: hash,
	}
	id, err := s.users.CreateUser(ctx, u)
	if err != nil {
		if errors.Is(err, store.ErrEmailTaken) {
			return nil, apierr.Conflict("email_taken", "an account with this email already exists")
		}
		return nil, apierr.Internal("store_error", "could not create user", err)
	}
	u.ID = id
	return u, nil
}

// Login verifies credentials and returns the user, or an unauthorized
// error. The same error is returned for unknown email and wrong password
// to avoid user enumeration.
func (s *Service) Login(ctx context.Context, email, password string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.users.GetUserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apierr.Unauthorized("invalid_credentials", "invalid email or password")
	}
	if err != nil {
		return nil, apierr.Internal("store_error", "could not load user", err)
	}
	if !auth.VerifyPassword(u.PasswordHash, password) {
		return nil, apierr.Unauthorized("invalid_credentials", "invalid email or password")
	}
	return u, nil
}

// GetUserByID returns a user by id, or a not-found error.
func (s *Service) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	u, err := s.users.GetUserByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) {
		return nil, apierr.NotFound("user_not_found", "user does not exist")
	}
	if err != nil {
		return nil, apierr.Internal("store_error", "could not load user", err)
	}
	return u, nil
}

func validateRegister(p RegisterParams) *apierr.Error {
	if p.Name == "" {
		return apierr.Invalid("invalid_input", "name is required")
	}
	if len([]rune(p.Name)) > 200 {
		return apierr.Invalid("invalid_input", "name must be at most 200 characters")
	}
	if !isValidEmail(p.Email) {
		return apierr.Invalid("invalid_input", "email is not valid")
	}
	if len(p.Password) < auth.MinPasswordBytes {
		return apierr.Invalid("invalid_input", "password must be at least 8 characters")
	}
	if len(p.Password) > auth.MaxPasswordBytes {
		return apierr.Invalid("invalid_input", "password must be at most 72 bytes")
	}
	if p.Role != model.RoleOwner && p.Role != model.RoleTenant {
		return apierr.Invalid("invalid_input", "role must be one of: owner, tenant")
	}
	return nil
}

func isValidEmail(email string) bool {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return false
	}
	// Reject display-name forms like "Foo <foo@bar.com>" — we want a bare address.
	return addr.Address == email
}
