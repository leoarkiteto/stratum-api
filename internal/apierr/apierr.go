// Package apierr defines the API error model shared by all layers.
// Every error that reaches the HTTP layer is an *Error carrying a Kind,
// a machine-readable Code and a human-readable Message.
package apierr

import (
	"errors"
	"net/http"
)

// Kind classifies an error for HTTP status mapping.
type Kind int

const (
	// KindInvalid maps to 400 Bad Request.
	KindInvalid Kind = iota
	// KindUnauthorized maps to 401 Unauthorized.
	KindUnauthorized
	// KindForbidden maps to 403 Forbidden.
	KindForbidden
	// KindNotFound maps to 404 Not Found.
	KindNotFound
	// KindConflict maps to 409 Conflict.
	KindConflict
	// KindInternal maps to 500 Internal Server Error.
	KindInternal
	// KindTooManyRequests maps to 429 Too Many Requests.
	KindTooManyRequests
)

// Error is a structured API error.
type Error struct {
	Err     error
	Code    string
	Message string
	Kind    Kind
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the wrapped cause, if any.
func (e *Error) Unwrap() error { return e.Err }

// Status returns the HTTP status code for the error's kind.
func (e *Error) Status() int {
	switch e.Kind {
	case KindInvalid:
		return http.StatusBadRequest
	case KindUnauthorized:
		return http.StatusUnauthorized
	case KindForbidden:
		return http.StatusForbidden
	case KindNotFound:
		return http.StatusNotFound
	case KindConflict:
		return http.StatusConflict
	case KindTooManyRequests:
		return http.StatusTooManyRequests
	default:
		return http.StatusInternalServerError
	}
}

// Invalid builds a 400 error.
func Invalid(code, message string) *Error {
	return &Error{Kind: KindInvalid, Code: code, Message: message}
}

// Unauthorized builds a 401 error.
func Unauthorized(code, message string) *Error {
	return &Error{Kind: KindUnauthorized, Code: code, Message: message}
}

// Forbidden builds a 403 error.
func Forbidden(code, message string) *Error {
	return &Error{Kind: KindForbidden, Code: code, Message: message}
}

// NotFound builds a 404 error.
func NotFound(code, message string) *Error {
	return &Error{Kind: KindNotFound, Code: code, Message: message}
}

// Conflict builds a 409 error.
func Conflict(code, message string) *Error {
	return &Error{Kind: KindConflict, Code: code, Message: message}
}

// Internal builds a 500 error, wrapping the original cause for logging.
func Internal(code, message string, err error) *Error {
	return &Error{Kind: KindInternal, Code: code, Message: message, Err: err}
}

// TooManyRequests builds a 429 error.
func TooManyRequests(code, message string) *Error {
	return &Error{Kind: KindTooManyRequests, Code: code, Message: message}
}

// As extracts an *Error from err, returning nil if it is not one.
func As(err error) *Error {
	if e, ok := errors.AsType[*Error](err); ok {
		return e
	}
	return nil
}
