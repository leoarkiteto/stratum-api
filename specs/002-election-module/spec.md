# Feature Specification: Election Module

**Feature Branch**: `002-election-module`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "Document `Election` module: Feature managed by
syndic to perform election for new syndic, where the winner will become new
syndic (switch role from "owner" -> "syndic"), by the way only "owner" can be
voted as syndic. After election time the transition (hand over) occur during 1
week"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Syndic Runs an Election for a New Syndic (Priority: P1)

The current `syndic` creates an election: they pick the candidate pool (which
MUST contain only `owner` profile users), set the end time for voting, and
open the election. Residents vote until the end time, at which point the
election closes automatically, the winner is announced, and the handover
period begins.

**Why this priority**: Without the syndic being able to open and close an
election, no handover can ever happen — this is the heart of the feature.

**Independent Test**: A `syndic` can complete a full election lifecycle —
create an election with owner candidates, watch it close at the end time, and
see a winner announced — and this delivers the ability to start the syndic
succession process.

**Acceptance Scenarios**:

1. **Given** a user with the `syndic` profile, **When** they create an election
   with at least one `owner` candidate and a valid end time, **Then** the
   election opens for voting and is visible to eligible residents.
2. **Given** an election in the voting phase, **When** the end time passes,
   **Then** voting closes automatically and the candidate with the most votes
   is declared the winner.
3. **Given** an open election, **When** a non-`syndic` user tries to create,
   modify or cancel it, **Then** the request is denied by the server.

---

### User Story 2 - Residents Cast Their Votes (Priority: P1)

Eligible residents — authenticated users with the `owner` or `tenant` profile —
see the active election and its candidates, cast exactly one vote, and can
follow the outcome once the election closes. Candidates are `owner` users
only; no other profile can appear on the ballot.

**Why this priority**: An election with no voters is meaningless; voting is
what makes the result legitimate.

**Independent Test**: A resident can cast one vote in an active election, is
prevented from voting twice, and sees the announced winner after the close —
delivering a complete, verifiable voting flow.

**Acceptance Scenarios**:

1. **Given** an active election, **When** an authenticated `owner` or `tenant`
   user votes for one of the candidates, **Then** the vote is recorded and the
   user cannot vote again in that election.
2. **Given** the ballot shown to a voter, **When** the candidate list is
   inspected, **Then** every candidate has the `owner` profile and no
   `tenant` or `syndic` user appears on it.
3. **Given** an unauthenticated visitor, **When** they try to vote or view the
   ballot, **Then** they are directed to log in first.

---

### User Story 3 - Handover: The One-Week Role Transition (Priority: P2)

After the election closes, a handover period of exactly one week begins. The
winner does not become `syndic` instantly: the transition happens during that
week, and at its end the roles switch — the winner moves from `owner` to
`syndic`, and the outgoing `syndic` moves to `owner`. Until the handover
completes, the outgoing `syndic` remains in charge and no new election can
start.

**Why this priority**: The controlled transition is what the "1 week handover"
requirement is about; it protects continuity of administration.

**Independent Test**: A completed election followed by the handover week ends
with the winner holding the `syndic` profile and the former syndic holding
`owner`, verified by the system's own records — demonstrating the succession
works end to end.

**Acceptance Scenarios**:

1. **Given** a closed election with a winner, **When** the handover period
   starts, **Then** the winner still holds `owner` and the outgoing `syndic`
   still holds `syndic` for the full 7-day period.
2. **Given** a handover in progress, **When** the 7-day period ends, **Then**
   the winner's role changes to `syndic` and the outgoing syndic's role
   changes to `owner`, and the handover is recorded as complete.
3. **Given** an election that is still in the voting or handover phase, **When**
   the syndic tries to create another election, **Then** the system rejects
   it (only one election may be active at a time).

---

### Edge Cases

- Election created with no candidates — the syndic must add at least one
  `owner` candidate before the election can open.
- Election end time in the past or earlier than the creation moment — rejected
  as invalid.
- Two or more candidates tied for the most votes — the election is declared
  void, no role change occurs, and the syndic may create a new election.
- A voter attempts a second vote in the same election — the second vote is
  rejected; the first vote stands.
- A non-`owner` user is added as a candidate — rejected by the system.
- The syndic cancels an election while voting is open — votes are discarded
  and no handover starts.
- No user votes before the end time — the election is void (no winner), and
  the syndic may run a new election.
- The outgoing syndic's session/role during the handover — they keep full
  `syndic` powers until the handover completes, then the switch applies.
- Handover interruption (e.g., the winner's account is deactivated during the
  week) — the transition must be re-attemptable or the election voided; the
  outcome must never leave the building without a `syndic`.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The system MUST allow a user with the `syndic` profile to create
  an election with one or more candidates and an end time for voting.
- **FR-002**: Only users with the `owner` profile MUST be eligible as
  candidates; the system MUST reject any candidate who is not an `owner`.
- **FR-003**: The system MUST allow each authenticated resident user with the
  `owner` or `tenant` profile to cast exactly one vote per election; duplicate
  votes MUST be rejected.
- **FR-004**: The system MUST close voting automatically at the election's end
  time and MUST declare the candidate with the most votes as the winner.
- **FR-005**: In the absence of a unique winner (tie, or zero votes cast), the
  system MUST void the election without any role change and MUST allow the
  `syndic` to create a new election.
- **FR-006**: After an election closes with a winner, the system MUST start a
  handover period of exactly 7 days during which no role change occurs.
- **FR-007**: At the end of the 7-day handover, the system MUST switch the
  winner's role from `owner` to `syndic` and the outgoing syndic's role from
  `syndic` to `owner`.
- **FR-008**: The system MUST enforce that only one election is active at a
  time — an election in the voting or handover phase MUST block the creation
  of a new one.
- **FR-009**: The `syndic` MUST be able to cancel an election while voting is
  open; cancellation MUST discard votes and MUST NOT start a handover.
- **FR-010**: All election actions (create, vote, cancel, close, role switch)
  MUST be authorized server-side against the caller's role; UI-level hiding of
  actions MUST NOT be the only enforcement.
- **FR-011**: The system MUST record the election outcome (winner, end time,
  handover start and completion) so the role transition is auditable, and MUST
  leave the building with exactly one `syndic` at all times.

### Key Entities *(include if feature involves data)*

- **Election**: A succession event created by the `syndic`. Carries its
  candidates, its end time, its phase (voting / closed / handover / completed /
  void / cancelled) and its declared winner (if any).
- **Candidate**: A user with the `owner` profile entered in an election.
  Relationship: an election has one or more candidates; the winner is the
  candidate with the most votes.
- **Vote**: A single, immutable ballot cast by an eligible resident
  (`owner`/`tenant`) for one candidate in one election. Relationship: each
  eligible user has at most one vote per election.
- **Handover**: The 7-day transition period between election close and role
  switch. Relationship: belongs to exactly one election with a winner; its
  completion triggers the `owner → syndic` / `syndic → owner` role change.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A full election run (create → vote → close → 7-day handover →
  role switch) completes end to end, and the system records the winner
  holding `syndic` and the outgoing syndic holding `owner` — verified by the
  audit record with zero manual intervention.
- **SC-002**: 100% of ballots cast in an election are counted exactly once;
  duplicate and ineligible votes are rejected in 100% of attempts.
- **SC-003**: Every ballot's candidate list contains only `owner`-profile
  users; a review of any completed election finds zero non-`owner` candidates.
- **SC-004**: The handover period lasts exactly 7 days in 100% of completed
  elections, and no role change occurs before it ends.
- **SC-005**: At no point does the system end up with zero or more than one
  `syndic`; role-switch checks after any election find exactly one `syndic`.
- **SC-006**: A resident can locate the active election, cast a vote, and see
  the result within 3 navigations and under 2 minutes of effort.

## Assumptions

- **Feature is not yet implemented**: The Election module does not exist in the
  codebase today; this specification defines the feature to be built. It is not
  documentation of existing behavior.
- **Voters**: Every authenticated resident (`owner` or `tenant`) gets one
  vote. Per-unit or area-weighted voting is out of scope for this version
  because the platform has no unit entity yet; it can be added later without
  changing the election model.
- **Candidacy**: Only `owner`-profile users can be candidates. The current
  `syndic` cannot be a candidate (they are not an `owner` while in office).
- **Outgoing syndic**: The outgoing `syndic` becomes `owner` when the handover
  completes, so the building always has exactly one `syndic`.
- **Timing**: Voting opens when the syndic creates the election and closes at
  the chosen end time; the handover is fixed at 7 days and the role switch
  happens at the end of that week (not at election close).
- **Concurrency**: Only one election is active (voting or handover) at a time.
- **Eligibility for management**: Only the current `syndic` can create, cancel
  or otherwise manage an election, per the RBAC model in the constitution.
