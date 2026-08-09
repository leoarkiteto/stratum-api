// Command server is the Stratum backend entrypoint: load config, open DB,
// apply migrations, wire store → service → handler → server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leoarkiteto/stratum-api/internal/auth"
	"github.com/leoarkiteto/stratum-api/internal/config"
	"github.com/leoarkiteto/stratum-api/internal/db"
	"github.com/leoarkiteto/stratum-api/internal/server"
	"github.com/leoarkiteto/stratum-api/internal/store"
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

	st := store.New(pool)
	jwtm := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTTTL, cfg.JWTIssuer)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.New(server.Deps{Store: st, JWT: jwtm}),
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
