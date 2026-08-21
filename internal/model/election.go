package model

import "time"

// ElectionStatus is the lifecycle phase of a syndic election.
type ElectionStatus string

const (
	// ElectionStatusNomination accepts candidate registrations.
	ElectionStatusNomination ElectionStatus = "nomination"
	// ElectionStatusVoting accepts votes from owners.
	ElectionStatusVoting ElectionStatus = "voting"
	// ElectionStatusTransition is the handover period: the winner is known but
	// the previous syndic stays in office until TransitionEndsAt.
	ElectionStatusTransition ElectionStatus = "transition"
	// ElectionStatusClosed means the handover finished (or there was no
	// winner) and the result is final.
	ElectionStatusClosed ElectionStatus = "closed"
)

// Election is a syndic election. It moves nomination → voting → transition
// (when there is a winner) → closed.
type Election struct {
	ID               int64
	Title            string
	Description      string
	Status           ElectionStatus
	CreatedBy        int64
	CreatorName      string
	CreatedAt        time.Time
	TransitionDays   int        // handover period configured at creation (default 7)
	TransitionEndsAt *time.Time // set when voting closes with a winner
	ClosedAt         *time.Time
	WinnerID         *int64
	WinnerName       string
	CandidateCount   int
}

// Candidate is a user running in an election, with an optional statement.
type Candidate struct {
	ID         int64
	ElectionID int64
	UserID     int64
	Name       string
	Statement  string
	CreatedAt  time.Time
}

// Tally is a candidate's vote count in a closed election.
type Tally struct {
	Candidate Candidate
	Votes     int
}

// ElectionView is the data the election detail page renders for a user:
// the election plus the actions that user may take at its current phase.
type ElectionView struct {
	Election      *Election
	Candidates    []Candidate
	Tallies       []Tally
	IsCandidate   bool
	HasVoted      bool
	CanRegister   bool
	CanWithdraw   bool
	CanVote       bool
	CanOpenVoting bool
	CanClose      bool
	IsOwner       bool
}
