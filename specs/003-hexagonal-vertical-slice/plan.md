# Implementation Plan: Hexagonal Architecture & Vertical Slice Migration

**Branch**: `003-hexagonal-vertical-slice` | **Date**: 2026-08-22 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/003-hexagonal-vertical-slice/spec.md`

**Note**: This plan is the output of `/speckit-plan`; it covers Phase 0 (research) and Phase 1 (design). Task breakdown is produced separately by `/speckit-tasks`.

## Summary

Migrate the existing GOTTH modular monolith into self-contained hexagonal vertical slices. Each user-facing feature (`auth`, `home`) owns its domain, ports, service, HTTP handler and (when it persists data) Postgres repository under `internal/<feature>/`; shared infrastructure moves under `internal/shared/` (`database/`, `httputil/`, `config/`, `model/`, `password/`, `session/`, `templates/`). The migration is a pure structural refactor: routes, pages, roles, sessions and CSRF behavior stay identical. Delivery is incremental, every commit keeps `go test ./...` green, and TDD is used in two ways: existing tests act as characterization tests that must stay green, and new architecture boundary tests are written first and must fail against the current layout, then pass as the migration completes.

## Technical Context

**Language/Version**: Go 1.26.5 (`go.mod`)

**Primary Dependencies**: stdlib `net/http` + `ServeMux`; `github.com/a-h/templ` v0.3.1020; `github.com/jackc/pgx/v5` v5.10.0 (stdlib driver); `golang.org/x/crypto` v0.54.0 (Argon2id). No web framework, no ORM.

**Storage**: PostgreSQL 16 via `database/sql` + pgx stdlib driver; versioned migrations `migrations/0001_create_users.{up,down}.sql` and `0002_create_sessions.{up,down}.sql`.

**Testing**: `go test ./...` (stdlib `testing`), `go vet ./...`, `gofmt`; database-dependent store/session integration tests skip unless `TEST_DATABASE_URL` is set. TDD strategy: characterization tests (existing) + new boundary tests (written first).

**Target Platform**: Linux server, single deployable binary (`cmd/server`); local development on macOS with Postgres in `docker-compose.yaml`.

**Project Type**: server-rendered web application (modular monolith).

**Performance Goals**: Unchanged from today — no new runtime work; page and flow latencies must remain at current levels.

**Constraints**: No new dependencies; no JSON REST API and no SPA/separate frontend; env-only configuration; generated `*_templ.go` files and compiled CSS stay committed; every committed step must build and pass tests.

**Scale/Scope**: 2 feature slices (`auth`, `home`), 6 current shared packages to relocate (`config`, `db`, `model`, `password`, `session`, `web`), ~25 Go source files, 2 database tables.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Requirement | Status | How this plan satisfies it |
|-----------|-------------|--------|----------------------------|
| I. GOTTH Stack — Server-Rendered Modular Monolith (Vertical Slices) | One server-rendered deployable; features are self-contained vertical slices | PASS | Target layout keeps `internal/app` as the only composition root and gives each feature its own domain/ports/service/adapters; no JSON API or SPA is introduced. |
| II. Standard Library First | stdlib first; no frameworks/ORMs; new deps must be justified | PASS | The migration adds zero dependencies and moves existing stdlib/approved-library code only. |
| III. CSS First over JavaScript | Tailwind + HTMX only; no JS frameworks | PASS | Static assets, `.templ` files and HTMX usage are unchanged; generated files move with their packages. |
| IV. Cookie-Based Server-Side Sessions | Postgres sessions, hashed tokens, rotation on login, CSRF on state-changing forms | PASS | `session` moves to `internal/shared/session` with its API and tests intact; handlers and middleware keep the same behavior. |
| V. Hexagonal Architecture & Vertical Slices (Ports & Adapters) | Services depend only on ports; thin handlers; stores return domain errors | PASS | This is the core of the plan: each feature gets `ports.go`; services depend only on those ports; HTTP handlers and Postgres repositories become adapters. |
| VI. RBAC — Three Access Profiles | `syndic`/`owner`/`tenant`, server-side enforcement | PASS | Roles, `model.Role`, authorization middleware and role tests are preserved; role behavior is not modified. |
| Technology & Infrastructure Constraints | Go 1.26+, Postgres 16, env config, committed artifacts, Makefile workflow | PASS | `cmd/server` startup order (config → DB → migrate → app) is unchanged; only import paths change. |
| Development Workflow & Quality Gates | `make check` green; tests next to code; RBAC handler tests; Conventional Commits | PASS | Each step keeps the test suite green; boundary tests are added next to features; commits follow Conventional Commits. |

**Gate result**: PASS — no violations requiring justification.

## Project Structure

### Documentation (this feature)

```text
specs/003-hexagonal-vertical-slice/
├── plan.md              # This file (/speckit-plan command output)
├── research.md          # Phase 0 output (/speckit-plan command)
├── data-model.md        # Phase 1 output (/speckit-plan command)
├── quickstart.md        # Phase 1 output (/speckit-plan command)
├── contracts/           # Phase 1 output (/speckit-plan command)
│   └── boundaries.md    # Ports and shared-package contracts
└── tasks.md             # Phase 2 output (/speckit-tasks command - NOT created by /speckit-plan)
```

### Source Code (repository root)

Target layout after the migration:

```text
cmd/
└── server/
    └── main.go                      # config → DB → migrate → app (import paths updated)

internal/
├── app/                             # composition root (kept, not a feature)
│   ├── app.go                       # wires features + shared adapters
│   └── middleware.go                # logging, recovery, security headers
├── auth/                            # feature slice
│   ├── domain.go                    # domain errors + validation types (from errors.go)
│   ├── ports.go                     # UserStore, SessionStore interfaces
│   ├── service.go                   # Register / Login / GetUserByID business logic
│   ├── http_handler.go              # thin HTTP adapter (from handler.go)
│   ├── postgres_repository.go       # user persistence (from store.go)
│   ├── module.go                    # Deps, New, RegisterRoutes
│   ├── *_test.go                    # tests live next to the feature
│   └── templates/                   # feature-owned Templ files + generated code
├── home/                            # feature slice
│   ├── domain.go                    # home view context types
│   ├── ports.go                     # UserReader, SessionReader interfaces
│   ├── service.go                   # home/dashboard view logic (kept minimal)
│   ├── http_handler.go              # Home / Dashboard / Hello handlers
│   ├── module.go                    # Deps, New, RegisterRoutes
│   ├── *_test.go
│   └── templates/                   # feature-owned Templ files + generated code
└── shared/                          # shared infrastructure (no feature owns it)
    ├── config/                      # from internal/config
    ├── database/                    # from internal/db (Open + Migrate)
    ├── httputil/                    # from internal/web: render, cookies, CSRF,
    │                                #   middleware, context, htmx helpers
    ├── model/                       # shared User + Role types
    ├── password/                    # Argon2id hasher
    ├── session/                     # Postgres-backed session store + contracts
    └── templates/                   # shared base layout (from internal/web/templates)

migrations/                          # unchanged
static/                              # unchanged (vendored htmx + compiled css)
assets/                              # unchanged
```

**Structure Decision**: Adopt the suggested hexagonal vertical-slice layout with two adjustments: (1) `module.go` stays in each feature for dependency wiring and route registration so the slice remains self-describing; (2) `postgres_repository.go` is required only for features that own persistence — `auth` has it, `home` does not (it reads the user/session via shared ports and owns no data today). Shared concerns currently spread across `internal/{config,db,model,password,session,web}` move under `internal/shared/` with `db` renamed to `database` and `web` renamed to `httputil`; the shared base layout moves to `internal/shared/templates`. `internal/app` remains the composition root and is the only package allowed to import every feature.

## Complexity Tracking

> Filled only if Constitution Check has violations that must be justified.

None — the Constitution Check passes without violations.
