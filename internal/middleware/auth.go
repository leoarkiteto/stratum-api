// Package middleware provides HTTP middleware: JWT authentication,
// role-based authorization, and rate limiting. Additional middleware
// (logging, request ID, panic recovery, CORS) will be added here as the
// API grows.
package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/leoarkiteto/stratum-backend/internal/apierr"
	"github.com/leoarkiteto/stratum-backend/internal/auth"
	"github.com/leoarkiteto/stratum-backend/internal/httpx"
	"github.com/leoarkiteto/stratum-backend/internal/model"
)

// Authenticate validates the Authorization: Bearer <token> header and
// puts the authenticated principal into the request context. Requests
// without a valid token are rejected with 401.
func Authenticate(jwtm *auth.JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := bearerToken(r)
			if !ok {
				writeUnauthorized(w)
				return
			}
			claims, err := jwtm.Parse(token)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			userID, err := strconv.ParseInt(claims.Subject, 10, 64)
			if err != nil {
				writeUnauthorized(w)
				return
			}
			ctx := auth.WithPrincipal(r.Context(), auth.Principal{
				UserID: userID,
				Email:  claims.Email,
				Role:   claims.Role,
			})
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole allows the request only when the authenticated principal's
// role is in the allowed set. Runs after Authenticate.
func RequireRole(allowed ...model.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, ok := auth.PrincipalFrom(r.Context())
			if !ok {
				writeUnauthorized(w)
				return
			}
			for _, role := range allowed {
				if p.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.WriteError(w, apierr.Forbidden("forbidden", "you do not have permission to perform this action"))
		})
	}
}

func bearerToken(r *http.Request) (string, bool) {
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || strings.TrimSpace(token) == "" {
		return "", false
	}
	return strings.TrimSpace(token), true
}

func writeUnauthorized(w http.ResponseWriter) {
	httpx.WriteError(w, apierr.Unauthorized("unauthorized", "invalid or missing access token"))
}
