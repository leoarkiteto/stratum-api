package store_test

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/leoarkiteto/stratum-api/internal/db"
	"github.com/leoarkiteto/stratum-api/internal/model"
	"github.com/leoarkiteto/stratum-api/internal/store"
)

// TestUsersIntegration exercises the real Postgres store.
// It requires a TEST_DATABASE_URL pointing at a disposable test database,
// e.g. postgres://postgres:postgres@localhost:5432/stratum_test?sslmode=disable
func TestUsersIntegration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration test")
	}

	ctx := context.Background()
	pool, err := db.Open(dsn)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { pool.Close() })

	// Migrations live in backend/migrations; go test runs with the package
	// dir (internal/store) as CWD.
	if err := db.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	// Start from a clean slate (dedicated test DB only).
	if _, err := pool.ExecContext(ctx, "TRUNCATE users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate users: %v", err)
	}

	s := store.New(pool)

	t.Run("create and fetch by email", func(t *testing.T) {
		u := &model.User{
			Email:        "ana@example.com",
			Name:         "Ana Souza",
			Role:         model.RoleOwner,
			PasswordHash: "somedummyhash",
		}
		id, err := s.CreateUser(ctx, u)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		if id == 0 || u.CreatedAt.IsZero() {
			t.Fatal("expected generated id and timestamps")
		}

		got, err := s.GetUserByEmail(ctx, "ana@example.com")
		if err != nil {
			t.Fatalf("GetUserByEmail: %v", err)
		}
		if got.ID != id || got.Name != "Ana Souza" || got.Role != model.RoleOwner {
			t.Errorf("got %+v, want the created user", got)
		}
		if got.PasswordHash != "somedummyhash" {
			t.Error("password hash must round-trip")
		}
	})

	t.Run("fetch by id", func(t *testing.T) {
		u := &model.User{Email: "bob@example.com", Name: "Bob", Role: model.RoleTenant, PasswordHash: "h"}
		id, err := s.CreateUser(ctx, u)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
		got, err := s.GetUserByID(ctx, id)
		if err != nil {
			t.Fatalf("GetUserByID: %v", err)
		}
		if got.Email != "bob@example.com" || got.Role != model.RoleTenant {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("duplicate email", func(t *testing.T) {
		u := &model.User{Email: "ana@example.com", Name: "Ana Again", Role: model.RoleOwner, PasswordHash: "h"}
		if _, err := s.CreateUser(ctx, u); !errors.Is(err, store.ErrEmailTaken) {
			t.Fatalf("want ErrEmailTaken, got %v", err)
		}
	})

	t.Run("not found", func(t *testing.T) {
		if _, err := s.GetUserByEmail(ctx, "ghost@example.com"); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
		if _, err := s.GetUserByID(ctx, 999999); !errors.Is(err, store.ErrNotFound) {
			t.Fatalf("want ErrNotFound, got %v", err)
		}
	})
}
