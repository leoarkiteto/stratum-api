package auth

import (
	"net/http"
	"time"

	"github.com/leoarkiteto/stratum/internal/shared/password"
	"github.com/leoarkiteto/stratum/internal/shared/session"
)

// Deps are the auth module's dependencies.
type Deps struct {
	Users      UserStore
	Sessions   session.Store
	Hasher     *password.Hasher
	Secure     bool
	SessionTTL time.Duration
}

// Handler exposes the auth HTTP endpoints.
type Handler struct {
	service    *Service
	sessions   session.Store
	secure     bool
	sessionTTL time.Duration
}

// New builds the auth module handler.
func New(deps Deps) *Handler {
	return &Handler{
		service:    NewService(deps.Users, deps.Hasher),
		sessions:   deps.Sessions,
		secure:     deps.Secure,
		sessionTTL: deps.SessionTTL,
	}
}

// RegisterRoutes wires the auth routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /login", h.LoginPage)
	mux.HandleFunc("POST /login", h.Login)
	mux.HandleFunc("GET /register", h.RegisterPage)
	mux.HandleFunc("POST /register", h.Register)
	mux.HandleFunc("POST /logout", h.Logout)
}
