// Package auth implements the authentication feature: registration, login,
// logout and password handling, rendered server-side with Templ + HTMX.
package auth

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/leoarkiteto/stratum/internal/model"
	"github.com/leoarkiteto/stratum/internal/password"
)

// Password policy bounds. Argon2id has no input-length limit (unlike bcrypt),
// so the max is only a DoS guard against absurdly long passwords.
const (
	minPasswordBytes = 8
	maxPasswordBytes = 1024
)

// UserStore is the persistence contract the auth service needs.
// *Store implements it; tests use a hand-written fake.
type UserStore interface {
	CreateUser(ctx context.Context, u *model.User) (int64, error)
	GetUserByEmail(ctx context.Context, email string) (*model.User, error)
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
}

// Service is the auth business logic.
type Service struct {
	users  UserStore
	hasher *password.Hasher
}

// NewService builds the auth service.
func NewService(users UserStore, hasher *password.Hasher) *Service {
	return &Service{users: users, hasher: hasher}
}

// RegisterParams is the input for Register.
type RegisterParams struct {
	Name     string
	Email    string
	Password string
	Role     model.Role
}

// Register creates a new user and returns it. Role defaults to owner; only
// owner and tenant may self-select (syndic is granted out of band).
func (s *Service) Register(ctx context.Context, p RegisterParams) (*model.User, error) {
	p.Name = strings.TrimSpace(p.Name)
	p.Email = strings.ToLower(strings.TrimSpace(p.Email))
	if p.Role == "" {
		p.Role = model.RoleOwner
	}

	if err := validateRegister(p); err != nil {
		return nil, err
	}

	hash, err := s.hasher.Hash(p.Password)
	if err != nil {
		return nil, err
	}

	u := &model.User{
		Email:        p.Email,
		Name:         p.Name,
		Role:         p.Role,
		PasswordHash: hash,
	}
	id, err := s.users.CreateUser(ctx, u)
	if err != nil {
		if errors.Is(err, ErrEmailTaken) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	u.ID = id
	return u, nil
}

// Login verifies credentials and returns the user. Unknown email and wrong
// password return the same error to avoid user enumeration.
func (s *Service) Login(ctx context.Context, email, password string) (*model.User, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	u, err := s.users.GetUserByEmail(ctx, email)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !s.hasher.Verify(u.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}

// GetUserByID returns a user by id, or ErrNotFound.
func (s *Service) GetUserByID(ctx context.Context, id int64) (*model.User, error) {
	u, err := s.users.GetUserByID(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return u, nil
}

func validateRegister(p RegisterParams) error {
	if p.Name == "" {
		return &ValidationError{Message: "Name is required."}
	}
	if len([]rune(p.Name)) > 200 {
		return &ValidationError{Message: "Name must be at most 200 characters."}
	}
	if !isValidEmail(p.Email) {
		return &ValidationError{Message: "Please enter a valid email address."}
	}
	if len(p.Password) < minPasswordBytes {
		return &ValidationError{Message: "Password must be at least 8 characters."}
	}
	if len(p.Password) > maxPasswordBytes {
		return &ValidationError{Message: "Password must be at most 1024 characters."}
	}
	if p.Role != model.RoleOwner && p.Role != model.RoleTenant {
		return &ValidationError{Message: "Role must be one of: owner, tenant."}
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
