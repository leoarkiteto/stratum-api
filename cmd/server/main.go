// Command server is the Stratum application entrypoint: load config, open DB,
// apply migrations, assemble the GOTTH app and serve.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leoarkiteto/stratum/internal/app"
	"github.com/leoarkiteto/stratum/internal/config"
	"github.com/leoarkiteto/stratum/internal/db"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx := context.Background()

	pool, err := db.Open(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := db.Migrate(ctx, pool, cfg.MigrationsDir); err != nil {
		return err
	}

	// Assemble the application: HTTP handler + background jobs (composition
	// root). The background loop promotes election winners once their handover
	// period expires; it is stopped (via the deferred cancel) on shutdown.
	application := app.New(cfg, pool)
	settlerCtx, stopBackground := context.WithCancel(ctx)
	defer stopBackground()
	go application.Run(settlerCtx)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           application.Handler(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() { errCh <- httpSrv.ListenAndServe() }()
	slog.Info("server listening", "addr", cfg.HTTPAddr)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-sig:
		slog.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpSrv.Shutdown(shutdownCtx)
	}
}
