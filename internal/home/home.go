// Package home implements the landing and dashboard pages.
package home

import (
	"net/http"

	hometpl "github.com/leoarkiteto/stratum/internal/home/templates"
	"github.com/leoarkiteto/stratum/internal/session"
	"github.com/leoarkiteto/stratum/internal/web"
)

// Deps are the home module's dependencies.
type Deps struct {
	Sessions session.Store
	Users    web.UserStore
	Secure   bool
}

// Handler exposes the home/dashboard endpoints.
type Handler struct {
	sessions session.Store
	users    web.UserStore
	secure   bool
}

// New builds the home module handler.
func New(deps Deps) *Handler {
	return &Handler{sessions: deps.Sessions, users: deps.Users, secure: deps.Secure}
}

// RegisterRoutes wires the home routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.Handle("GET /{$}", web.LoadUser(h.sessions, h.users)(http.HandlerFunc(h.Home)))
	mux.Handle("GET /dashboard", web.RequireAuth(h.sessions, h.users, h.secure)(http.HandlerFunc(h.Dashboard)))
	mux.Handle("GET /dashboard/hello", web.RequireAuth(h.sessions, h.users, h.secure)(http.HandlerFunc(h.Hello)))
}

// Home renders the landing page, or redirects logged-in users to the dashboard.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if _, ok := web.UserFrom(r.Context()); ok {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}
	web.Render(w, r, hometpl.LandingPage())
}

// Dashboard renders the authenticated dashboard (RequireAuth provides the user).
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	web.Render(w, r, hometpl.DashboardPage(u))
}

// Hello returns the htmx partial loaded by the dashboard.
func (h *Handler) Hello(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	web.Render(w, r, hometpl.Hello(u))
}
