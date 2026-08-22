// Package auth implements the authentication feature: registration, login,
// logout and password handling, rendered server-side with Templ + HTMX.
package auth

import "errors"

// Domain errors, mapped by handlers to user-facing messages.
var (
	// ErrNotFound means the requested user does not exist.
	ErrNotFound = errors.New("auth: user not found")
	// ErrEmailTaken means the email is already registered.
	ErrEmailTaken = errors.New("auth: email already taken")
	// ErrInvalidCredentials is returned for unknown email or wrong password
	// (same error for both, to avoid user enumeration).
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
)

// ValidationError is a user-input error carrying a human-readable message.
type ValidationError struct{ Message string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Message }
