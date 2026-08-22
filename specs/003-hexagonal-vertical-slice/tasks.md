---
description: "Task list for feature implementation"
---

# Tasks: Hexagonal Architecture & Vertical Slice Migration

**Input**: Design documents from `specs/003-hexagonal-vertical-slice/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Tests**: Included — the user requested TDD for this migration, and the plan
defines an architecture boundary test plus a `home` service test.

**Organization**: Tasks are grouped by user story so each story can be
implemented and tested independently.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- Go module root is the repository root.
- Feature slices live under `internal/<feature>/`.
- Shared infrastructure lives under `internal/shared/`.
- Spec artifacts live under `specs/003-hexagonal-vertical-slice/`.

---

## Phase 1: Setup (Baseline + Architecture Test)

**Purpose**: Record the green baseline and write the failing architecture test
before any code moves.

- [x] T001 Run `go test ./...` and `gofmt -l .` from the repository root, record the green baseline, and commit it before making changes
- [x] T002 Create the architecture test `internal/app/arch_test.go` using only the standard library (`go/parser`), encoding the boundary rules from `specs/003-hexagonal-vertical-slice/contracts/boundaries.md`: features must not import other features or legacy shared paths; expected slice files must exist (`auth`: `domain.go`, `ports.go`, `service.go`, `http_handler.go`, `postgres_repository.go`, `module.go`; `home`: `domain.go`, `ports.go`, `service.go`, `http_handler.go`, `module.go`); `service.go` and `http_handler.go` must not import `database/sql` or `github.com/jackc/pgx`, and `postgres_repository.go` must be the only slice file allowed to import those packages
- [x] T003 Run `go test ./internal/app -run Architecture` and confirm the architecture test FAILS against the current layout (expected red)

---

## Phase 2: Foundational (Move Shared Packages)

**Purpose**: Move all shared infrastructure under `internal/shared/` before any
feature is restructured. These tasks are sequential because they update common
importers and the build must stay compilable after each move.

**⚠️ CRITICAL**: No user story work can begin until this phase is complete.

- [x] T004 Move `internal/config/config.go` and `internal/config/config_test.go` to `internal/shared/config/`; update import paths in `cmd/server/main.go` and `internal/app/app.go`; run `go test ./...` (only the architecture test may fail)
- [x] T005 Move `internal/model/user.go` to `internal/shared/model/user.go`; update every importer found by `rg "internal/model"` (including `internal/auth/*`, `internal/web/*`, `internal/home/templates/home_templ.go`, `internal/web/templates/base_templ.go`, and test files); run `go test ./...` (only the architecture test may fail)
- [x] T006 Move `internal/password/password.go` and `internal/password/password_test.go` to `internal/shared/password/`; update import paths in `internal/auth/service.go`, `internal/auth/module.go`, `internal/auth/service_test.go` and `internal/app/app.go`; run `go test ./...` (only the architecture test may fail)
- [x] T007 Move `internal/session/session.go` and `internal/session/session_test.go` to `internal/shared/session/`; update import paths in `internal/web/context.go`, `internal/web/csrf.go`, `internal/web/middleware.go`, `internal/auth/handler.go`, `internal/auth/module.go`, `internal/auth/handler_test.go`, `internal/home/home.go` and `internal/app/app.go`; run `go test ./...` (only the architecture test may fail)
- [x] T008 Move `internal/db/db.go` and `internal/db/migrate.go` to `internal/shared/database/`; update import paths in `cmd/server/main.go`, `internal/auth/store_test.go` and `internal/session/session_test.go`; run `go test ./...` (only the architecture test may fail)
- [x] T009 Move `internal/web/render.go`, `internal/web/cookie.go`, `internal/web/csrf.go`, `internal/web/context.go` and `internal/web/middleware.go` to `internal/shared/httputil/`; move `internal/web/templates/base.templ` and `internal/web/templates/base_templ.go` to `internal/shared/templates/`; rename package `web` to `httputil` and update all `web.` references plus import paths across `internal/auth/*`, `internal/home/*`, `internal/app/*`, test files and generated templ files; run `go test ./...` (only the architecture test may fail)

**Checkpoint**: All shared packages now live under `internal/shared/`; every
package except the architecture test still passes. User story work can begin.

---

## Phase 3: User Story 1 - Self-Contained Feature Slices (Priority: P1) 🎯 MVP

**Goal**: `auth` becomes a complete hexagonal slice with `domain.go`,
`ports.go`, `service.go`, `http_handler.go`, `postgres_repository.go` and
`module.go`; the slice compiles and its tests pass.

**Independent Test**: List `internal/auth/` and confirm the five slice files
plus `module.go` exist; run `go test ./internal/auth` and confirm the auth
tests pass without a database.

### Tests for User Story 1

The architecture test from T002 is the test for this story; no new test files
are needed here.

- [x] T010 [US1] Run `go test ./internal/app -run Architecture` and confirm the remaining failures are only the missing slice files in `internal/auth/` and `internal/home/`

### Implementation for User Story 1

- [x] T011 [P] [US1] Create `internal/auth/domain.go` by moving `ErrNotFound`, `ErrEmailTaken`, `ErrInvalidCredentials` and `ValidationError` from `internal/auth/errors.go`; delete `errors.go`
- [x] T012 [P] [US1] Create `internal/auth/ports.go` with the `UserStore` interface (`CreateUser`, `GetUserByEmail`, `GetUserByID`) moved from `internal/auth/service.go`; remove the interface from `service.go`
- [x] T013 [P] [US1] Create `internal/auth/http_handler.go` by moving every handler method from `internal/auth/handler.go`; delete `handler.go`
- [x] T014 [P] [US1] Create `internal/auth/postgres_repository.go` with `PostgresUserRepository` and `NewPostgresUserRepository` (renamed from `Store`/`NewStore` in `internal/auth/store.go`); delete `store.go` and update `internal/auth/store_test.go` to use `NewPostgresUserRepository`
- [x] T015 [US1] Update `internal/app/app.go` to construct `auth.NewPostgresUserRepository(pool)` and pass it to `auth.New` and `home.New`; run `go test ./...` and confirm only the `home` structure failures remain

**Checkpoint**: `auth` is a self-contained slice; `go test ./internal/auth`
passes and the app still compiles.

---

## Phase 4: User Story 2 - Business Logic Isolated Behind Ports (Priority: P1)

**Goal**: `home` becomes a self-contained slice with its own `domain.go`,
`ports.go`, `service.go` and `http_handler.go`, and the landing redirect rule
is unit-tested through the new service.

**Independent Test**: Run `go test ./internal/home` and confirm the home
service test passes without a database or HTTP server; run the architecture
test and confirm it now passes for both `auth` and `home`.

### Tests for User Story 2 (TDD — write first, watch it fail)

- [x] T016 [US2] Write `internal/home/service_test.go` covering `(*Service).LandingTarget` (`LandingTarget(true)` returns `/dashboard`, `LandingTarget(false)` returns an empty target); run `go test ./internal/home` and confirm it fails because the service does not exist yet

### Implementation for User Story 2

- [x] T017 [P] [US2] Create `internal/home/domain.go` defining `type LandingTarget string` with constants `LandingTargetNone` (empty string) and `LandingTargetDashboard` (`/dashboard`)
- [x] T018 [P] [US2] Create `internal/home/ports.go` with the `UserReader` interface (`GetUserByID(ctx, id) (*model.User, error)`)
- [x] T019 [P] [US2] Create `internal/home/service.go` with `type Service struct{}` and `func (s *Service) LandingTarget(userPresent bool) string` returning `LandingTargetDashboard` when the user is logged in and `LandingTargetNone` otherwise
- [x] T020 [US2] Create `internal/home/http_handler.go` with `Home`, `Dashboard` and `Hello` moved from `internal/home/home.go`; create `internal/home/module.go` with `Deps`, `New` and `RegisterRoutes`; delete `internal/home/home.go`
- [x] T021 [US2] Update `internal/app/app.go` so `home.New` receives `home.UserReader` (the same auth user repository) and the shared session store; run `go test ./...` and confirm the architecture test and all behavioral tests pass

**Checkpoint**: Both feature slices are self-contained and their services are
testable without infrastructure; the architecture test is fully green.

---

## Phase 5: User Story 3 - Shared Infrastructure Reused, Behavior Unchanged (Priority: P2)

**Goal**: Prove the migration is a pure structural refactor: the app builds,
the quality gate passes, generated artifacts are correct, and the user-facing
flow is unchanged.

**Independent Test**: Follow `specs/003-hexagonal-vertical-slice/quickstart.md`
from start to finish — layout check, boundary test, quality gate and the
register → login → dashboard → logout smoke flow.

### Implementation for User Story 3

- [x] T022 [US3] Verify `cmd/server/main.go` imports `internal/shared/config` and `internal/shared/database`; run `go build ./...` from the repository root
- [x] T023 [US3] Run `gofmt -w .` and `go mod tidy` from the repository root; review the diff and commit only formatting/import changes
- [x] T024 [US3] Run `make templ` (or `templ generate`) from the repository root and confirm the regenerated `*_templ.go` files reference `internal/shared/templates`; commit the generated files
- [x] T025 [US3] Run `make check` from the repository root and fix any format, vet or test failures
- [x] T026 [US3] Run the structural validation from `specs/003-hexagonal-vertical-slice/quickstart.md`: `make check`, `go test ./internal/app -run Architecture -v`, and `find internal -maxdepth 2 -type d | sort`; then prove SC-006 by creating a temporary throwaway slice under `internal/`, wiring it in `internal/app/app.go`, verifying `git diff -- internal/shared` is empty, and reverting the probe
- [x] T027 [US3] Run the database smoke test from `specs/003-hexagonal-vertical-slice/quickstart.md`: `make db-up`, `make run`, then exercise register → login → dashboard → logout and confirm identical behavior
- [x] T028 [US3] Verify no duplicated helpers remain: `rg -n "func (Render|SetSessionCookie|ReadSessionCookie|ClearSessionCookie|ValidCSRF)" internal` returns definitions only under `internal/shared/httputil`

**Checkpoint**: The migrated app passes the full quality gate and the smoke
flow behaves exactly as before the migration.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Final documentation and cleanup after all stories are complete.

- [x] T029 [P] Update `README.md` and `REASONIX.md` to document the new `internal/shared/` layout and the per-feature slice file conventions (`domain.go`, `ports.go`, `service.go`, `http_handler.go`, `postgres_repository.go` for data-owning features)
- [x] T030 Run `make check`, `git status --short` and `git diff --stat` from the repository root; confirm only intended migration changes remain and commit the final state

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies — start immediately.
- **Foundational (Phase 2)**: Depends on Phase 1; BLOCKS all user stories.
- **User Story 1 (Phase 3)**: Depends on Phase 2.
- **User Story 2 (Phase 4)**: Depends on Phase 3 (the auth repository created
  in US1 is reused as the home `UserReader` implementation).
- **User Story 3 (Phase 5)**: Depends on Phases 3 and 4.
- **Polish (Phase 6)**: Depends on Phase 5.

### User Story Dependencies

- **User Story 1 (P1)**: Starts after Phase 2; no dependency on US2/US3.
- **User Story 2 (P1)**: Starts after US1; reuses `auth.PostgresUserRepository`
  through `home.UserReader` without `home` importing `auth`.
- **User Story 3 (P2)**: Starts after US1 and US2; validates the whole
  migration.

### Within Each User Story

- Tests (TDD) are written and fail before implementation (T003 red, T016 red).
- Domain/ports/service files are created before handlers are wired.
- The composition root (`internal/app/app.go`) is updated after each slice.
- Each phase ends with a green `go test ./...` checkpoint (except the known
  architecture-test red state during Phase 2).

### Parallel Opportunities

- Phase 1: T001 is a prerequisite; T002/T003 are sequential.
- Phase 2: moves are sequential because they share importers.
- Phase 3: T011, T012, T013, T014 can run in parallel (different files).
- Phase 4: T017, T018, T019 can run in parallel (different files).
- Phase 6: T029 can run in parallel with nothing else; T030 is the final gate.

---

## Parallel Example: User Story 1

```bash
# Launch the auth slice file-split tasks together (different files):
Task: "Create internal/auth/domain.go by moving ... from internal/auth/errors.go"
Task: "Create internal/auth/ports.go with the UserStore interface ..."
Task: "Create internal/auth/http_handler.go by moving ... from internal/auth/handler.go"
Task: "Create internal/auth/postgres_repository.go with PostgresUserRepository ..."
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (baseline + red architecture test).
2. Complete Phase 2: Foundational (move all shared packages).
3. Complete Phase 3: User Story 1 (`auth` slice).
4. **STOP and VALIDATE**: `go test ./internal/auth` passes; `auth` has the
   full slice layout; the app compiles.
5. Commit the MVP.

### Incremental Delivery

1. Setup + Foundational → shared infrastructure relocated.
2. Add User Story 1 (`auth` slice) → validate → commit.
3. Add User Story 2 (`home` slice) → architecture test green → validate →
   commit.
4. Add User Story 3 (full quality gate + smoke) → validate → commit.
5. Polish docs and final review.

### Solo-Developer Strategy

- Work strictly in task order T001 → T030.
- Commit after each task or logical group.
- After T009, every non-architecture test must be green.
- After T021, the architecture test becomes green and stays green.

---

## Notes

- [P] tasks touch different files and can run in parallel.
- [US1]/[US2]/[US3] labels map tasks to spec user stories for traceability.
- The architecture test is expected to be RED from T003 until T021; all other
  tests must stay green from T004 onward.
- Commit after each task or logical group; use Conventional Commits.
- Stop at any checkpoint to validate the story independently.
- Avoid: moving two packages in one commit, editing generated `*_templ.go`
  files by hand (regenerate them), and introducing any new dependency.
