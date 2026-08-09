// Package httpx provides shared HTTP helpers: JSON writing and the
// standard error envelope used by every API response.
//
// Envelope shape: {"error": {"code": "...", "message": "..."}}
package httpx

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/leoarkiteto/stratum-api/internal/apierr"
)

// ErrorEnvelope is the JSON body of every error response.
type ErrorEnvelope struct {
	Error ErrorBody `json:"error"`
}

// ErrorBody is the error detail inside the envelope.
type ErrorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// WriteJSON writes v as JSON with the given status code.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", "error", err)
	}
}

// WriteError writes the standard error envelope for err.
// Unexpected (non-apierr) errors are logged and masked as 500.
func WriteError(w http.ResponseWriter, err error) {
	ae := apierr.As(err)
	if ae == nil {
		slog.Error("unexpected error", "error", err)
		ae = apierr.Internal("internal_error", "internal server error", err)
	}
	WriteJSON(w, ae.Status(), ErrorEnvelope{
		Error: ErrorBody{Code: ae.Code, Message: ae.Message},
	})
}
