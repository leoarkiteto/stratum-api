# Feature Specification: Hexagonal Architecture & Vertical Slice Migration

**Feature Branch**: `003-hexagonal-vertical-slice`

**Created**: 2026-08-22

**Status**: Draft

**Input**: User description: "Migrate the current architecture to "Hexagonal
Architecture" + "Vertical Slice" in order to make each feature self-contained.
Following a suggestion of folder structure: `internal/feature-1/{domain.go,
ports.go, service.go, http_handler.go, postgres_repository.go}` and
`internal/shared/{database, httputil}`"

## Background & Current State

Stratum is a server-rendered modular monolith on the GOTTH stack. Today the
codebase already moves in the direction of the constitution: user-facing
features live under `internal/` (`auth`, `home`), services depend on
interfaces rather than concrete stores, and handlers are relatively thin.
However, the feature packages are not yet fully self-contained vertical
slices: shared concerns (`web`, `session`, `db`, `config`, `model`,
`password`) are spread across packages whose boundaries do not yet match the
constitution's target, and there is no single, predictable layout for a
feature's domain, ports, service and adapters.

This specification defines the migration that makes every feature a
self-contained hexagonal vertical slice, using the suggested structure below
as the reference target. No new user-facing functionality is added; the goal
is structural clarity, independent testability, and consistency with
Principles I and V of the constitution.

## Clarifications

### Session 2026-08-22

- Q: For features that own no persistence today (like `home`), how should the
  migration satisfy the required vertical-slice layout? → A: Features without
  persistence are exempt from `postgres_repository.go`; they keep `domain.go`,
  `ports.go`, `service.go` and `http_handler.go`, and add a repository only
  when they start persisting data.
- Q: Which existing packages should be relocated under `internal/shared/`
  during the migration? → A: All shared concerns move under
  `internal/shared/`: `config`, `db` → `database`, `model`, `password`,
  `session`, `web` → `httputil`, and the shared base layout →
  `internal/shared/templates`.
- Q: How should the feature/shared boundary rules (no feature-to-feature
  imports, no legacy shared imports) be enforced so violations are caught? →
  A: An automated architecture test runs inside the standard Go test suite
  and fails `go test` / the quality gate on any boundary violation.

## Target Structure (Reference)

The migration adopts the suggested layout, adjusted to the existing Go
conventions of this repository:

- Each user-facing feature becomes one package under `internal/`
  (for example `internal/auth/`, `internal/home/`) containing:
  - `domain.go` — the feature's domain types and rules
  - `ports.go` — the interfaces the feature's service depends on
  - `service.go` — the feature's business logic
  - `http_handler.go` — the HTTP transport adapter
  - `postgres_repository.go` — the Postgres persistence adapter (only for
    features that persist data)
  - feature-owned templates remain inside the feature folder
- Shared infrastructure lives under `internal/shared/`:
  - `database/` — connection and migration support (currently `internal/db`)
  - `httputil/` — rendering, cookies, CSRF and middleware helpers (currently
    `internal/web`)
  - other shared concerns move under `internal/shared/` as well: `config`,
    `model`, `password`, `session`, and the shared base layout →
    `internal/shared/templates`

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Self-Contained Feature Slices (Priority: P1)

A developer can open any feature folder, understand the whole feature by
reading that one folder, and trust that the feature does not reach into
another feature's internals. Adding or modifying a feature follows one
predictable layout.

**Why this priority**: This is the core goal of the migration — each feature
self-contained in one vertical slice. Without it, the other benefits of the
hexagonal structure cannot be realized.

**Independent Test**: List the contents of any migrated feature folder and
confirm it contains the feature's domain, ports, service and HTTP handler
(plus `postgres_repository.go` when the feature persists data), and run a
dependency check that finds zero imports from one feature package into
another feature's internal code.

**Acceptance Scenarios**:

1. **Given** the migrated codebase, **When** a developer lists any feature
   folder under `internal/`, **Then** the folder contains the feature's domain,
   ports, service and HTTP handler (plus `postgres_repository.go` when the
   feature persists data), and no source file belonging to another feature.
2. **Given** the migrated codebase, **When** a dependency analysis runs across
   feature packages, **Then** no feature package imports another feature
   package's internal code.
3. **Given** a request to add a new feature, **When** a developer creates a new
   folder under `internal/`, **Then** all feature-specific code fits in that
   folder except shared infrastructure and top-level app wiring.

---

### User Story 2 - Business Logic Isolated Behind Ports (Priority: P1)

A developer can test and change a feature's business rules without a database,
without a web server, and without reading transport or persistence code.
HTTP and persistence concerns are adapters that implement the feature's ports.

**Why this priority**: This is what makes the vertical slices hexagonal —
business logic testable in isolation and adapters swappable. It is the main
engineering value of the migration alongside Story 1.

**Independent Test**: Run a feature's service unit tests with no database or
HTTP server, and review the feature's handler and repository files to confirm
business rules live only in the service.

**Acceptance Scenarios**:

1. **Given** any feature's service tests, **When** the tests run, **Then** they
   pass without a database connection and without starting an HTTP server.
2. **Given** any feature's HTTP handler, **When** reviewed, **Then** it only
   parses/validates the request, calls one service method, and renders or
   redirects — it contains no business rules and no SQL.
3. **Given** any feature's Postgres repository, **When** reviewed, **Then** it
   is the only file in the feature that executes SQL for that feature's data,
   and it implements the feature's ports.

---

### User Story 3 - Shared Infrastructure Reused, Behavior Unchanged (Priority: P2)

A developer finds shared database and HTTP utilities in one place under
`internal/shared/` instead of duplicated across features, and after the
migration residents and administrators can still register, log in, view the
dashboard and log out exactly as before.

**Why this priority**: Shared infrastructure is what every slice depends on,
and preserving user-visible behavior is the safety gate that makes the
migration shippable.

**Independent Test**: Start the application, run the existing automated test
suite, and complete the register → login → dashboard → logout flow; confirm
the flow behaves identically to before the migration and that each shared
helper exists once under `internal/shared/`.

**Acceptance Scenarios**:

1. **Given** the migrated codebase, **When** the full automated test suite
   runs, **Then** all behavioral tests that passed before the migration still
   pass (mechanical import-path updates to moved packages are allowed).
2. **Given** the migrated codebase, **When** the application starts, **Then** a
   user can register, log in, view the dashboard and log out with the same
   routes, pages and role checks as before.
3. **Given** the migrated codebase, **When** database and HTTP helper code is
   inspected, **Then** each helper exists once in the shared area rather than
   being reimplemented per feature.

---

### Edge Cases

- A feature needs data or behavior owned by another feature — it MUST use an
  explicitly exposed port/boundary of that feature or a shared service; it MUST
  NOT import the other feature's internals. If no boundary exists, one is
  defined before coupling is introduced.
- Existing shared packages not named in the suggested layout (`config`,
  `model`, `password`, `session`, shared rendering/templates) — they are
  relocated under `internal/shared/` (`config`, `model`, `password`,
  `session`, `templates`); they never become the internals of a single
  feature.
- Cross-cutting pages (for example the dashboard, which needs session, user and
  auth data) — they depend on ports exposed by the relevant slices/shared
  packages, never on another feature's internals.
- Generated template files (`*_templ.go`) — they move with their feature and
  remain committed; the compiled stylesheet and vendored HTMX stay under
  `static/` and must keep working unchanged.
- Store/session integration tests that need a database — they continue to skip
  unless `TEST_DATABASE_URL` is set, exactly as today.
- Large package moves — the migration is done incrementally so the build stays
  green at every committed step.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: The codebase MUST be organized into self-contained vertical
  slices: one package per user-facing feature under `internal/`, each owning
  its domain, ports, service and HTTP handler; features that persist data MUST
  also own a Postgres repository, while features without persistence MUST omit
  it.
- **FR-002**: Feature packages MUST NOT import another feature package's
  internal code; any cross-feature access MUST go through an explicitly
  exposed port or a shared package.
- **FR-003**: Business logic MUST live in services that depend only on ports
  (interfaces) — never on HTTP or SQL directly.
- **FR-004**: HTTP handlers MUST be thin adapters: parse/validate the request,
  make exactly one service call, then render a template or redirect; they MUST
  NOT contain business rules.
- **FR-005**: For features that persist data, the Postgres repository MUST be
  the only place where SQL is executed for that feature's persistence, and it
  MUST implement that feature's ports.
- **FR-006**: Shared infrastructure MUST live under `internal/shared/`,
  including at least `database/` and `httputil/`, so all features reuse the
  same database and HTTP utilities without duplicating them.
- **FR-007**: Existing shared packages (`config`, `model`, `password`,
  `session`, and the rendering/cookie/CSRF/auth-middleware concerns currently
  in `web`) MUST be relocated under `internal/shared/` — as `config`, `model`,
  `password`, `session` and `httputil` respectively, with the shared base
  layout under `internal/shared/templates` — and MUST remain owned by no
  single feature.
- **FR-008**: The application MUST remain a single server-rendered modular
  monolith on the GOTTH stack: no JSON REST API, no separate frontend/SPA, and
  route registration confined to the top-level app wiring.
- **FR-009**: All existing user-visible behavior MUST remain unchanged after
  the migration: register, login, logout, dashboard, RBAC role enforcement,
  session handling and CSRF protection behave exactly as before.
- **FR-010**: Tests MUST live next to the feature code they test; unit and
  handler tests MUST NOT require a database; integration tests MUST keep the
  existing `TEST_DATABASE_URL` skip behavior.
- **FR-011**: Generated and vendored artifacts MUST remain committed and
  working: `*_templ.go` files move with their features, and the compiled CSS
  and vendored HTMX stay under `static/`.
- **FR-012**: The project's quality gate MUST pass after the migration:
  formatting, static analysis and the automated test suite all complete
  successfully.
- **FR-013**: The boundary rules from FR-002 and FR-007 MUST be enforced by an
  automated architecture test that runs as part of the standard test suite and
  fails the quality gate on any violation.

### Key Entities *(include if feature involves data)*

- **Feature Slice**: A self-contained vertical package under `internal/`
  containing one feature's domain, ports, service and HTTP handler, plus a
  Postgres repository when the feature persists data. Relationship: depends
  only on shared infrastructure and explicit ports.
- **Port**: An interface owned by a feature that its service depends on for
  persistence or external effects. Relationship: implemented by adapters
  (HTTP handlers, Postgres repositories).
- **Adapter**: A transport or persistence implementation of a port — the HTTP
  handler (inbound) and the Postgres repository (outbound). Relationship: one
  adapter may implement multiple ports of the same feature.
- **Domain Service**: The business-logic unit of a feature slice; depends only
  on ports, never on HTTP or SQL.
- **Shared Infrastructure**: Cross-feature capabilities under
  `internal/shared/` (database, httputil, config, model, password, session)
  available to every feature without coupling to a single feature's internals.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: 100% of feature packages are self-contained — the automated
  architecture test in the standard test suite finds zero imports from one
  feature package into another feature package's internal code and zero
  feature imports of legacy shared paths.
- **SC-002**: 100% of service unit tests run and pass without a database
  connection and without an HTTP server.
- **SC-003**: 100% of HTTP handlers make at most one service call and contain
  no SQL; a review of every handler confirms zero business rules.
- **SC-004**: 100% of a feature's SQL statements live in its Postgres
  repository adapter; zero SQL is found in handlers or services.
- **SC-005**: The project's formatting, static analysis and automated test
  suite pass after the migration, with no additional skipped tests beyond the
  pre-existing database-dependent skips.
- **SC-006**: A new feature slice can be added by creating one folder under
  `internal/` and updating the top-level app wiring; during that probe, every
  file under `internal/shared/` remains byte-identical.
- **SC-007**: The register → login → dashboard → logout smoke flow completes in
  under 3 minutes and behaves identically to before the migration.
- **SC-008**: Zero duplicated database or HTTP helper implementations remain
  across feature packages — each helper exists once in `internal/shared/`.

## Assumptions

- **Scope**: All existing user-facing features (`auth`, `home`/dashboard) and
  the current shared packages are in scope. No new user-facing functionality is
  added, and no public JSON REST API is introduced.
- **Target layout**: The suggested structure is adopted as the reference;
  file names follow the suggestion (`domain.go`, `ports.go`, `service.go`,
  `http_handler.go`, and `postgres_repository.go` only for data-owning
  features) and feature-owned templates stay inside their feature folder.
- **Shared package mapping**: `internal/db` moves to
  `internal/shared/database`, `internal/web` moves to
  `internal/shared/httputil`, and the remaining shared concerns move under
  `internal/shared/` as `config`, `model`, `password`, `session` and
  `templates` (shared base layout).
- **Session is infrastructure**: The Postgres-backed session package is treated
  as shared infrastructure rather than a user-facing feature slice, because it
  serves authentication middleware across features.
- **Behavior preservation**: Routes, URLs, page templates, role rules and
  cookie/CSRF behavior remain identical; this is a pure structural migration.
- **Incremental delivery**: The migration is performed in small committed
  steps, each keeping the build and test suite green, so the refactor can be
  reviewed and merged incrementally.
- **Generated artifacts**: Generated `*_templ.go` files and compiled static
  assets remain committed so the app builds without the templ/tailwind
  toolchains installed.
