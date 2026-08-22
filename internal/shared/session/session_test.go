package session

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum/internal/shared/database"
)

func TestNewToken(t *testing.T) {
	a, b := newToken(), newToken()
	if len(a) < 32 {
		t.Fatalf("token too short: %q", a)
	}
	if a == b {
		t.Fatal("tokens must be unique")
	}
}

func TestHashToken(t *testing.T) {
	h := hash("abc")
	if h == "abc" || len(h) != 64 {
		t.Fatalf("hash = %q", h)
	}
	if hash("abc") != h {
		t.Fatal("hash must be deterministic")
	}
}

// TestSessionIntegration exercises the real Postgres-backed session store.
// It requires TEST_DATABASE_URL, e.g.
// postgres://postgres:postgres@localhost:5432/stratum_test?sslmode=disable
func TestSessionIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}

	ctx := context.Background()
	pool, err := database.Open(dsn)
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	if err := database.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := pool.ExecContext(ctx, "TRUNCATE users, sessions RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate: %v", err)
	}

	m := New(pool, time.Hour)

	token, csrf, err := m.Create(ctx)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if token == "" || csrf == "" {
		t.Fatal("Create must return token and csrf")
	}

	s, err := m.Get(ctx, token)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.UserID != 0 || s.CSRF != csrf {
		t.Errorf("Get = %+v, want userID 0 and csrf %q", s, csrf)
	}

	newToken, err := m.BindUser(ctx, token, 42)
	if err != nil {
		t.Fatalf("BindUser: %v", err)
	}
	if newToken == token {
		t.Fatal("BindUser must rotate the token")
	}
	if _, err := m.Get(ctx, token); !errors.Is(err, ErrNotFound) {
		t.Fatalf("old token should be gone, got %v", err)
	}
	s, err = m.Get(ctx, newToken)
	if err != nil {
		t.Fatalf("Get after BindUser: %v", err)
	}
	if s.UserID != 42 {
		t.Errorf("UserID = %d, want 42", s.UserID)
	}

	if err := m.Delete(ctx, newToken); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := m.Get(ctx, newToken); !errors.Is(err, ErrNotFound) {
		t.Fatalf("session should be deleted, got %v", err)
	}
}
