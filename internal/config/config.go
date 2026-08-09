// Package config loads the backend configuration from environment
// variables (12-factor). Required variables fail fast at startup.
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration.
type Config struct {
	HTTPAddr      string
	DatabaseURL   string
	JWTSecret     string
	JWTIssuer     string
	MigrationsDir string
	JWTTTL        time.Duration
}

// Load reads configuration from the environment.
// DATABASE_URL and JWT_SECRET are required; the rest have defaults.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:      getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		JWTSecret:     os.Getenv("JWT_SECRET"),
		JWTTTL:        getDuration("JWT_TTL", 24*time.Hour),
		JWTIssuer:     getenv("JWT_ISSUER", "stratum-backend"),
		MigrationsDir: getenv("MIGRATIONS_DIR", "migrations"),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.JWTSecret == "" {
		missing = append(missing, "JWT_SECRET")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf(
			"missing required environment variables: %s",
			strings.Join(missing, ", "),
		)
	}
	if len(cfg.JWTSecret) < 32 {
		return Config{}, fmt.Errorf(
			"JWT_SECRET must be at least 32 characters (got %d)",
			len(cfg.JWTSecret),
		)
	}
	// Defense-in-depth: reject the placeholder shipped in .env.example so
	// it cannot accidentally end up in a real deployment.
	if cfg.JWTSecret == "change-me-to-a-long-random-string" {
		return Config{}, fmt.Errorf(
			"JWT_SECRET is still the .env.example placeholder — generate a real secret with: openssl rand -hex 32",
		)
	}
	return cfg, nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}
