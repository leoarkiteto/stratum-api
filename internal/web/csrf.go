package web

import (
	"crypto/subtle"
	"net/http"

	"github.com/leoarkiteto/stratum/internal/session"
)

// ValidCSRF reports whether the submitted csrf field matches the session's
// token using a constant-time comparison.
func ValidCSRF(r *http.Request, s *session.Session) bool {
	got := r.FormValue("csrf")
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(s.CSRF)) == 1
}
