package election

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/leoarkiteto/stratum/internal/model"
)

// fakeStore is an in-memory ElectionStore for tests (no DB needed).
type fakeStore struct {
	users     map[int64]*model.User
	elections map[int64]*model.Election
	// candidates keyed by election id → user id.
	candidates map[int64]map[int64]*model.Candidate
	// votes keyed by election id → voter id → candidate id.
	votes           map[int64]map[int64]int64
	nextElectionID  int64
	nextCandidateID int64
}

func newFakeStore(users ...*model.User) *fakeStore {
	f := &fakeStore{
		users:      map[int64]*model.User{},
		elections:  map[int64]*model.Election{},
		candidates: map[int64]map[int64]*model.Candidate{},
		votes:      map[int64]map[int64]int64{},
	}
	for _, u := range users {
		f.users[u.ID] = u
	}
	return f
}

func (f *fakeStore) CreateElection(_ context.Context, e *model.Election) error {
	f.nextElectionID++
	e.ID = f.nextElectionID
	e.CreatedAt = time.Now()
	e.CreatorName = f.users[e.CreatedBy].Name
	f.elections[e.ID] = e
	return nil
}

func (f *fakeStore) GetElection(_ context.Context, id int64) (*model.Election, error) {
	e, ok := f.elections[id]
	if !ok {
		return nil, ErrNotFound
	}
	return e, nil
}

func (f *fakeStore) ListElections(_ context.Context) ([]*model.Election, error) {
	var out []*model.Election
	for _, e := range f.elections {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (f *fakeStore) SetStatus(_ context.Context, id int64, status model.ElectionStatus, closedAt *time.Time) error {
	e, ok := f.elections[id]
	if !ok {
		return ErrNotFound
	}
	e.Status = status
	e.ClosedAt = closedAt
	return nil
}

func (f *fakeStore) CreateCandidate(_ context.Context, c *model.Candidate) error {
	if _, ok := f.elections[c.ElectionID]; !ok {
		return ErrNotFound
	}
	if _, dup := f.candidates[c.ElectionID][c.UserID]; dup {
		return ErrAlreadyCandidate
	}
	f.nextCandidateID++
	c.ID = f.nextCandidateID
	c.CreatedAt = time.Now()
	c.Name = f.users[c.UserID].Name
	if f.candidates[c.ElectionID] == nil {
		f.candidates[c.ElectionID] = map[int64]*model.Candidate{}
	}
	f.candidates[c.ElectionID][c.UserID] = c
	return nil
}

func (f *fakeStore) ListCandidates(_ context.Context, electionID int64) ([]model.Candidate, error) {
	var out []model.Candidate
	for _, c := range f.candidates[electionID] {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (f *fakeStore) DeleteCandidate(_ context.Context, electionID, userID int64) error {
	if _, ok := f.candidates[electionID][userID]; !ok {
		return ErrNotCandidate
	}
	delete(f.candidates[electionID], userID)
	return nil
}

func (f *fakeStore) HasVoted(_ context.Context, electionID, voterID int64) (bool, error) {
	_, ok := f.votes[electionID][voterID]
	return ok, nil
}

func (f *fakeStore) CreateVote(_ context.Context, electionID, voterID, candidateID int64) error {
	if _, dup := f.votes[electionID][voterID]; dup {
		return ErrAlreadyVoted
	}
	if f.candidateByID(electionID, candidateID) == nil {
		return ErrInvalidCandidate
	}
	if f.votes[electionID] == nil {
		f.votes[electionID] = map[int64]int64{}
	}
	f.votes[electionID][voterID] = candidateID
	return nil
}

func (f *fakeStore) Tally(_ context.Context, electionID int64) ([]model.Tally, error) {
	counts := map[int64]int{}
	for _, candidateID := range f.votes[electionID] {
		counts[candidateID]++
	}
	var out []model.Tally
	for _, c := range f.candidates[electionID] {
		out = append(out, model.Tally{Candidate: *c, Votes: counts[c.ID]})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Votes != out[j].Votes {
			return out[i].Votes > out[j].Votes
		}
		return out[i].Candidate.ID < out[j].Candidate.ID
	})
	return out, nil
}

func (f *fakeStore) CloseElection(_ context.Context, electionID, winnerID int64, closedAt time.Time, transitionEndsAt *time.Time) error {
	e, ok := f.elections[electionID]
	if !ok {
		return ErrNotFound
	}
	if e.Status != model.ElectionStatusVoting {
		return ErrWrongPhase
	}
	e.ClosedAt = &closedAt
	if transitionEndsAt != nil {
		e.Status = model.ElectionStatusTransition
		e.TransitionEndsAt = transitionEndsAt
	}
	if winnerID != 0 {
		w := winnerID
		e.WinnerID = &w
		e.WinnerName = f.users[winnerID].Name
	}
	if transitionEndsAt == nil {
		e.Status = model.ElectionStatusClosed
	}
	return nil
}

func (f *fakeStore) SettleTransitions(_ context.Context, now time.Time) error {
	for _, e := range f.elections {
		if e.Status != model.ElectionStatusTransition || e.TransitionEndsAt == nil || !e.TransitionEndsAt.Before(now) {
			continue
		}
		if e.WinnerID != nil {
			for _, u := range f.users {
				if u.Role == model.RoleSyndic && u.ID != *e.WinnerID {
					u.Role = model.RoleOwner
				}
			}
			if u := f.users[*e.WinnerID]; u != nil {
				u.Role = model.RoleSyndic
			}
		}
		e.Status = model.ElectionStatusClosed
	}
	return nil
}

func (f *fakeStore) candidateByID(electionID, candidateID int64) *model.Candidate {
	for _, c := range f.candidates[electionID] {
		if c.ID == candidateID {
			return c
		}
	}
	return nil
}

// testUsers returns the three roles as distinct users.
func testUsers() (syndic, owner1, owner2, tenant *model.User) {
	syndic = &model.User{ID: 1, Name: "Sonia Sindica", Role: model.RoleSyndic}
	owner1 = &model.User{ID: 2, Name: "Ana Owner", Role: model.RoleOwner}
	owner2 = &model.User{ID: 3, Name: "Bruno Owner", Role: model.RoleOwner}
	tenant = &model.User{ID: 4, Name: "Teresa Tenant", Role: model.RoleTenant}
	return
}

func newTestService(users ...*model.User) (*Service, *fakeStore) {
	fs := newFakeStore(users...)
	return NewService(fs), fs
}

func seedElection(t *testing.T, svc *Service, actor *model.User, transitionDays ...int) *model.Election {
	t.Helper()
	days := defaultTransitionDays
	if len(transitionDays) > 0 {
		days = transitionDays[0]
	}
	e, err := svc.CreateElection(context.Background(), actor, "2026 Election", "Choose the next syndic", days)
	if err != nil {
		t.Fatalf("seed CreateElection: %v", err)
	}
	return e
}

func seedCandidate(t *testing.T, svc *Service, e *model.Election, actor *model.User) int64 {
	t.Helper()
	if err := svc.RegisterCandidate(context.Background(), actor, e.ID, "Vote for me"); err != nil {
		t.Fatalf("seed RegisterCandidate: %v", err)
	}
	cands, err := svc.store.ListCandidates(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("seed ListCandidates: %v", err)
	}
	for _, c := range cands {
		if c.UserID == actor.ID {
			return c.ID
		}
	}
	t.Fatal("candidate not found after registration")
	return 0
}

func TestCreateElectionSuccess(t *testing.T) {
	syndic, _, _, _ := testUsers()
	svc, _ := newTestService(syndic)

	// transitionDays 0 → default 7.
	e, err := svc.CreateElection(context.Background(), syndic, "  2026 Election  ", "Desc", 0)
	if err != nil {
		t.Fatalf("CreateElection: %v", err)
	}
	if e.ID == 0 {
		t.Error("election should have an id")
	}
	if e.Title != "2026 Election" {
		t.Errorf("Title = %q, want trimmed", e.Title)
	}
	if e.Status != model.ElectionStatusNomination {
		t.Errorf("Status = %q, want nomination", e.Status)
	}
	if e.TransitionDays != defaultTransitionDays {
		t.Errorf("TransitionDays = %d, want default %d", e.TransitionDays, defaultTransitionDays)
	}
}

func TestCreateElectionOnlySyndic(t *testing.T) {
	_, owner1, _, tenant := testUsers()
	for _, actor := range []*model.User{owner1, tenant} {
		svc, _ := newTestService(actor)
		if _, err := svc.CreateElection(context.Background(), actor, "Election", "", 7); !errors.Is(err, ErrNotSyndic) {
			t.Errorf("role %s: want ErrNotSyndic, got %v", actor.Role, err)
		}
	}
}

func TestCreateElectionValidation(t *testing.T) {
	syndic, _, _, _ := testUsers()
	svc, _ := newTestService(syndic)

	if _, err := svc.CreateElection(context.Background(), syndic, "   ", "", 7); !isValidationError(err) {
		t.Errorf("empty title: want ValidationError, got %v", err)
	}
	if _, err := svc.CreateElection(context.Background(), syndic, "Election", "", 0); err != nil {
		t.Errorf("zero days should default, got %v", err)
	}
	if _, err := svc.CreateElection(context.Background(), syndic, "Election", "", 91); !isValidationError(err) {
		t.Errorf("91 days: want ValidationError, got %v", err)
	}
}

func TestRegisterCandidateLifecycle(t *testing.T) {
	syndic, owner1, _, tenant := testUsers()
	svc, _ := newTestService(syndic, owner1, tenant)
	e := seedElection(t, svc, syndic)

	// Owner registers during nomination.
	if err := svc.RegisterCandidate(context.Background(), owner1, e.ID, "  My platform  "); err != nil {
		t.Fatalf("RegisterCandidate: %v", err)
	}
	cands, _ := svc.store.ListCandidates(context.Background(), e.ID)
	if len(cands) != 1 || cands[0].Name != owner1.Name || cands[0].Statement != "My platform" {
		t.Fatalf("candidates = %+v, want one with trimmed statement", cands)
	}

	// Tenant cannot run.
	if err := svc.RegisterCandidate(context.Background(), tenant, e.ID, ""); !errors.Is(err, ErrNotEligible) {
		t.Errorf("tenant: want ErrNotEligible, got %v", err)
	}

	// Duplicate registration.
	if err := svc.RegisterCandidate(context.Background(), owner1, e.ID, ""); !errors.Is(err, ErrAlreadyCandidate) {
		t.Errorf("duplicate: want ErrAlreadyCandidate, got %v", err)
	}

	// Withdraw then re-register.
	if err := svc.WithdrawCandidate(context.Background(), owner1, e.ID); err != nil {
		t.Fatalf("WithdrawCandidate: %v", err)
	}
	if err := svc.WithdrawCandidate(context.Background(), owner1, e.ID); !errors.Is(err, ErrNotCandidate) {
		t.Errorf("withdraw twice: want ErrNotCandidate, got %v", err)
	}
}

func TestRegisterCandidateWrongPhase(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	svc, _ := newTestService(syndic, owner1, owner2)
	e := seedElection(t, svc, syndic)
	seedCandidate(t, svc, e, owner2)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	if err := svc.RegisterCandidate(context.Background(), owner1, e.ID, ""); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("want ErrWrongPhase, got %v", err)
	}
}

func TestOpenVoting(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	svc, _ := newTestService(syndic, owner1)
	e := seedElection(t, svc, syndic)

	// No candidates yet → rejected.
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); !isValidationError(err) {
		t.Fatalf("no candidates: want ValidationError, got %v", err)
	}

	seedCandidate(t, svc, e, owner1)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	if got, _ := svc.store.GetElection(context.Background(), e.ID); got.Status != model.ElectionStatusVoting {
		t.Errorf("Status = %q, want voting", got.Status)
	}

	// Only the syndic may open voting; only from nomination.
	if err := svc.OpenVoting(context.Background(), owner1, e.ID); !errors.Is(err, ErrNotSyndic) {
		t.Errorf("owner: want ErrNotSyndic, got %v", err)
	}
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("voting again: want ErrWrongPhase, got %v", err)
	}
}

func TestVote(t *testing.T) {
	syndic, owner1, owner2, tenant := testUsers()
	svc, _ := newTestService(syndic, owner1, owner2, tenant)
	e := seedElection(t, svc, syndic)
	c1 := seedCandidate(t, svc, e, owner1)
	c2 := seedCandidate(t, svc, e, owner2)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}

	// Owner votes.
	if err := svc.Vote(context.Background(), owner1, e.ID, c2); err != nil {
		t.Fatalf("Vote: %v", err)
	}

	// Double vote rejected.
	if err := svc.Vote(context.Background(), owner1, e.ID, c1); !errors.Is(err, ErrAlreadyVoted) {
		t.Errorf("double vote: want ErrAlreadyVoted, got %v", err)
	}

	// Tenant and syndic cannot vote (owners only).
	if err := svc.Vote(context.Background(), tenant, e.ID, c1); !errors.Is(err, ErrNotOwner) {
		t.Errorf("tenant: want ErrNotOwner, got %v", err)
	}
	if err := svc.Vote(context.Background(), syndic, e.ID, c1); !errors.Is(err, ErrNotOwner) {
		t.Errorf("syndic: want ErrNotOwner, got %v", err)
	}

	// Unknown candidate rejected.
	if err := svc.Vote(context.Background(), owner2, e.ID, 9999); !errors.Is(err, ErrInvalidCandidate) {
		t.Errorf("unknown candidate: want ErrInvalidCandidate, got %v", err)
	}
}

func TestVoteWrongPhase(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	svc, _ := newTestService(syndic, owner1)
	e := seedElection(t, svc, syndic)
	c1 := seedCandidate(t, svc, e, owner1)

	if err := svc.Vote(context.Background(), owner1, e.ID, c1); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("nomination phase: want ErrWrongPhase, got %v", err)
	}
}

func TestCloseElectionStartsHandoverAndSettles(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	svc, fs := newTestService(syndic, owner1, owner2)
	e := seedElection(t, svc, syndic, 7)
	c1 := seedCandidate(t, svc, e, owner1)
	seedCandidate(t, svc, e, owner2)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	// owner1 votes for themselves; a third owner (owner2) votes for owner1.
	if err := svc.Vote(context.Background(), owner2, e.ID, c1); err != nil {
		t.Fatalf("Vote: %v", err)
	}

	// Closing starts the handover: winner recorded, nobody promoted yet.
	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}
	got, _ := svc.store.GetElection(context.Background(), e.ID)
	if got.Status != model.ElectionStatusTransition {
		t.Fatalf("Status = %q, want transition", got.Status)
	}
	if got.WinnerID == nil || *got.WinnerID != owner1.ID {
		t.Fatalf("WinnerID = %v, want %d", got.WinnerID, owner1.ID)
	}
	if got.TransitionEndsAt == nil {
		t.Fatal("TransitionEndsAt should be set during the handover")
	}
	if fs.users[owner1.ID].Role != model.RoleOwner {
		t.Errorf("winner promoted too early: role = %q, want owner until handover ends", fs.users[owner1.ID].Role)
	}
	if fs.users[syndic.ID].Role != model.RoleSyndic {
		t.Errorf("previous syndic demoted too early: role = %q, want syndic", fs.users[syndic.ID].Role)
	}

	// The handover ends (past deadline) → settle promotes the winner.
	past := time.Now().Add(-time.Hour)
	fs.elections[e.ID].TransitionEndsAt = &past
	if err := svc.SettleTransitions(context.Background()); err != nil {
		t.Fatalf("SettleTransitions: %v", err)
	}
	got, _ = svc.store.GetElection(context.Background(), e.ID)
	if got.Status != model.ElectionStatusClosed {
		t.Fatalf("Status = %q, want closed after handover", got.Status)
	}
	if fs.users[owner1.ID].Role != model.RoleSyndic {
		t.Errorf("winner role = %q, want syndic", fs.users[owner1.ID].Role)
	}
	if fs.users[syndic.ID].Role != model.RoleOwner {
		t.Errorf("previous syndic role = %q, want owner", fs.users[syndic.ID].Role)
	}
	if fs.users[owner2.ID].Role != model.RoleOwner {
		t.Errorf("loser role = %q, want owner", fs.users[owner2.ID].Role)
	}
}

func TestCloseElectionIncumbentReelected(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	svc, fs := newTestService(syndic, owner1)
	e := seedElection(t, svc, syndic)
	c1 := seedCandidate(t, svc, e, syndic)
	seedCandidate(t, svc, e, owner1)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	if err := svc.Vote(context.Background(), owner1, e.ID, c1); err != nil {
		t.Fatalf("Vote: %v", err)
	}

	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}
	if fs.users[syndic.ID].Role != model.RoleSyndic {
		t.Fatalf("incumbent demoted during handover: %q", fs.users[syndic.ID].Role)
	}
	past := time.Now().Add(-time.Hour)
	fs.elections[e.ID].TransitionEndsAt = &past
	if err := svc.SettleTransitions(context.Background()); err != nil {
		t.Fatalf("SettleTransitions: %v", err)
	}
	if fs.users[syndic.ID].Role != model.RoleSyndic {
		t.Errorf("re-elected syndic role = %q, want syndic", fs.users[syndic.ID].Role)
	}
}

func TestCloseElectionTieHasNoWinner(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	owner3 := &model.User{ID: 5, Name: "Carla Owner", Role: model.RoleOwner}
	owner4 := &model.User{ID: 6, Name: "Diego Owner", Role: model.RoleOwner}
	svc, fs := newTestService(syndic, owner1, owner2, owner3, owner4)
	e := seedElection(t, svc, syndic)
	c1 := seedCandidate(t, svc, e, owner1)
	c2 := seedCandidate(t, svc, e, owner2)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}

	// Real tie: one vote each → no handover, election closes outright.
	if err := svc.Vote(context.Background(), owner3, e.ID, c1); err != nil {
		t.Fatalf("Vote owner3: %v", err)
	}
	if err := svc.Vote(context.Background(), owner4, e.ID, c2); err != nil {
		t.Fatalf("Vote owner4: %v", err)
	}

	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}
	got, _ := svc.store.GetElection(context.Background(), e.ID)
	if got.Status != model.ElectionStatusClosed {
		t.Fatalf("Status = %q, want closed", got.Status)
	}
	if got.WinnerID != nil {
		t.Errorf("WinnerID = %v, want nil (tie)", *got.WinnerID)
	}
	if got.TransitionEndsAt != nil {
		t.Errorf("TransitionEndsAt = %v, want nil (no handover)", *got.TransitionEndsAt)
	}
	for _, u := range []*model.User{owner1, owner2} {
		if fs.users[u.ID].Role != model.RoleOwner {
			t.Errorf("tied candidate %s role = %q, want owner", u.Name, fs.users[u.ID].Role)
		}
	}
	if fs.users[syndic.ID].Role != model.RoleSyndic {
		t.Errorf("syndic role changed without a winner: %q", fs.users[syndic.ID].Role)
	}
}

func TestCloseElectionNoVotesHasNoWinner(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	svc, fs := newTestService(syndic, owner1, owner2)
	e := seedElection(t, svc, syndic)
	seedCandidate(t, svc, e, owner1)
	seedCandidate(t, svc, e, owner2)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}

	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}
	got, _ := svc.store.GetElection(context.Background(), e.ID)
	if got.WinnerID != nil {
		t.Errorf("WinnerID = %v, want nil (no votes)", *got.WinnerID)
	}
	if got.TransitionEndsAt != nil {
		t.Errorf("TransitionEndsAt = %v, want nil (no handover)", *got.TransitionEndsAt)
	}
	if fs.users[syndic.ID].Role != model.RoleSyndic {
		t.Errorf("syndic role changed without a winner: %q", fs.users[syndic.ID].Role)
	}
}

func TestSettleTransitionsSkipsFuture(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	svc, fs := newTestService(syndic, owner1)
	e := seedElection(t, svc, syndic, 30)
	c1 := seedCandidate(t, svc, e, owner1)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	if err := svc.Vote(context.Background(), owner1, e.ID, c1); err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}

	// The handover deadline is still in the future (30 days): no promotion.
	if err := svc.SettleTransitions(context.Background()); err != nil {
		t.Fatalf("SettleTransitions: %v", err)
	}
	got, _ := svc.store.GetElection(context.Background(), e.ID)
	if got.Status != model.ElectionStatusTransition {
		t.Fatalf("Status = %q, want transition (not yet expired)", got.Status)
	}
	if fs.users[owner1.ID].Role != model.RoleOwner {
		t.Errorf("winner promoted before the handover ended: %q", fs.users[owner1.ID].Role)
	}
}

func TestCloseElectionGuards(t *testing.T) {
	syndic, owner1, _, _ := testUsers()
	svc, _ := newTestService(syndic, owner1)
	e := seedElection(t, svc, syndic)

	// Only the syndic may close; only from the voting phase.
	if err := svc.CloseElection(context.Background(), owner1, e.ID); !errors.Is(err, ErrNotSyndic) {
		t.Errorf("owner: want ErrNotSyndic, got %v", err)
	}
	if err := svc.CloseElection(context.Background(), syndic, e.ID); !errors.Is(err, ErrWrongPhase) {
		t.Errorf("nomination: want ErrWrongPhase, got %v", err)
	}
}

func TestDetailFlags(t *testing.T) {
	syndic, owner1, owner2, tenant := testUsers()
	svc, _ := newTestService(syndic, owner1, owner2, tenant)
	e := seedElection(t, svc, syndic)

	// Nomination phase.
	v, err := svc.Detail(context.Background(), owner1, e.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if !v.CanRegister || v.IsCandidate || v.CanOpenVoting || v.CanVote || v.CanClose {
		t.Errorf("owner in nomination: got %+v, want only CanRegister", v)
	}
	v, _ = svc.Detail(context.Background(), syndic, e.ID)
	if !v.CanOpenVoting || !v.CanRegister {
		t.Errorf("syndic in nomination: got %+v, want CanOpenVoting + CanRegister (incumbent may run)", v)
	}

	// After registering, the owner is a candidate and can withdraw.
	if err := svc.RegisterCandidate(context.Background(), owner1, e.ID, ""); err != nil {
		t.Fatalf("RegisterCandidate: %v", err)
	}
	v, _ = svc.Detail(context.Background(), owner1, e.ID)
	if !v.IsCandidate || !v.CanWithdraw || v.CanRegister {
		t.Errorf("candidate: got %+v, want IsCandidate + CanWithdraw", v)
	}

	// Voting phase.
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	v, _ = svc.Detail(context.Background(), owner2, e.ID)
	if !v.CanVote {
		t.Errorf("owner in voting: got %+v, want CanVote", v)
	}
	v, _ = svc.Detail(context.Background(), tenant, e.ID)
	if v.CanVote || v.IsOwner {
		t.Errorf("tenant in voting: got %+v, want no voting rights", v)
	}
	v, _ = svc.Detail(context.Background(), syndic, e.ID)
	if !v.CanClose {
		t.Errorf("syndic in voting: got %+v, want CanClose", v)
	}

	// Closed phase shows tallies.
	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}
	v, _ = svc.Detail(context.Background(), owner2, e.ID)
	if v.Election.Status != model.ElectionStatusClosed || len(v.Tallies) != 1 {
		t.Errorf("closed detail: got status %q with %d tallies, want closed with 1", v.Election.Status, len(v.Tallies))
	}
}

func TestCloseElectionUsesInjectedClock(t *testing.T) {
	syndic, owner1, owner2, _ := testUsers()
	fs := newFakeStore(syndic, owner1, owner2)
	fixed := time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	svc := NewService(fs, WithClock(func() time.Time { return fixed }))

	e := seedElection(t, svc, syndic, 7)
	c1 := seedCandidate(t, svc, e, owner1)
	seedCandidate(t, svc, e, owner2)
	if err := svc.OpenVoting(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("OpenVoting: %v", err)
	}
	if err := svc.Vote(context.Background(), owner2, e.ID, c1); err != nil {
		t.Fatalf("Vote: %v", err)
	}
	if err := svc.CloseElection(context.Background(), syndic, e.ID); err != nil {
		t.Fatalf("CloseElection: %v", err)
	}

	got, err := fs.GetElection(context.Background(), e.ID)
	if err != nil {
		t.Fatalf("GetElection: %v", err)
	}
	if got.ClosedAt == nil || !got.ClosedAt.Equal(fixed) {
		t.Errorf("ClosedAt = %v, want the injected clock time %v", got.ClosedAt, fixed)
	}
	wantEnds := fixed.Add(7 * 24 * time.Hour)
	if got.TransitionEndsAt == nil || !got.TransitionEndsAt.Equal(wantEnds) {
		t.Errorf("TransitionEndsAt = %v, want %v", got.TransitionEndsAt, wantEnds)
	}
}

func isValidationError(err error) bool {
	var vErr *ValidationError
	return errors.As(err, &vErr)
}
