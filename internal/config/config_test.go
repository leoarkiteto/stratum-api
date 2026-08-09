package config

import (
	"strings"
	"testing"
	"time"
)

func TestLoadRequiresVars(t *testing.T) {
	t.Setenv("DATABASE_URL", "")
	t.Setenv("JWT_SECRET", "")
	if _, err := Load(); err == nil {
		t.Fatal("Load should fail when required vars are missing")
	} else if !strings.Contains(err.Error(), "DATABASE_URL") || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("error should list the missing vars, got: %v", err)
	}
}

func TestLoadRejectsShortJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("JWT_SECRET", "too-short")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "at least 32") {
		t.Fatalf("want a min-length error, got: %v", err)
	}
}

func TestLoadRejectsPlaceholderSecret(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("JWT_SECRET", "change-me-to-a-long-random-string")
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("want a placeholder error, got: %v", err)
	}
}

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":8080" {
		t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
	}
	if cfg.JWTTTL != 24*time.Hour {
		t.Errorf("JWTTTL = %v, want 24h", cfg.JWTTTL)
	}
	if cfg.JWTIssuer != "stratum-api" {
		t.Errorf("JWTIssuer = %q, want stratum-api", cfg.JWTIssuer)
	}
	if cfg.MigrationsDir != "migrations" {
		t.Errorf("MigrationsDir = %q, want migrations", cfg.MigrationsDir)
	}
}

func TestLoadHonorsOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://localhost/x")
	t.Setenv("JWT_SECRET", strings.Repeat("s", 32))
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("JWT_TTL", "2h")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.HTTPAddr != ":9090" {
		t.Errorf("HTTPAddr = %q, want :9090", cfg.HTTPAddr)
	}
	if cfg.JWTTTL != 2*time.Hour {
		t.Errorf("JWTTTL = %v, want 2h", cfg.JWTTTL)
	}
}
