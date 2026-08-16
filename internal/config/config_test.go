package config

import (
	"strings"
	"testing"
	"time"
)

const testPepper = "a-32-characters-or-more-test-pepper-0000"

func TestLoadRequiresVars(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PASSWORD_PEPPER", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load should fail when required vars are missing")
	} else if !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "PASSWORD_PEPPER") {
		t.Fatalf("error should list the missing vars, got: %v", err)
	}
}

func TestLoadRejectsShortPepper(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("PASSWORD_PEPPER", "too-short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("want a min-length error, got: %v", err)
	}
}

func TestLoadRejectsPlaceholderPepper(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("PASSWORD_PEPPER", "change-me-to-a-long-random-string")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("want a placeholder error, got: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("PASSWORD_PEPPER", testPepper)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v, want 24h", cfg.SessionTTL)
	}
	if cfg.CookieSecure {
		t.Error("CookieSecure should default to false")
	}
	if cfg.MigrationsDir != "migrations" {
		t.Errorf("MigrationsDir = %q, want migrations", cfg.MigrationsDir)
	}
	if cfg.PasswordPepper != testPepper {
		t.Errorf("PasswordPepper = %q, want %q", cfg.PasswordPepper, testPepper)
	}
}

func TestLoadHonorsOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("PASSWORD_PEPPER", testPepper)
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("SESSION_TTL", "2h")
	t.Setenv("COOKIE_SECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.SessionTTL != 2*time.Hour {
		t.Errorf("SessionTTL = %v, want 2h", cfg.SessionTTL)
	}
	if !cfg.CookieSecure {
		t.Error("CookieSecure should honor COOKIE_SECURE=true")
	}
}
