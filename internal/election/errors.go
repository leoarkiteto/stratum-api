// Package election implements the syndic election feature: creation,
// candidate registrations, voting and automatic promotion of the winner.
// Elections move nomination → voting → closed, driven by the current syndic.
package election

import "errors"

// Domain errors, mapped by handlers to user-facing messages.
var (
	// ErrNotFound means the election does not exist.
	ErrNotFound = errors.New("election: not found")
	// ErrNotSyndic means only the current syndic may perform the action.
	ErrNotSyndic = errors.New("election: only the syndic can do this")
	// ErrNotOwner means only owners may perform the action (voting).
	ErrNotOwner = errors.New("election: only owners can do this")
	// ErrNotEligible means the user's role cannot run for syndic.
	ErrNotEligible = errors.New("election: only owners and the current syndic can run")
	// ErrWrongPhase means the action does not apply to the current status.
	ErrWrongPhase = errors.New("election: wrong phase")
	// ErrAlreadyCandidate means the user already registered in this election.
	ErrAlreadyCandidate = errors.New("election: already a candidate")
	// ErrNotCandidate means the user is not a candidate (or the candidate is unknown).
	ErrNotCandidate = errors.New("election: not a candidate")
	// ErrAlreadyVoted means the user already voted in this election.
	ErrAlreadyVoted = errors.New("election: already voted")
	// ErrInvalidCandidate means the submitted candidate does not belong to this election.
	ErrInvalidCandidate = errors.New("election: invalid candidate")
)

// ValidationError is a user-input error carrying a human-readable message.
type ValidationError struct{ Message string }

// Error implements the error interface.
func (e *ValidationError) Error() string { return e.Message }
