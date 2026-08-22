// Package config loads the application configuration from environment
// variables (12-factor). Required variables fail fast at startup.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration.
type Config struct {
	HTTPAddr       string
	DatabaseURL    string
	MigrationsDir  string
	PasswordPepper string
	SessionTTL     time.Duration
	CookieSecure   bool
}

// Load reads configuration from the environment.
// DATABASE_URL and PASSWORD_PEPPER are required; the rest have defaults.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:       getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:    os.Getenv("DATABASE_URL"),
		MigrationsDir:  getenv("MIGRATIONS_DIR", "migrations"),
		PasswordPepper: os.Getenv("PASSWORD_PEPPER"),
		SessionTTL:     getDuration("SESSION_TTL", 24*time.Hour),
		CookieSecure:   getBool("COOKIE_SECURE", false),
	}

	var missing []string
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.PasswordPepper == "" {
		missing = append(missing, "PASSWORD_PEPPER")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf(
			"missing required environment variables: %s",
			strings.Join(missing, ", "),
		)
	}
	if len(cfg.PasswordPepper) < 32 {
		return Config{}, fmt.Errorf(
			"PASSWORD_PEPPER must be at least 32 characters (got %d)",
			len(cfg.PasswordPepper),
		)
	}
	// Defense-in-depth: reject the placeholder shipped in .env.example so it
	// cannot accidentally end up in a real deployment.
	if cfg.PasswordPepper == "change-me-to-a-long-random-string" {
		return Config{}, fmt.Errorf(
			"PASSWORD_PEPPER is still the .env.example placeholder — generate a real secret with: openssl rand -hex 32",
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

func getBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}
