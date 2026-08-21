// Package app assembles the application: modules, shared middleware, static
// assets and background jobs. It is the composition root — the only place that
// constructs concrete adapters and injects them via Deps structs.
package app

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"github.com/leoarkiteto/stratum/internal/auth"
	"github.com/leoarkiteto/stratum/internal/config"
	"github.com/leoarkiteto/stratum/internal/election"
	"github.com/leoarkiteto/stratum/internal/home"
	"github.com/leoarkiteto/stratum/internal/password"
	"github.com/leoarkiteto/stratum/internal/session"
)

// settleInterval is how often the background loop promotes election winners
// whose handover period has ended.
const settleInterval = time.Minute

// App is the assembled application: the HTTP handler plus background jobs.
type App struct {
	handler http.Handler
	settler *election.Settler
}

// New builds the application with all modules wired via constructor injection.
func New(cfg config.Config, db *sql.DB) *App {
	sessions := session.New(db, cfg.SessionTTL)
	users := auth.NewStore(db)
	hasher := password.New(cfg.PasswordPepper, password.DefaultParams)

	authModule := auth.New(auth.Deps{
		Users:      users,
		Sessions:   sessions,
		Hasher:     hasher,
		Secure:     cfg.CookieSecure,
		SessionTTL: cfg.SessionTTL,
	})

	homeModule := home.New(home.Deps{
		Sessions: sessions,
		Users:    users,
		Secure:   cfg.CookieSecure,
	})

	electionStore := election.NewStore(db)
	electionModule := election.New(election.Deps{
		Store:    electionStore,
		Sessions: sessions,
		Users:    users,
		Secure:   cfg.CookieSecure,
	})

	mux := http.NewServeMux()
	authModule.RegisterRoutes(mux)
	homeModule.RegisterRoutes(mux)
	electionModule.RegisterRoutes(mux)

	// Static assets: vendored HTMX and compiled TailwindCSS.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	return &App{
		handler: withMiddleware(mux),
		settler: election.NewSettler(electionStore, settleInterval),
	}
}

// Handler returns the root HTTP handler.
func (a *App) Handler() http.Handler {
	return a.handler
}

// Run starts the app's background jobs and blocks until ctx is cancelled.
func (a *App) Run(ctx context.Context) {
	if a.settler != nil {
		a.settler.Run(ctx)
	}
}
