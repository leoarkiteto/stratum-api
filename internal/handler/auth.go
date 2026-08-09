// Package handler implements thin HTTP handlers: parse/validate the
// request, call exactly one service method, map result/error to JSON.
package handler

import (
	"encoding/json"
	"net/http"

	"github.com/leoarkiteto/stratum-backend/internal/apierr"
	"github.com/leoarkiteto/stratum-backend/internal/auth"
	"github.com/leoarkiteto/stratum-backend/internal/httpx"
	"github.com/leoarkiteto/stratum-backend/internal/model"
	"github.com/leoarkiteto/stratum-backend/internal/service"
)

// Deps are the handler's dependencies.
type Deps struct {
	Service *service.Service
	JWT     *auth.JWTManager
}

// Handler exposes the auth HTTP endpoints.
type Handler struct {
	svc *service.Service
	jwt *auth.JWTManager
}

// New builds the auth handler.
func New(deps Deps) *Handler {
	return &Handler{svc: deps.Service, jwt: deps.JWT}
}

// maxBodyBytes caps request bodies (defense against oversized payloads).
const maxBodyBytes = 1 << 20 // 1 MiB

type registerRequest struct {
	Name     string     `json:"name"`
	Email    string     `json:"email"`
	Password string     `json:"password"`
	Role     model.Role `json:"role"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type authResponse struct {
	Token string     `json:"token"`
	User  model.User `json:"user"`
}

// Register handles POST /api/v1/auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := decodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	u, err := h.svc.Register(r.Context(), service.RegisterParams{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Role:     req.Role,
	})
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	token, err := h.jwt.Issue(u)
	if err != nil {
		httpx.WriteError(w, apierr.Internal("token_failed", "could not issue token", err))
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, authResponse{Token: token, User: *u})
}

// Login handles POST /api/v1/auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, err)
		return
	}
	u, err := h.svc.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	token, err := h.jwt.Issue(u)
	if err != nil {
		httpx.WriteError(w, apierr.Internal("token_failed", "could not issue token", err))
		return
	}
	httpx.WriteJSON(w, http.StatusOK, authResponse{Token: token, User: *u})
}

// Me handles GET /api/v1/auth/me. Requires an authenticated principal
// (enforced by the Authenticate middleware in the router).
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	p, ok := auth.PrincipalFrom(r.Context())
	if !ok {
		httpx.WriteError(w, apierr.Unauthorized("unauthorized", "authentication required"))
		return
	}
	u, err := h.svc.GetUserByID(r.Context(), p.UserID)
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, u)
}

// decodeJSON decodes the request body into v, enforcing the body size cap.
func decodeJSON(w http.ResponseWriter, r *http.Request, v any) *apierr.Error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		return apierr.Invalid("invalid_json", "malformed request body")
	}
	return nil
}
