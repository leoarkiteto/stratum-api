// Package server wires the HTTP server: routes, middleware, startup wiring.
package server

import (
	"net/http"
	"time"

	"github.com/leoarkiteto/stratum-api/internal/auth"
	"github.com/leoarkiteto/stratum-api/internal/handler"
	"github.com/leoarkiteto/stratum-api/internal/middleware"
	"github.com/leoarkiteto/stratum-api/internal/service"
)

// Credential endpoint throttling (per client IP, single instance).
const (
	loginRateLimit    = 10
	registerRateLimit = 5
	rateLimitWindow   = time.Minute
)

// Deps are the server's dependencies.
type Deps struct {
	Store service.UserStore
	JWT   *auth.JWTManager
}

// New builds the root handler with all routes wired.
func New(deps Deps) http.Handler {
	svc := service.New(deps.Store)
	h := handler.New(handler.Deps{Service: svc, JWT: deps.JWT})

	mux := http.NewServeMux()
	mux.Handle("POST /api/v1/auth/register",
		middleware.RateLimit(middleware.NewRateLimiter(registerRateLimit, rateLimitWindow))(
			http.HandlerFunc(h.Register)))
	mux.Handle("POST /api/v1/auth/login",
		middleware.RateLimit(middleware.NewRateLimiter(loginRateLimit, rateLimitWindow))(
			http.HandlerFunc(h.Login)))
	mux.Handle("GET /api/v1/auth/me", middleware.Authenticate(deps.JWT)(http.HandlerFunc(h.Me)))
	return mux
}
