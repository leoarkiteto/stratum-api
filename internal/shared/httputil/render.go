package httputil

import (
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
)

// IsHTMX reports whether the request was issued by htmx.
func IsHTMX(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

// Render writes a templ component as HTML.
func Render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		// Headers may already be committed; log rather than write an error.
		slog.Error("template render failed", "error", err)
	}
}

// HXRedirect tells htmx to navigate the browser to url. Used instead of a
// 3xx redirect so htmx swaps/follows correctly after a form post.
func HXRedirect(w http.ResponseWriter, url string) {
	w.Header().Set("HX-Redirect", url)
	w.WriteHeader(http.StatusOK)
}
