package election

import (
	"context"
	"strings"
	"time"

	"github.com/leoarkiteto/stratum/internal/model"
)

// Input length bounds, guarding against absurd payloads.
const (
	maxTitleLength        = 200
	maxDescriptionLength  = 2000
	maxStatementLength    = 1000
	defaultTransitionDays = 7
	maxTransitionDays     = 90
)

// ElectionStore is the persistence contract the election service needs.
// *Store implements it; tests use a hand-written fake.
type ElectionStore interface {
	CreateElection(ctx context.Context, e *model.Election) error
	GetElection(ctx context.Context, id int64) (*model.Election, error)
	ListElections(ctx context.Context) ([]*model.Election, error)
	SetStatus(ctx context.Context, id int64, status model.ElectionStatus, closedAt *time.Time) error
	CreateCandidate(ctx context.Context, c *model.Candidate) error
	ListCandidates(ctx context.Context, electionID int64) ([]model.Candidate, error)
	DeleteCandidate(ctx context.Context, electionID, userID int64) error
	HasVoted(ctx context.Context, electionID, voterID int64) (bool, error)
	CreateVote(ctx context.Context, electionID, voterID, candidateID int64) error
	Tally(ctx context.Context, electionID int64) ([]model.Tally, error)
	// CloseElection ends the voting phase: with a winner it starts the handover
	// (transition) period ending at transitionEndsAt; without one it closes.
	CloseElection(ctx context.Context, electionID, winnerID int64, closedAt time.Time, transitionEndsAt *time.Time) error
	// SettleTransitions promotes the winners of elections whose handover period
	// has ended and marks those elections closed.
	SettleTransitions(ctx context.Context, now time.Time) error
}

// Service is the election business logic.
type Service struct {
	store ElectionStore
	clock Clock
}

// Clock is the time-source port the election service depends on. It exists so
// tests and future adapters can control time instead of reading the wall clock.
type Clock func() time.Time

// Option configures the election service.
type Option func(*Service)

// WithClock replaces the default wall-clock time source.
func WithClock(c Clock) Option {
	return func(s *Service) { s.clock = c }
}

// NewService builds the election service.
func NewService(store ElectionStore, opts ...Option) *Service {
	s := &Service{store: store, clock: Clock(time.Now)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// CreateElection opens a new election in the nomination phase. Only the
// current syndic may create elections. transitionDays sets the handover
// period (default 7 days when zero).
func (s *Service) CreateElection(ctx context.Context, actor *model.User, title, description string, transitionDays int) (*model.Election, error) {
	if actor.Role != model.RoleSyndic {
		return nil, ErrNotSyndic
	}
	title = strings.TrimSpace(title)
	description = strings.TrimSpace(description)
	if title == "" {
		return nil, &ValidationError{Message: "Title is required."}
	}
	if len([]rune(title)) > maxTitleLength {
		return nil, &ValidationError{Message: "Title must be at most 200 characters."}
	}
	if len([]rune(description)) > maxDescriptionLength {
		return nil, &ValidationError{Message: "Description must be at most 2000 characters."}
	}
	if transitionDays == 0 {
		transitionDays = defaultTransitionDays
	}
	if transitionDays < 1 || transitionDays > maxTransitionDays {
		return nil, &ValidationError{Message: "Transition period must be between 1 and 90 days."}
	}

	e := &model.Election{
		Title:          title,
		Description:    description,
		Status:         model.ElectionStatusNomination,
		CreatedBy:      actor.ID,
		TransitionDays: transitionDays,
	}
	if err := s.store.CreateElection(ctx, e); err != nil {
		return nil, err
	}
	return e, nil
}

// ListElections returns all elections, newest first.
func (s *Service) ListElections(ctx context.Context) ([]*model.Election, error) {
	return s.store.ListElections(ctx)
}

// Detail returns the election page data for actor (who may be nil on routes
// that do not require auth, though all current routes do).
func (s *Service) Detail(ctx context.Context, actor *model.User, id int64) (*model.ElectionView, error) {
	e, err := s.store.GetElection(ctx, id)
	if err != nil {
		return nil, err
	}
	candidates, err := s.store.ListCandidates(ctx, id)
	if err != nil {
		return nil, err
	}

	v := &model.ElectionView{Election: e, Candidates: candidates}
	if e.Status == model.ElectionStatusClosed || e.Status == model.ElectionStatusTransition {
		tallies, err := s.store.Tally(ctx, id)
		if err != nil {
			return nil, err
		}
		v.Tallies = tallies
	}
	if actor == nil {
		return v, nil
	}

	for _, c := range candidates {
		if c.UserID == actor.ID {
			v.IsCandidate = true
			break
		}
	}
	v.HasVoted, err = s.store.HasVoted(ctx, id, actor.ID)
	if err != nil {
		return nil, err
	}
	v.IsOwner = actor.Role == model.RoleOwner

	canRun := actor.Role == model.RoleOwner || actor.Role == model.RoleSyndic
	v.CanRegister = e.Status == model.ElectionStatusNomination && canRun && !v.IsCandidate
	v.CanWithdraw = e.Status == model.ElectionStatusNomination && v.IsCandidate
	v.CanVote = e.Status == model.ElectionStatusVoting && actor.Role == model.RoleOwner && !v.HasVoted
	v.CanOpenVoting = e.Status == model.ElectionStatusNomination && actor.Role == model.RoleSyndic
	v.CanClose = e.Status == model.ElectionStatusVoting && actor.Role == model.RoleSyndic
	return v, nil
}

// RegisterCandidate registers actor in the election. Only owners and the
// current syndic can run, and only while the election is in nomination.
func (s *Service) RegisterCandidate(ctx context.Context, actor *model.User, electionID int64, statement string) error {
	if actor.Role != model.RoleOwner && actor.Role != model.RoleSyndic {
		return ErrNotEligible
	}
	e, err := s.store.GetElection(ctx, electionID)
	if err != nil {
		return err
	}
	if e.Status != model.ElectionStatusNomination {
		return ErrWrongPhase
	}
	statement = strings.TrimSpace(statement)
	if len([]rune(statement)) > maxStatementLength {
		return &ValidationError{Message: "Statement must be at most 1000 characters."}
	}
	return s.store.CreateCandidate(ctx, &model.Candidate{
		ElectionID: electionID,
		UserID:     actor.ID,
		Statement:  statement,
	})
}

// WithdrawCandidate removes actor's candidacy while nominations are open.
func (s *Service) WithdrawCandidate(ctx context.Context, actor *model.User, electionID int64) error {
	e, err := s.store.GetElection(ctx, electionID)
	if err != nil {
		return err
	}
	if e.Status != model.ElectionStatusNomination {
		return ErrWrongPhase
	}
	return s.store.DeleteCandidate(ctx, electionID, actor.ID)
}

// OpenVoting closes the nomination phase and opens voting. Requires at least
// one registered candidate so the ballot is never empty.
func (s *Service) OpenVoting(ctx context.Context, actor *model.User, electionID int64) error {
	if actor.Role != model.RoleSyndic {
		return ErrNotSyndic
	}
	e, err := s.store.GetElection(ctx, electionID)
	if err != nil {
		return err
	}
	if e.Status != model.ElectionStatusNomination {
		return ErrWrongPhase
	}
	candidates, err := s.store.ListCandidates(ctx, electionID)
	if err != nil {
		return err
	}
	if len(candidates) == 0 {
		return &ValidationError{Message: "There are no candidates yet."}
	}
	return s.store.SetStatus(ctx, electionID, model.ElectionStatusVoting, nil)
}

// Vote records a single vote from an owner during the voting phase. Each
// owner votes once; the vote cannot be changed.
func (s *Service) Vote(ctx context.Context, actor *model.User, electionID, candidateID int64) error {
	if actor.Role != model.RoleOwner {
		return ErrNotOwner
	}
	e, err := s.store.GetElection(ctx, electionID)
	if err != nil {
		return err
	}
	if e.Status != model.ElectionStatusVoting {
		return ErrWrongPhase
	}
	candidates, err := s.store.ListCandidates(ctx, electionID)
	if err != nil {
		return err
	}
	valid := false
	for _, c := range candidates {
		if c.ID == candidateID {
			valid = true
			break
		}
	}
	if !valid {
		return ErrInvalidCandidate
	}
	return s.store.CreateVote(ctx, electionID, actor.ID, candidateID)
}

// CloseElection ends the voting phase. With a winner, the election enters the
// transition (handover) phase: the winner is announced but the previous
// syndic stays in office until the configured period elapses, when
// SettleTransitions promotes the winner automatically. Ties and void
// elections close outright without a winner.
func (s *Service) CloseElection(ctx context.Context, actor *model.User, electionID int64) error {
	if actor.Role != model.RoleSyndic {
		return ErrNotSyndic
	}
	e, err := s.store.GetElection(ctx, electionID)
	if err != nil {
		return err
	}
	if e.Status != model.ElectionStatusVoting {
		return ErrWrongPhase
	}
	tallies, err := s.store.Tally(ctx, electionID)
	if err != nil {
		return err
	}

	var winnerID int64
	maxVotes, tied := 0, false
	for _, t := range tallies {
		switch {
		case t.Votes > maxVotes:
			maxVotes, winnerID, tied = t.Votes, t.Candidate.UserID, false
		case t.Votes == maxVotes:
			tied = true
		}
	}
	if maxVotes == 0 || tied {
		winnerID = 0
	}

	now := s.clock()
	var transitionEndsAt *time.Time
	if winnerID != 0 {
		ends := now.Add(time.Duration(e.TransitionDays) * 24 * time.Hour)
		transitionEndsAt = &ends
	}
	return s.store.CloseElection(ctx, electionID, winnerID, now, transitionEndsAt)
}

// SettleTransitions promotes the winners of elections whose handover period
// has ended. It is called automatically by the app's background loop and
// opportunistically when election pages are loaded.
func (s *Service) SettleTransitions(ctx context.Context) error {
	return s.store.SettleTransitions(ctx, s.clock())
}
