package election

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	electpl "github.com/leoarkiteto/stratum/internal/election/templates"
	"github.com/leoarkiteto/stratum/internal/model"
	"github.com/leoarkiteto/stratum/internal/session"
	"github.com/leoarkiteto/stratum/internal/web"
)

// Handler exposes the election HTTP endpoints.
type Handler struct {
	service  *Service
	sessions session.Store
	users    web.UserStore
	secure   bool
}

// RegisterRoutes wires the election routes, all behind RequireAuth.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	requireAuth := web.RequireAuth(h.sessions, h.users, h.secure)
	syndicOnly := web.RequireRole(model.RoleSyndic, "/elections")
	mux.Handle("GET /elections", requireAuth(http.HandlerFunc(h.ListPage)))
	mux.Handle("GET /elections/new", requireAuth(syndicOnly(http.HandlerFunc(h.NewPage))))
	mux.Handle("POST /elections", requireAuth(syndicOnly(http.HandlerFunc(h.Create))))
	mux.Handle("GET /elections/{id}", requireAuth(http.HandlerFunc(h.DetailPage)))
	mux.Handle("POST /elections/{id}/candidates", requireAuth(http.HandlerFunc(h.RegisterCandidate)))
	mux.Handle("POST /elections/{id}/candidates/withdraw", requireAuth(http.HandlerFunc(h.WithdrawCandidate)))
	mux.Handle("POST /elections/{id}/open-voting", requireAuth(http.HandlerFunc(h.OpenVoting)))
	mux.Handle("POST /elections/{id}/vote", requireAuth(http.HandlerFunc(h.Vote)))
	mux.Handle("POST /elections/{id}/close", requireAuth(http.HandlerFunc(h.Close)))
}

// ListPage renders the election list, settling any finished handover periods first.
func (h *Handler) ListPage(w http.ResponseWriter, r *http.Request) {
	h.settle(r)
	u, _ := web.UserFrom(r.Context())
	elections, err := h.service.ListElections(r.Context())
	if err != nil {
		h.internalError(w, err)
		return
	}
	web.Render(w, r, electpl.ElectionsPage(u, elections))
}

// NewPage renders the create-election form (RequireRole gates it for syndics).
func (h *Handler) NewPage(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	sess, _ := web.SessionFrom(r.Context())
	web.Render(w, r, electpl.NewPage(u, sess.CSRF, "", "", "7", ""))
}

// Create processes POST /elections (RequireRole gates it for syndics; the
// service re-checks and is the source of truth).
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	sess, ok := web.SessionFrom(r.Context())
	if !ok || !web.ValidCSRF(r, sess) {
		csrf := ""
		if ok {
			csrf = sess.CSRF
		}
		h.renderNewForm(w, r, u, csrf, "", "", "7", "Your session expired. Please try again.")
		return
	}

	title := r.FormValue("title")
	description := r.FormValue("description")
	transitionDays, err := parseTransitionDays(r.FormValue("transition_days"))
	if err != nil {
		h.renderNewForm(w, r, u, sess.CSRF, title, description, r.FormValue("transition_days"), userMessage(err))
		return
	}
	e, err := h.service.CreateElection(r.Context(), u, title, description, transitionDays)
	if err != nil {
		h.renderNewForm(w, r, u, sess.CSRF, title, description, r.FormValue("transition_days"), userMessage(err))
		return
	}
	h.redirect(w, r, fmt.Sprintf("/elections/%d", e.ID))
}

// DetailPage renders a single election with its candidates and actions,
// settling any finished handover period first.
func (h *Handler) DetailPage(w http.ResponseWriter, r *http.Request) {
	h.settle(r)
	u, _ := web.UserFrom(r.Context())
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	v, err := h.service.Detail(r.Context(), u, id)
	if errors.Is(err, ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		h.internalError(w, err)
		return
	}
	web.Render(w, r, electpl.DetailPage(u, v, "", ""))
}

// settle promotes winners whose handover period has ended. Failures are
// logged but never block the page: the background loop retries.
func (h *Handler) settle(r *http.Request) {
	if err := h.service.SettleTransitions(r.Context()); err != nil {
		slog.Error("settle transitions", "error", err)
	}
}

// parseTransitionDays parses the create-form field; empty means the 7-day default.
func parseTransitionDays(v string) (int, error) {
	v = strings.TrimSpace(v)
	if v == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return 0, &ValidationError{Message: "Transition period must be a whole number of days."}
	}
	return n, nil
}

// RegisterCandidate processes POST /elections/{id}/candidates.
func (h *Handler) RegisterCandidate(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	id, ok := h.validatedPost(w, r, u)
	if !ok {
		return
	}
	if err := h.service.RegisterCandidate(r.Context(), u, id, r.FormValue("statement")); err != nil {
		h.renderDetailErr(w, r, u, id, userMessage(err))
		return
	}
	h.redirect(w, r, fmt.Sprintf("/elections/%d", id))
}

// WithdrawCandidate processes POST /elections/{id}/candidates/withdraw.
func (h *Handler) WithdrawCandidate(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	id, ok := h.validatedPost(w, r, u)
	if !ok {
		return
	}
	if err := h.service.WithdrawCandidate(r.Context(), u, id); err != nil {
		h.renderDetailErr(w, r, u, id, userMessage(err))
		return
	}
	h.redirect(w, r, fmt.Sprintf("/elections/%d", id))
}

// OpenVoting processes POST /elections/{id}/open-voting.
func (h *Handler) OpenVoting(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	id, ok := h.validatedPost(w, r, u)
	if !ok {
		return
	}
	if err := h.service.OpenVoting(r.Context(), u, id); err != nil {
		h.renderDetailErr(w, r, u, id, userMessage(err))
		return
	}
	h.redirect(w, r, fmt.Sprintf("/elections/%d", id))
}

// Vote processes POST /elections/{id}/vote.
func (h *Handler) Vote(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	id, ok := h.validatedPost(w, r, u)
	if !ok {
		return
	}
	candidateID, err := strconv.ParseInt(r.FormValue("candidate_id"), 10, 64)
	if err != nil {
		h.renderDetailErr(w, r, u, id, "Please select a candidate.")
		return
	}
	if err := h.service.Vote(r.Context(), u, id, candidateID); err != nil {
		h.renderDetailErr(w, r, u, id, userMessage(err))
		return
	}
	h.redirect(w, r, fmt.Sprintf("/elections/%d", id))
}

// Close processes POST /elections/{id}/close.
func (h *Handler) Close(w http.ResponseWriter, r *http.Request) {
	u, _ := web.UserFrom(r.Context())
	id, ok := h.validatedPost(w, r, u)
	if !ok {
		return
	}
	if err := h.service.CloseElection(r.Context(), u, id); err != nil {
		h.renderDetailErr(w, r, u, id, userMessage(err))
		return
	}
	h.redirect(w, r, fmt.Sprintf("/elections/%d", id))
}

// validatedPost checks that the request has a numeric election id and a valid
// session + CSRF, re-rendering the detail page with an error otherwise.
func (h *Handler) validatedPost(w http.ResponseWriter, r *http.Request, u *model.User) (int64, bool) {
	id, err := pathID(r)
	if err != nil {
		http.NotFound(w, r)
		return 0, false
	}
	sess, hasSession := web.SessionFrom(r.Context())
	if !hasSession || !web.ValidCSRF(r, sess) {
		h.renderDetailErr(w, r, u, id, "Your session expired. Please try again.")
		return 0, false
	}
	return id, true
}

func pathID(r *http.Request) (int64, error) {
	return strconv.ParseInt(r.PathValue("id"), 10, 64)
}

// renderDetailErr re-renders the election detail page with an error message,
// as a partial for htmx requests or a full page otherwise.
func (h *Handler) renderDetailErr(w http.ResponseWriter, r *http.Request, u *model.User, id int64, errMsg string) {
	v, err := h.service.Detail(r.Context(), u, id)
	if err != nil {
		h.internalError(w, err)
		return
	}
	sess, _ := web.SessionFrom(r.Context())
	if web.IsHTMX(r) {
		web.Render(w, r, electpl.DetailSection(v, sess.CSRF, errMsg))
		return
	}
	web.Render(w, r, electpl.DetailPage(u, v, sess.CSRF, errMsg))
}

// renderNewForm re-renders the create form, as a partial for htmx requests
// or a full page otherwise.
func (h *Handler) renderNewForm(w http.ResponseWriter, r *http.Request, u *model.User, csrf, title, description, transitionDays, errMsg string) {
	if web.IsHTMX(r) {
		web.Render(w, r, electpl.NewForm(csrf, title, description, transitionDays, errMsg))
		return
	}
	web.Render(w, r, electpl.NewPage(u, csrf, title, description, transitionDays, errMsg))
}

// redirect navigates to path, using HX-Redirect for htmx requests.
func (h *Handler) redirect(w http.ResponseWriter, r *http.Request, path string) {
	if web.IsHTMX(r) {
		web.HXRedirect(w, path)
		return
	}
	http.Redirect(w, r, path, http.StatusSeeOther)
}

// userMessage maps a domain error to a user-facing message.
func userMessage(err error) string {
	var vErr *ValidationError
	switch {
	case errors.As(err, &vErr):
		return vErr.Message
	case errors.Is(err, ErrNotSyndic):
		return "Only the syndic can do this."
	case errors.Is(err, ErrNotOwner):
		return "Only owners can vote in this election."
	case errors.Is(err, ErrNotEligible):
		return "Only owners and the current syndic can run for syndic."
	case errors.Is(err, ErrWrongPhase):
		return "This action is not available at the current stage of the election."
	case errors.Is(err, ErrAlreadyCandidate):
		return "You are already registered as a candidate."
	case errors.Is(err, ErrNotCandidate):
		return "You are not registered as a candidate in this election."
	case errors.Is(err, ErrAlreadyVoted):
		return "You have already voted in this election."
	case errors.Is(err, ErrInvalidCandidate):
		return "Unknown candidate."
	default:
		slog.Error("election action failed", "error", err)
		return "Something went wrong. Please try again."
	}
}

func (h *Handler) internalError(w http.ResponseWriter, err error) {
	slog.Error("internal error", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
