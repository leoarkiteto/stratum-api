package election

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/leoarkiteto/stratum/internal/model"
)

const electionColumns = `
	e.id, e.title, e.description, e.status, e.created_by, u.name,
	e.created_at, e.transition_days, e.transition_ends_at, e.closed_at, e.winner_id, w.name`

// Store provides election persistence against Postgres.
type Store struct {
	db *sql.DB
}

// NewStore builds an election store on the given database handle.
func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

// CreateElection inserts an election in the nomination phase.
func (s *Store) CreateElection(ctx context.Context, e *model.Election) error {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO elections (title, description, status, created_by, transition_days)
		VALUES ($1, $2, 'nomination', $3, $4)
		RETURNING id, created_at`,
		e.Title, e.Description, e.CreatedBy, e.TransitionDays,
	).Scan(&e.ID, &e.CreatedAt)
	return err
}

// GetElection returns the election with the given id, or ErrNotFound.
func (s *Store) GetElection(ctx context.Context, id int64) (*model.Election, error) {
	e := &model.Election{}
	var transitionEndsAt sql.NullTime
	var closedAt sql.NullTime
	var winnerID sql.NullInt64
	var winnerName sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT`+electionColumns+`
		FROM elections e
		JOIN users u ON u.id = e.created_by
		LEFT JOIN users w ON w.id = e.winner_id
		WHERE e.id = $1`, id,
	).Scan(&e.ID, &e.Title, &e.Description, &e.Status, &e.CreatedBy, &e.CreatorName,
		&e.CreatedAt, &e.TransitionDays, &transitionEndsAt, &closedAt, &winnerID, &winnerName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if transitionEndsAt.Valid {
		e.TransitionEndsAt = &transitionEndsAt.Time
	}
	if closedAt.Valid {
		e.ClosedAt = &closedAt.Time
	}
	if winnerID.Valid {
		e.WinnerID = &winnerID.Int64
		e.WinnerName = winnerName.String
	}
	return e, nil
}

// ListElections returns all elections, newest first, with candidate counts.
func (s *Store) ListElections(ctx context.Context) ([]*model.Election, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT e.id, e.title, e.status, u.name, e.created_at, COUNT(c.id) AS candidate_count
		FROM elections e
		JOIN users u ON u.id = e.created_by
		LEFT JOIN candidates c ON c.election_id = e.id
		GROUP BY e.id, e.title, e.status, u.name, e.created_at
		ORDER BY e.created_at DESC, e.id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var elections []*model.Election
	for rows.Next() {
		e := &model.Election{}
		if err := rows.Scan(&e.ID, &e.Title, &e.Status, &e.CreatorName, &e.CreatedAt, &e.CandidateCount); err != nil {
			return nil, err
		}
		elections = append(elections, e)
	}
	return elections, rows.Err()
}

// SetStatus updates an election's status (used to open voting).
func (s *Store) SetStatus(ctx context.Context, id int64, status model.ElectionStatus, closedAt *time.Time) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE elections SET status = $2, closed_at = $3 WHERE id = $1`,
		id, string(status), closedAt)
	return err
}

// CreateCandidate registers a candidate, returning ErrAlreadyCandidate when
// the user already registered in this election.
func (s *Store) CreateCandidate(ctx context.Context, c *model.Candidate) error {
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO candidates (election_id, user_id, statement)
		VALUES ($1, $2, $3)
		RETURNING id, created_at`,
		c.ElectionID, c.UserID, c.Statement,
	).Scan(&c.ID, &c.CreatedAt)
	if isUniqueViolation(err) {
		return ErrAlreadyCandidate
	}
	return err
}

// ListCandidates returns the election's candidates with their names.
func (s *Store) ListCandidates(ctx context.Context, electionID int64) ([]model.Candidate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.election_id, c.user_id, u.name, c.statement, c.created_at
		FROM candidates c
		JOIN users u ON u.id = c.user_id
		WHERE c.election_id = $1
		ORDER BY c.created_at, c.id`, electionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var candidates []model.Candidate
	for rows.Next() {
		var c model.Candidate
		if err := rows.Scan(&c.ID, &c.ElectionID, &c.UserID, &c.Name, &c.Statement, &c.CreatedAt); err != nil {
			return nil, err
		}
		candidates = append(candidates, c)
	}
	return candidates, rows.Err()
}

// DeleteCandidate removes a candidacy, returning ErrNotCandidate when the
// user is not registered.
func (s *Store) DeleteCandidate(ctx context.Context, electionID, userID int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM candidates WHERE election_id = $1 AND user_id = $2`,
		electionID, userID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotCandidate
	}
	return nil
}

// HasVoted reports whether the voter already voted in the election.
func (s *Store) HasVoted(ctx context.Context, electionID, voterID int64) (bool, error) {
	var voted bool
	err := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM votes WHERE election_id = $1 AND voter_id = $2)`,
		electionID, voterID,
	).Scan(&voted)
	return voted, err
}

// CreateVote records a vote, returning ErrAlreadyVoted when the voter already
// voted in this election.
func (s *Store) CreateVote(ctx context.Context, electionID, voterID, candidateID int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO votes (election_id, voter_id, candidate_id) VALUES ($1, $2, $3)`,
		electionID, voterID, candidateID)
	if isUniqueViolation(err) {
		return ErrAlreadyVoted
	}
	return err
}

// Tally counts votes per candidate, ordered by votes (desc) then id (asc).
func (s *Store) Tally(ctx context.Context, electionID int64) ([]model.Tally, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.id, c.election_id, c.user_id, u.name, c.statement, c.created_at, COUNT(v.id) AS vote_count
		FROM candidates c
		JOIN users u ON u.id = c.user_id
		LEFT JOIN votes v ON v.candidate_id = c.id
		WHERE c.election_id = $1
		GROUP BY c.id, c.election_id, c.user_id, u.name, c.statement, c.created_at
		ORDER BY vote_count DESC, c.id`, electionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tallies []model.Tally
	for rows.Next() {
		var t model.Tally
		if err := rows.Scan(&t.Candidate.ID, &t.Candidate.ElectionID, &t.Candidate.UserID,
			&t.Candidate.Name, &t.Candidate.Statement, &t.Candidate.CreatedAt, &t.Votes); err != nil {
			return nil, err
		}
		tallies = append(tallies, t)
	}
	return tallies, rows.Err()
}

// CloseElection closes the voting phase. With a winner, the election moves to
// the transition phase (handover) ending at transitionEndsAt, and the
// previous syndic stays in office; without a winner it closes outright.
// The winner is only promoted by SettleTransitions once the handover ends.
func (s *Store) CloseElection(ctx context.Context, electionID, winnerID int64, closedAt time.Time, transitionEndsAt *time.Time) error {
	status := string(model.ElectionStatusClosed)
	if transitionEndsAt != nil {
		status = string(model.ElectionStatusTransition)
	}
	res, err := s.db.ExecContext(ctx,
		`UPDATE elections SET status = $2, closed_at = $3, winner_id = $4, transition_ends_at = $5
		 WHERE id = $1 AND status = 'voting'`,
		electionID, status, closedAt, nullableInt64(winnerID), transitionEndsAt)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrWrongPhase
	}
	return nil
}

// SettleTransitions promotes the winner of every election whose handover
// period has ended and marks those elections closed, atomically. Elections
// without a recorded winner are closed without any role change.
func (s *Store) SettleTransitions(ctx context.Context, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
		SELECT id, winner_id FROM elections
		WHERE status = 'transition' AND transition_ends_at <= $1
		FOR UPDATE`, now)
	if err != nil {
		return err
	}
	type pending struct {
		id       int64
		winnerID int64
	}
	var pendingList []pending
	for rows.Next() {
		var p pending
		var winnerID sql.NullInt64
		if err := rows.Scan(&p.id, &winnerID); err != nil {
			rows.Close()
			return err
		}
		if winnerID.Valid {
			p.winnerID = winnerID.Int64
		}
		pendingList = append(pendingList, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, p := range pendingList {
		if p.winnerID != 0 {
			// Demote the previous syndic(s), except when the winner is the incumbent.
			if _, err := tx.ExecContext(ctx,
				`UPDATE users SET role = 'owner' WHERE role = 'syndic' AND id <> $1`, p.winnerID); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx,
				`UPDATE users SET role = 'syndic' WHERE id = $1`, p.winnerID); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx,
			`UPDATE elections SET status = 'closed' WHERE id = $1`, p.id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// nullableInt64 returns a nil any for 0 so Postgres stores NULL.
func nullableInt64(v int64) any {
	if v == 0 {
		return nil
	}
	return v
}

// isUniqueViolation reports whether err is a Postgres unique_violation (23505).
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
