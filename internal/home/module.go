// Package home implements the landing and dashboard pages.
package home

import (
	"net/http"

	"github.com/leoarkiteto/stratum/internal/shared/httputil"
	"github.com/leoarkiteto/stratum/internal/shared/session"
)

// Deps are the home module's dependencies.
type Deps struct {
	Sessions session.Store
	Users    UserReader
	Secure   bool
}

// New builds the home module handler.
func New(deps Deps) *Handler {
	return &Handler{
		service:  NewService(),
		sessions: deps.Sessions,
		users:    deps.Users,
		secure:   deps.Secure,
	}
}

// RegisterRoutes wires the home routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /{$}", httputil.LoadUser(h.sessions, h.users)(http.HandlerFunc(h.Home)))
	mux.Handle("GET /dashboard", httputil.RequireAuth(h.sessions, h.users, h.secure)(http.HandlerFunc(h.Dashboard)))
	mux.Handle("GET /dashboard/hello", httputil.RequireAuth(h.sessions, h.users, h.secure)(http.HandlerFunc(h.Hello)))
}
