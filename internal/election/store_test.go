package election_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum/internal/auth"
	"github.com/leoarkiteto/stratum/internal/db"
	"github.com/leoarkiteto/stratum/internal/election"
	"github.com/leoarkiteto/stratum/internal/model"
)

// TestStoreIntegration exercises the real Postgres election store.
// It requires TEST_DATABASE_URL pointing at a disposable test database,
// e.g. postgres://postgres:postgres@localhost:5432/stratum_test?sslmode=disable
func TestStoreIntegration(t *testing.T) {
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

	if err := db.Migrate(ctx, pool, "../../migrations"); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if _, err := pool.ExecContext(ctx, "TRUNCATE users RESTART IDENTITY CASCADE"); err != nil {
		t.Fatalf("truncate users: %v", err)
	}

	users := auth.NewStore(pool)
	store := election.NewStore(pool)

	syndic := &model.User{Email: "syndic@example.com", Name: "Sonia", Role: model.RoleSyndic, PasswordHash: "h"}
	owner1 := &model.User{Email: "ana@example.com", Name: "Ana", Role: model.RoleOwner, PasswordHash: "h"}
	owner2 := &model.User{Email: "bob@example.com", Name: "Bob", Role: model.RoleOwner, PasswordHash: "h"}
	for _, u := range []*model.User{syndic, owner1, owner2} {
		if _, err := users.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser(%s): %v", u.Email, err)
		}
	}

	t.Run("create election and list", func(t *testing.T) {
		e := &model.Election{Title: "2026 Election", Description: "desc", Status: model.ElectionStatusNomination, CreatedBy: syndic.ID}
		if err := store.CreateElection(ctx, e); err != nil {
			t.Fatalf("CreateElection: %v", err)
		}
		if e.ID == 0 || e.CreatedAt.IsZero() {
			t.Fatal("expected generated id and timestamp")
		}
		got, err := store.GetElection(ctx, e.ID)
		if err != nil {
			t.Fatalf("GetElection: %v", err)
		}
		if got.Title != e.Title || got.Status != model.ElectionStatusNomination || got.CreatorName != "Sonia" {
			t.Errorf("got %+v", got)
		}
		all, err := store.ListElections(ctx)
		if err != nil || len(all) != 1 {
			t.Fatalf("ListElections = %d rows, err %v; want 1", len(all), err)
		}
		if all[0].CandidateCount != 0 {
			t.Errorf("CandidateCount = %d, want 0", all[0].CandidateCount)
		}
	})

	t.Run("candidates, votes, tally and close", func(t *testing.T) {
		e := &model.Election{Title: "Runoff", Status: model.ElectionStatusNomination, CreatedBy: syndic.ID}
		if err := store.CreateElection(ctx, e); err != nil {
			t.Fatalf("CreateElection: %v", err)
		}

		c1 := &model.Candidate{ElectionID: e.ID, UserID: owner1.ID, Statement: "Platform A"}
		c2 := &model.Candidate{ElectionID: e.ID, UserID: owner2.ID, Statement: "Platform B"}
		if err := store.CreateCandidate(ctx, c1); err != nil {
			t.Fatalf("CreateCandidate 1: %v", err)
		}
		if err := store.CreateCandidate(ctx, c2); err != nil {
			t.Fatalf("CreateCandidate 2: %v", err)
		}
		if err := store.CreateCandidate(ctx, &model.Candidate{ElectionID: e.ID, UserID: owner2.ID}); !errors.Is(err, election.ErrAlreadyCandidate) {
			t.Fatalf("duplicate candidate: want ErrAlreadyCandidate, got %v", err)
		}

		cands, err := store.ListCandidates(ctx, e.ID)
		if err != nil || len(cands) != 2 {
			t.Fatalf("ListCandidates = %d, err %v; want 2", len(cands), err)
		}
		if cands[0].Name != "Ana" {
			t.Errorf("candidate name = %q, want joined user name", cands[0].Name)
		}

		if err := store.SetStatus(ctx, e.ID, model.ElectionStatusVoting, nil); err != nil {
			t.Fatalf("SetStatus: %v", err)
		}

		// owner1 votes for c2; owner2 votes for c1 → tie.
		if err := store.CreateVote(ctx, e.ID, owner1.ID, c2.ID); err != nil {
			t.Fatalf("CreateVote 1: %v", err)
		}
		if err := store.CreateVote(ctx, e.ID, owner2.ID, c1.ID); err != nil {
			t.Fatalf("CreateVote 2: %v", err)
		}
		if err := store.CreateVote(ctx, e.ID, owner1.ID, c1.ID); !errors.Is(err, election.ErrAlreadyVoted) {
			t.Fatalf("double vote: want ErrAlreadyVoted, got %v", err)
		}

		tallies, err := store.Tally(ctx, e.ID)
		if err != nil || len(tallies) != 2 {
			t.Fatalf("Tally = %d, err %v; want 2", len(tallies), err)
		}
		if tallies[0].Votes != 1 || tallies[1].Votes != 1 {
			t.Errorf("tallies = %+v, want 1 vote each", tallies)
		}

		// Tie → close outright without a winner, roles untouched.
		if err := store.CloseElection(ctx, e.ID, 0, time.Now(), nil); err != nil {
			t.Fatalf("CloseElection: %v", err)
		}
		got, _ := store.GetElection(ctx, e.ID)
		if got.Status != model.ElectionStatusClosed || got.WinnerID != nil || got.ClosedAt == nil {
			t.Errorf("closed election = %+v, want closed, no winner", got)
		}
		if got.TransitionEndsAt != nil {
			t.Errorf("TransitionEndsAt = %v, want nil (no handover)", *got.TransitionEndsAt)
		}
	})

	t.Run("handover promotes winner once it expires", func(t *testing.T) {
		e := &model.Election{Title: "Decisive", Status: model.ElectionStatusNomination, CreatedBy: syndic.ID}
		if err := store.CreateElection(ctx, e); err != nil {
			t.Fatalf("CreateElection: %v", err)
		}
		c1 := &model.Candidate{ElectionID: e.ID, UserID: owner1.ID}
		if err := store.CreateCandidate(ctx, c1); err != nil {
			t.Fatalf("CreateCandidate: %v", err)
		}
		if err := store.SetStatus(ctx, e.ID, model.ElectionStatusVoting, nil); err != nil {
			t.Fatalf("SetStatus: %v", err)
		}
		if err := store.CreateVote(ctx, e.ID, owner2.ID, c1.ID); err != nil {
			t.Fatalf("CreateVote: %v", err)
		}

		// Close with a winner → transition phase, no role change yet.
		past := time.Now().Add(-time.Hour)
		if err := store.CloseElection(ctx, e.ID, owner1.ID, time.Now(), &past); err != nil {
			t.Fatalf("CloseElection: %v", err)
		}
		got, _ := store.GetElection(ctx, e.ID)
		if got.Status != model.ElectionStatusTransition || got.WinnerID == nil || *got.WinnerID != owner1.ID {
			t.Fatalf("election = %+v, want transition with winner %d", got, owner1.ID)
		}
		if got.TransitionEndsAt == nil || !got.TransitionEndsAt.Before(time.Now()) {
			t.Errorf("TransitionEndsAt = %v, want a past deadline", got.TransitionEndsAt)
		}
		if u, _ := users.GetUserByID(ctx, owner1.ID); u.Role != model.RoleOwner {
			t.Errorf("winner promoted during handover: role = %q", u.Role)
		}
		if u, _ := users.GetUserByID(ctx, syndic.ID); u.Role != model.RoleSyndic {
			t.Errorf("previous syndic demoted during handover: role = %q", u.Role)
		}

		// Settling the expired handover promotes the winner and demotes the old syndic.
		if err := store.SettleTransitions(ctx, time.Now()); err != nil {
			t.Fatalf("SettleTransitions: %v", err)
		}
		got, _ = store.GetElection(ctx, e.ID)
		if got.Status != model.ElectionStatusClosed {
			t.Errorf("Status = %q, want closed after handover", got.Status)
		}
		if u, _ := users.GetUserByID(ctx, owner1.ID); u.Role != model.RoleSyndic {
			t.Errorf("winner role = %q, want syndic", u.Role)
		}
		if u, _ := users.GetUserByID(ctx, syndic.ID); u.Role != model.RoleOwner {
			t.Errorf("previous syndic role = %q, want owner", u.Role)
		}
	})

	t.Run("future handover is not settled", func(t *testing.T) {
		e := &model.Election{Title: "Not Yet", Status: model.ElectionStatusNomination, CreatedBy: syndic.ID}
		if err := store.CreateElection(ctx, e); err != nil {
			t.Fatalf("CreateElection: %v", err)
		}
		c1 := &model.Candidate{ElectionID: e.ID, UserID: owner1.ID}
		if err := store.CreateCandidate(ctx, c1); err != nil {
			t.Fatalf("CreateCandidate: %v", err)
		}
		if err := store.SetStatus(ctx, e.ID, model.ElectionStatusVoting, nil); err != nil {
			t.Fatalf("SetStatus: %v", err)
		}
		future := time.Now().Add(24 * time.Hour)
		if err := store.CloseElection(ctx, e.ID, owner1.ID, time.Now(), &future); err != nil {
			t.Fatalf("CloseElection: %v", err)
		}
		if err := store.SettleTransitions(ctx, time.Now()); err != nil {
			t.Fatalf("SettleTransitions: %v", err)
		}
		got, _ := store.GetElection(ctx, e.ID)
		if got.Status != model.ElectionStatusTransition {
			t.Errorf("Status = %q, want transition (deadline in the future)", got.Status)
		}
	})
}
