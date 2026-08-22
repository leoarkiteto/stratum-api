package home

import (
	"net/http"

	hometpl "github.com/leoarkiteto/stratum/internal/home/templates"
	"github.com/leoarkiteto/stratum/internal/shared/httputil"
	"github.com/leoarkiteto/stratum/internal/shared/session"
)

// Handler exposes the home/dashboard endpoints.
type Handler struct {
	service  *Service
	sessions session.Store
	users    UserReader
	secure   bool
}

// Home renders the landing page, or redirects logged-in users to the dashboard.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	_, present := httputil.UserFrom(r.Context())
	if target := h.service.LandingTarget(present); target != LandingTargetNone {
		http.Redirect(w, r, string(target), http.StatusSeeOther)
		return
	}
	httputil.Render(w, r, hometpl.LandingPage())
}

// Dashboard renders the authenticated dashboard (RequireAuth provides the user).
func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	u, _ := httputil.UserFrom(r.Context())
	httputil.Render(w, r, hometpl.DashboardPage(u))
}

// Hello returns the htmx partial loaded by the dashboard.
func (h *Handler) Hello(w http.ResponseWriter, r *http.Request) {
	u, _ := httputil.UserFrom(r.Context())
	httputil.Render(w, r, hometpl.Hello(u))
}
