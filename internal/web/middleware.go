package web

import (
	"context"
	"net/http"

	"github.com/leoarkiteto/stratum/internal/model"
	"github.com/leoarkiteto/stratum/internal/session"
)

// UserStore is the persistence contract auth middleware needs to load a user.
type UserStore interface {
	GetUserByID(ctx context.Context, id int64) (*model.User, error)
}

// RequireAuth protects a handler: it resolves the session, loads the user and
// puts both into the request context. Unauthenticated requests are redirected
// to /login.
func RequireAuth(sessions session.Store, users UserStore, secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := ReadSessionCookie(r)
			if !ok {
				redirectLogin(w, r)
				return
			}
			s, err := sessions.Get(r.Context(), token)
			if err != nil || s.UserID == 0 {
				ClearSessionCookie(w, secure)
				redirectLogin(w, r)
				return
			}
			u, err := users.GetUserByID(r.Context(), s.UserID)
			if err != nil {
				ClearSessionCookie(w, secure)
				redirectLogin(w, r)
				return
			}
			ctx := WithUser(r.Context(), u)
			ctx = WithSession(ctx, s)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// LoadUser optionally loads the current user from a valid session cookie, but
// does not redirect. Use it on pages that render differently for logged-in
// visitors without requiring authentication.
func LoadUser(sessions session.Store, users UserStore) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token, ok := ReadSessionCookie(r); ok {
				if s, err := sessions.Get(r.Context(), token); err == nil && s.UserID != 0 {
					if u, err := users.GetUserByID(r.Context(), s.UserID); err == nil {
						ctx := WithUser(r.Context(), u)
						ctx = WithSession(ctx, s)
						next.ServeHTTP(w, r.WithContext(ctx))
						return
					}
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireRole protects a handler for a single role. It must wrap inside
// RequireAuth (or LoadUser) so the current user is present in the context.
// Requests with a different role are redirected to redirectTo. Authorization
// is still enforced by the services — this middleware only gates the HTTP edge.
func RequireRole(role model.Role, redirectTo string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			u, ok := UserFrom(r.Context())
			if !ok || u.Role != role {
				if IsHTMX(r) {
					HXRedirect(w, redirectTo)
					return
				}
				http.Redirect(w, r, redirectTo, http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func redirectLogin(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
