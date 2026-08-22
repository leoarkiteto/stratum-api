# Phase 0 Research: Hexagonal Architecture & Vertical Slice Migration

**Feature**: `003-hexagonal-vertical-slice`
**Date**: 2026-08-22

## Research Questions

The Technical Context had no `NEEDS CLARIFICATION` markers. The open design
questions were resolved by reading the current code and applying hexagonal +
vertical-slice best practices for a small Go server-rendered monolith:

1. What is the safest migration order for moving packages without breaking the build?
2. How do we enforce "no feature reaches into another feature's internals" automatically?
3. Which ports does each feature own, and how do shared packages relate to them?
4. Where does the shared base layout live?
5. How should TDD be applied to a structural refactor?

## Decisions

### 1. Incremental leaf-first migration (strangler-fig, no big-bang)

**Decision**: Migrate one package at a time, starting with the leaf shared
packages that have no internal dependencies, then the packages that depend on
them, and finish with the composition root.

**Rationale**: Package moves in Go are import-path changes. Doing them
leaf-first means each commit changes one package plus the few files that import
it, and `go test ./...` stays green at every step.

**Alternatives considered**:
- Big-bang move of everything in one commit — faster in one step but produces
  a huge, unreviewable diff and a long red period.
- Feature-first migration (move `auth` before shared packages) — would force
  features to temporarily import legacy shared paths, creating churn and
  violating the target rules during the transition.

**Chosen order**:

1. Baseline: run `go test ./...` and record the green state.
2. Add the boundary/architecture test (see decision 2) and watch it fail.
3. `internal/config` → `internal/shared/config`
4. `internal/model` → `internal/shared/model`
5. `internal/password` → `internal/shared/password`
6. `internal/session` → `internal/shared/session`
7. `internal/db` → `internal/shared/database`
8. `internal/web` → `internal/shared/httputil`; `internal/web/templates` →
   `internal/shared/templates`
9. Split `internal/auth` into `domain.go`, `ports.go`, `service.go`,
   `http_handler.go`, `postgres_repository.go`, `module.go` (same package).
10. Split `internal/home` into `domain.go`, `ports.go`, `service.go`,
    `http_handler.go`, `module.go` (no repository; it owns no persistence).
11. Update `internal/app` wiring and `cmd/server` imports.
12. Run the full quality gate and the boundary test.

### 2. Architecture boundary test written first (TDD red → green)

**Decision**: Add a test (in `internal/app/arch_test.go`, using the standard
library's `go/parser` or `go/build`) that encodes the target dependency rules:

- A package under `internal/<feature>` (currently `auth`, `home`) MUST NOT
  import another `internal/<feature>` package.
- A feature MUST NOT import the legacy shared paths `internal/config`,
  `internal/db`, `internal/model`, `internal/password`, `internal/session`,
  `internal/web`.
- A feature MAY import `internal/shared/*` and its own subpackages (e.g. its
  `templates`).
- `internal/app` is the composition root and may import every feature and
  shared package.
- Each data-owning feature MUST contain the expected slice files
  (`domain.go`, `ports.go`, `service.go`, `http_handler.go`,
  `postgres_repository.go`); `home` is exempt from the repository file because
  it owns no persistence.

This test is expected to fail against the current layout (features still import
the legacy shared paths and the slice files do not exist) and to pass only when
the migration is complete.

**Rationale**: A refactor is exactly the place where TDD makes sense: the tests
pin the desired architecture before the code moves, so the migration has an
objective definition of done and regressions are caught mechanically instead of
by review.

**Alternatives considered**:
- `go-arch-lint` or another dependency — rejected: it would violate Principle II
  (standard library first) and adds a new tool for one check.
- Relying on code review only — rejected: does not satisfy the user's
  "avoid violation of boundaries/contracts" requirement with an executable
  check.

### 3. Consumer-driven ports with Go structural typing

**Decision**: Each feature owns the interfaces it needs in its `ports.go`
(consumer-driven contracts). Shared packages define their own public contracts
(`shared/session.Store`, `shared/httputil.UserReader`, etc.). Concrete adapters
live in the feature that owns the data, and `internal/app` wires them where
they are needed.

Examples:

- `auth/ports.go` owns `UserStore` (`CreateUser`, `GetUserByEmail`,
  `GetUserByID`) — the persistence contract for the auth service.
- `auth/postgres_repository.go` implements `auth.UserStore`; because Go
  interfaces are structural, the same concrete repository also satisfies
  `home.UserReader` and `shared/httputil.UserReader` without any adapter
  boilerplate.
- `shared/session.Store` remains the session contract used by `shared/httputil`
  middleware and by the `auth` handler (a shared infrastructure dependency, not
  a feature-to-feature coupling).

**Rationale**: This keeps services independent of concrete infrastructure and
keeps every dependency explicit at the composition root, while structural
typing avoids the boilerplate of explicit adapter wrappers in a small monolith.

**Alternatives considered**:
- One shared `UserStore` interface imported by every feature — simpler but
  centralizes contracts and makes each slice less self-contained.
- Explicit adapter wrapper types — more "textbook hexagonal" but pure
  boilerplate in Go when the method sets are identical.

### 4. Shared base layout moves to `internal/shared/templates`

**Decision**: The shared base layout (`internal/web/templates/base.templ` and
its generated `base_templ.go`) moves to `internal/shared/templates`. Feature
templates stay inside their feature packages and import the shared base layout
from there.

**Rationale**: `httputil` should stay focused on HTTP mechanics (rendering,
cookies, CSRF, middleware, context). A shared Templ layout is a template
concern, so it gets its own shared package; this also keeps the feature
template dependencies obvious.

**Alternatives considered**:
- Put the base layout inside `shared/httputil` — rejected: mixes transport
  helpers with presentation.
- Duplicate the base layout per feature — rejected: duplicates presentation
  and violates the "no duplicated shared helpers" requirement.

### 5. TDD applied as characterization tests + new seams

**Decision**: Use the existing test suite as characterization tests: they pin
current behavior (registration, login, session rotation, CSRF, config,
password) and MUST stay green through the whole refactor with only mechanical
import-path updates. Write new tests only for (a) the architecture boundary
rules (decision 2) and (b) any genuinely new logic introduced while creating
the `home` service; do not rewrite passing tests for behavior that is not
changing.

**Rationale**: The migration must not change user-visible behavior, so the
existing behavioral tests are the regression net. New logic is minimal, so
TDD effort goes into the executable architecture contract rather than into
re-specifying unchanged behavior.

**Alternatives considered**:
- Red-green-refactor every moved function — rejected: it would mean rewriting
  already-covered behavior tests with no added protection, slowing the
  migration without reducing risk.

## Out of Scope / Deferred

- Any change to routes, templates, RBAC rules, session lifetimes or CSRF
  behavior (pure structural move only).
- Introducing a `User` feature slice or shared user repository: user data
  remains owned by `auth`; other features consume it through their own ports.
- Adding the `Election` module (`specs/002-election-module`) — it is a separate
  feature and must be built on top of the migrated structure, not inside this
  refactor.
