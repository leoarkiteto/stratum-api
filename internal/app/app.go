// Package app assembles the HTTP server: modules, shared middleware, static assets.
package app

import (
	"database/sql"
	"net/http"

	"github.com/leoarkiteto/stratum/internal/auth"
	"github.com/leoarkiteto/stratum/internal/config"
	"github.com/leoarkiteto/stratum/internal/home"
	"github.com/leoarkiteto/stratum/internal/password"
	"github.com/leoarkiteto/stratum/internal/session"
)

// New builds the application's root handler with all modules wired.
func New(cfg config.Config, db *sql.DB) http.Handler {
	sessions := session.New(db, cfg.SessionTTL)
	store := auth.NewStore(db)
	hasher := password.New(cfg.PasswordPepper, password.DefaultParams)

	authModule := auth.New(auth.Deps{
		Users:      store,
		Sessions:   sessions,
		Hasher:     hasher,
		Secure:     cfg.CookieSecure,
		SessionTTL: cfg.SessionTTL,
	})

	homeModule := home.New(home.Deps{
		Sessions: sessions,
		Users:    store,
		Secure:   cfg.CookieSecure,
	})

	mux := http.NewServeMux()
	authModule.RegisterRoutes(mux)
	homeModule.RegisterRoutes(mux)

	// Static assets: vendored HTMX and compiled TailwindCSS.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return withMiddleware(mux)
}
