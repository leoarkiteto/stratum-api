# Feature Specification: Auth Module Documentation

**Feature Branch**: `001-auth-module-docs`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "Document the auth module: Session ID + cookie + CSRF"
and "Document in the Auth module: we are using RBAC (Role based access control)
with 3 profile (syndic | owner | tenant)"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Onboarding Developer Understands the Auth Flow (Priority: P1)

A developer who is new to the project opens the documentation and can follow,
end to end, how authentication works: how a session is born, what travels in
the cookie, how the session becomes "logged in", how CSRF protects forms, and
how logout ends the session. After reading, they can answer "what happens on
the server when someone submits the login form?" and know which module owns
each responsibility.

**Why this priority**: Without an accurate mental model of the session + cookie
+ CSRF mechanism, any change to auth code risks breaking security guarantees.
This is the primary value of the deliverable.

**Independent Test**: A developer new to the codebase reads only the produced
documentation, then describes the full request→response journey for
registration, login and logout, correctly naming the session cookie, the
session store, and where CSRF tokens come from. The description is complete
with no gaps needing code reading.

**Acceptance Scenarios**:

1. **Given** a developer new to the project, **When** they read the
   documentation without opening the source code, **Then** they can explain the
   session lifecycle (creation, login binding, rotation, expiry, logout
   deletion) with no major gaps.
2. **Given** the documentation, **When** a developer searches for how CSRF
   tokens are validated, **Then** they find a section that describes where the
   token comes from, where it must appear, and how it is checked on
   state-changing requests.
3. **Given** the documentation, **When** a developer looks up the session
   cookie, **Then** they find its name, its security attributes, and a clear
   statement that it carries only an opaque token — no readable session data.

---

### User Story 2 - Security Reviewer Can Verify the Protections (Priority: P2)

A reviewer (security-focused developer or auditor) uses the documentation as a
checklist to confirm that the authentication mechanism applies its stated
defenses: token hashing at rest, fixation-safe login, httpOnly + SameSite
cookie attributes, CSRF protection on every state-changing form, and session
expiry. The docs state both what is protected and what the protections do not
cover.

**Why this priority**: Verification is only possible when behavior is documented
precisely; vague docs fail security review. This story ensures the deliverable
supports audits without reading every line of code.

**Independent Test**: A reviewer maps each documented security property to the
actual implementation (code + configuration) and finds every property
accurate and every state-changing form covered. A short checklist derived from
the docs has zero unanswered items.

**Acceptance Scenarios**:

1. **Given** the documentation, **When** a reviewer checks the session cookie,
   **Then** the documented attributes (httpOnly, SameSite, Secure behavior
   under configuration) match the implementation exactly.
2. **Given** the documentation, **When** a reviewer enumerates all
   state-changing forms, **Then** each one is documented as carrying and
   validating a CSRF token.
3. **Given** the documentation, **When** a reviewer looks for limitations and
   threat-model boundaries, **Then** the docs state what the mechanisms defend
   against and what remains out of scope (e.g., protections that require
   HTTPS/`COOKIE_SECURE` to take full effect).

---

### User Story 3 - Maintainer Understands Auth Configuration & Operations (Priority: P3)

A project maintainer or operator uses the documentation to understand the
configuration surface that touches authentication (session lifetime, secure
cookie flag, password pepper) and the operational consequences of each choice
(e.g., rotating the pepper invalidates stored password hashes; disabling secure
cookies weakens protection on HTTP).

**Why this priority**: Valuable operational guidance, but secondary to the
core understanding and verification stories above.

**Independent Test**: A maintainer reads the configuration section and can state
the security impact of each auth-related setting without consulting the source
code or README.

**Acceptance Scenarios**:

1. **Given** the documentation, **When** a maintainer reads the configuration
   section, **Then** every auth-related setting is listed with its default, its
   effect, and its security implications.
2. **Given** the documentation, **When** a maintainer considers rotating the
   password pepper, **Then** the docs warn that this invalidates all existing
   password hashes.

---

### User Story 4 - RBAC: The Three Profiles Are Documented (Priority: P2)

A developer or security reviewer reads the documentation and can state exactly
what role-based access control means in this project: the three profiles
`syndic` / `owner` / `tenant`, what each profile is permitted to do as
currently implemented, how a profile is obtained (self-service registration
vs out-of-band grant), and where authorization is enforced (server-side, per
request — never only in the UI).

**Why this priority**: RBAC is the authorization model of the whole product
domain, so it ranks second only to the core session/cookie/CSRF flow story; it
must be documented for both onboarding and security review.

**Independent Test**: A developer reads only the documentation and correctly
lists the three profiles, states who may obtain each by self-service, explains
how `syndic` is granted, and names where access decisions are enforced —
without reading source code.

**Acceptance Scenarios**:

1. **Given** the documentation, **When** a reader looks for the access model,
   **Then** they find exactly three profiles (`syndic` / `owner` / `tenant`)
   with the current permissions of each stated.
2. **Given** the documentation, **When** a reader checks how profiles are
   assigned, **Then** the docs state that self-service registration yields
   `owner`/`tenant` and that `syndic` is granted out of band.
3. **Given** the documentation, **When** a reader looks for enforcement, **Then**
   the docs state that authorization is decided server-side per request and
   flag any role-based permission not yet enforced (per FR-011).

---

### Edge Cases

- Session cookie missing or unreadable on a POST to a protected form — what the
  system does and what the user sees.
- Session expired while a user is mid-form — behavior and recovery path.
- CSRF token missing or mismatched on a state-changing request — request
  rejected, user sees a session-expired message.
- Repeated login attempt with invalid credentials — error surfaced, no session
  escalation.
- Logout when the session is already gone or the cookie is absent — no error,
  clean state.
- Registering with an email that already exists — duplicate rejected with a
  clear message.
- Running with `COOKIE_SECURE=false` locally vs `true` in production — documented
  difference in exposure.
- Self-service registration attempting to reach `syndic` — must not be possible;
  documented as out-of-band only.
- Unauthenticated access to a page that requires a signed-in session — user is
  sent to login, and the behavior is documented.
- Role-based permissions for features not yet permissioned — documented as
  gaps (FR-011/FR-014), never presented as enforced behavior.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Documentation MUST describe the full session lifecycle: anonymous
  session creation, user binding at login, token rotation (fixation
  prevention), expiry by TTL, and deletion on logout.
- **FR-002**: Documentation MUST specify the session cookie: its name, its
  purpose, its attributes (httpOnly, SameSite, Secure under configuration), and
  explicitly that it carries only an opaque random token, never readable user
  data.
- **FR-003**: Documentation MUST explain how session tokens are stored
  server-side (one-way hashed) so the raw token is never persisted, and the
  reason for that defense-in-depth choice.
- **FR-004**: Documentation MUST explain CSRF protection: how the CSRF token is
  generated and bound to the session, where it is embedded in forms, how it is
  validated on every state-changing request, and that comparison is done in
  constant time.
- **FR-005**: Documentation MUST describe the register, login and logout flows,
  including error paths (invalid credentials, duplicate email, expired or
  invalid session) and what the user sees in each case.
- **FR-006**: Documentation MUST describe the role model as currently
  implemented (syndic / owner / tenant), which roles can self-register, and how
  restricted roles are granted.
- **FR-007**: Documentation MUST cover auth-related configuration (session
  lifetime, secure cookie flag, password pepper) with defaults and the security
  implications of each setting.
- **FR-008**: Documentation MUST state the threat model: what the session +
  cookie + CSRF mechanisms defend against and their known limitations (e.g.,
  protections that only apply under HTTPS).
- **FR-009**: Every documented claim MUST be accurate against the current
  implementation; documentation MUST be reviewed against the code before it is
  considered complete.
- **FR-010**: Documentation MUST be discoverable: reachable from the project's
  main README within one link.
- **FR-011**: The documentation deliverable MUST NOT change application
  behavior — it documents the as-built auth module only. Any gap found while
  writing it is reported as a follow-up, not fixed in this effort.
- **FR-012**: Documentation MUST describe the RBAC model: exactly three
  profiles (`syndic` / `owner` / `tenant`), the current permissions of each,
  and that access decisions are made server-side per request — never UI-only.
- **FR-013**: Documentation MUST explain profile assignment: self-service
  registration creates `owner`/`tenant` accounts only, and `syndic` is granted
  out of band.
- **FR-014**: Documentation MUST mark each documented permission as enforced
  today or as a stated gap; unenforced permissions MUST be flagged, never
  presented as working behavior (see FR-011).
- **FR-015**: The documented access model MUST align with the project
  constitution's access model (Principle VI: three profiles; `syndic` granted
  out of band; server-side enforcement) with no contradictions.

### Key Entities *(include if feature involves data)*

- **Session**: A server-side record of an anonymous or authenticated user
  interaction; identified by a random token known only to the browser. Key
  attributes: user binding (or none), CSRF binding, expiry time.
- **Session Cookie**: The browser-side handle — an opaque token that the server
  uses to look up the session. Carries no user data or session content itself.
- **CSRF Token**: A per-session secret embedded in state-changing forms and
  validated server-side, proving the request originated from a page the session
  actually rendered. Relationship: bound to the session, never stored in the
  cookie.
- **User & Role**: The account bound to a session after login, carrying exactly
  one of three authorization profiles — `syndic` (condominium administrator),
  `owner` (unit owner) or `tenant` (renter). The role determines what the
  session may do; `owner`/`tenant` are obtainable via self-service
  registration, while `syndic` is granted out of band.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A developer new to the project can explain the complete auth
  flow (session lifecycle, cookie contents and attributes, CSRF validation)
  and the RBAC model (the three profiles and their current permissions) using
  only the documentation, in under 30 minutes, with no gaps that force reading
  source code.
- **SC-002**: 100% of documented claims are verified accurate against the
  implementation by a code review pass (zero inaccuracies found).
- **SC-003**: A security-review checklist derived from the documentation has
  zero unanswered items: cookie attributes, token hashing, login rotation,
  CSRF on every state-changing form, expiry, and RBAC authorization are all
  documented and correct.
- **SC-004**: The documentation is reachable from the project README in one
  link and a reader can find any of the required topics (session, cookie, CSRF,
  RBAC roles, configuration) within three navigations.
- **SC-005**: The documentation effort introduces zero changes to application
  behavior (no code, migration, or configuration changes outside documentation
  files).
- **SC-006**: A reader can determine from the docs, for each of the three
  profiles, which permissions are granted today and how the profile is
  obtained — with zero ambiguity.
- **SC-007**: The documentation identifies every role-based permission that is
  not yet enforced as a gap; a review pass finds no undocumented enforcement
  gaps.
- **SC-008**: The documented access model matches the project constitution
  (exactly three profiles, `syndic` granted out of band, server-side
  enforcement) with no contradictions.

## Assumptions

- **Audience**: The documentation targets developers and security reviewers who
  maintain or audit the project; it is technical documentation of the module's
  internals, not end-user help content.
- **Language & format**: Written in English, in Markdown, following the
  project's existing documentation conventions (README/`REASONIX.md` style).
- **Location**: Lives under a `docs/` directory with an entry-point document
  linked from the README (one link per FR-010).
- **Documentation only**: This feature documents the as-built behavior; it does
  not change, fix, or extend the auth implementation. Discovered gaps become
  follow-up items, not in-scope fixes.
- **Source of truth**: The current implementation (Postgres-backed sessions,
  opaque hashed cookie tokens, bound CSRF tokens, Argon2id password hashing
  with a pepper, roles syndic/owner/tenant) is authoritative; the README and
  `REASONIX.md` are used as cross-reference, and discrepancies are resolved in
  favor of the code and flagged.
- **RBAC documentation scope**: The docs describe the three-profile RBAC model
  and its current enforcement as built. A permission matrix is included only
  for permissions actually implemented; unenforced permissions are documented
  as gaps (FR-011/FR-014), not as working behavior.
