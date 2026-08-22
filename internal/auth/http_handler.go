package auth

import (
	"errors"
	"log/slog"
	"net/http"

	authtpl "github.com/leoarkiteto/stratum/internal/auth/templates"
	"github.com/leoarkiteto/stratum/internal/shared/httputil"
	"github.com/leoarkiteto/stratum/internal/shared/model"
	"github.com/leoarkiteto/stratum/internal/shared/session"
)

// LoginPage renders GET /login.
func (h *Handler) LoginPage(w http.ResponseWriter, r *http.Request) {
	csrf, _, err := h.ensureSession(w, r)
	if err != nil {
		h.internalError(w, err)
		return
	}
	httputil.Render(w, r, authtpl.LoginPage(csrf, "", ""))
}

// Login processes POST /login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	token, sess, ok := h.sessionForPost(r)
	if !ok || !httputil.ValidCSRF(r, sess) {
		csrf, _, _ := h.ensureSession(w, r)
		h.renderLoginForm(w, r, csrf, r.FormValue("email"), "Your session expired. Please try again.")
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	u, err := h.service.Login(r.Context(), email, password)
	if err != nil {
		switch {
		case errors.Is(err, ErrInvalidCredentials):
			h.renderLoginForm(w, r, sess.CSRF, email, "Invalid email or password.")
		default:
			slog.Error("login failed", "error", err)
			h.renderLoginForm(w, r, sess.CSRF, email, "Something went wrong. Please try again.")
		}
		return
	}

	h.completeLogin(w, r, token, u)
}

// RegisterPage renders GET /register.
func (h *Handler) RegisterPage(w http.ResponseWriter, r *http.Request) {
	csrf, _, err := h.ensureSession(w, r)
	if err != nil {
		h.internalError(w, err)
		return
	}
	httputil.Render(w, r, authtpl.RegisterPage(csrf, "", "", "owner", ""))
}

// Register processes POST /register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	token, sess, ok := h.sessionForPost(r)
	if !ok || !httputil.ValidCSRF(r, sess) {
		csrf, _, _ := h.ensureSession(w, r)
		h.renderRegisterForm(w, r, csrf, "", "", "owner", "Your session expired. Please try again.")
		return
	}

	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	role := model.Role(r.FormValue("role"))
	if role == "" {
		role = model.RoleOwner
	}

	u, err := h.service.Register(r.Context(), RegisterParams{
		Name:     name,
		Email:    email,
		Password: password,
		Role:     role,
	})
	if err != nil {
		var vErr *ValidationError
		switch {
		case errors.Is(err, ErrEmailTaken):
			h.renderRegisterForm(w, r, sess.CSRF, name, email, string(role), "An account with this email already exists.")
		case errors.As(err, &vErr):
			h.renderRegisterForm(w, r, sess.CSRF, name, email, string(role), vErr.Message)
		default:
			slog.Error("register failed", "error", err)
			h.renderRegisterForm(w, r, sess.CSRF, name, email, string(role), "Something went wrong. Please try again.")
		}
		return
	}

	// Auto-login after registration (token is rotated on bind).
	h.completeLogin(w, r, token, u)
}

// Logout processes POST /logout. It is best-effort: the cookie is cleared
// regardless, so a stale or unknown token cannot trap the user in a session.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if token, ok := httputil.ReadSessionCookie(r); ok {
		_ = h.sessions.Delete(r.Context(), token)
	}
	httputil.ClearSessionCookie(w, h.secure)
	if httputil.IsHTMX(r) {
		httputil.HXRedirect(w, "/")
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// completeLogin binds the anonymous session to the user and redirects home.
func (h *Handler) completeLogin(w http.ResponseWriter, r *http.Request, token string, u *model.User) {
	newToken, err := h.sessions.BindUser(r.Context(), token, u.ID)
	if err != nil {
		h.internalError(w, err)
		return
	}
	httputil.SetSessionCookie(w, newToken, h.sessionTTL, h.secure)
	if httputil.IsHTMX(r) {
		httputil.HXRedirect(w, "/dashboard")
		return
	}
	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// renderLoginForm re-renders the login form, as a partial for htmx requests
// or a full page otherwise.
func (h *Handler) renderLoginForm(w http.ResponseWriter, r *http.Request, csrf, email, errMsg string) {
	if httputil.IsHTMX(r) {
		httputil.Render(w, r, authtpl.LoginForm(csrf, email, errMsg))
		return
	}
	httputil.Render(w, r, authtpl.LoginPage(csrf, email, errMsg))
}

// renderRegisterForm re-renders the register form, as a partial for htmx
// requests or a full page otherwise.
func (h *Handler) renderRegisterForm(w http.ResponseWriter, r *http.Request, csrf, name, email, role, errMsg string) {
	if httputil.IsHTMX(r) {
		httputil.Render(w, r, authtpl.RegisterForm(csrf, name, email, role, errMsg))
		return
	}
	httputil.Render(w, r, authtpl.RegisterPage(csrf, name, email, role, errMsg))
}

// ensureSession returns the csrf (and token) of the current session, creating
// and cookie-ing a fresh anonymous session when none exists.
func (h *Handler) ensureSession(w http.ResponseWriter, r *http.Request) (csrf, token string, err error) {
	if t, ok := httputil.ReadSessionCookie(r); ok {
		if s, e := h.sessions.Get(r.Context(), t); e == nil {
			return s.CSRF, t, nil
		}
	}
	token, csrf, err = h.sessions.Create(r.Context())
	if err != nil {
		return "", "", err
	}
	httputil.SetSessionCookie(w, token, h.sessionTTL, h.secure)
	return csrf, token, nil
}

// sessionForPost loads the session for a POST, reporting whether it is valid.
func (h *Handler) sessionForPost(r *http.Request) (token string, s *session.Session, ok bool) {
	token, present := httputil.ReadSessionCookie(r)
	if !present {
		return "", nil, false
	}
	s, err := h.sessions.Get(r.Context(), token)
	if err != nil {
		return "", nil, false
	}
	return token, s, true
}

func (h *Handler) internalError(w http.ResponseWriter, err error) {
	slog.Error("internal error", "error", err)
	http.Error(w, "internal server error", http.StatusInternalServerError)
}
