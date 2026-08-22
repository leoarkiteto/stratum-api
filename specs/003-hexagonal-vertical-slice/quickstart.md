# Quickstart Validation: Hexagonal Architecture & Vertical Slice Migration

**Feature**: `003-hexagonal-vertical-slice`
**Date**: 2026-08-22

This guide validates that the migration is complete and correct. It is a
run/validation guide only — implementation steps belong in `tasks.md`.

## Prerequisites

- Go 1.26+ installed.
- Docker available (only for the Postgres smoke test).
- Environment variables from `.env.example`; required:
  - `DATABASE_URL` — Postgres connection string
  - `PASSWORD_PEPPER` — at least 32 random characters (generate with
    `openssl rand -hex 32`)
- Optional: `TEST_DATABASE_URL` to run database-dependent integration tests;
  when unset, those tests skip as they do today.

## 1. Structural validation (no database needed)

```bash
# Test, vet and format check in one gate
make check

# Boundary/architecture test only (red before migration, green after)
go test ./internal/app -run Architecture -v
```

Expected outcome:

- `make check` exits 0.
- The architecture test reports that `auth` and `home` import no legacy shared
  paths and no other feature, and that the expected slice files exist
  (`auth` has `postgres_repository.go`; `home` is exempt because it owns no
  persistence).

## 2. Verify the target layout

```bash
find internal -maxdepth 2 -type d | sort
```

Expected feature folders:

```text
internal/app
internal/auth
internal/home
internal/shared/config
internal/shared/database
internal/shared/httputil
internal/shared/model
internal/shared/password
internal/shared/session
internal/shared/templates
```

Expected slice files for `auth`:

```text
internal/auth/domain.go
internal/auth/ports.go
internal/auth/service.go
internal/auth/http_handler.go
internal/auth/postgres_repository.go
internal/auth/module.go
```

## 3. Database-backed smoke test

```bash
make db-up
make run        # starts the app on HTTP_ADDR (default :8080)
```

Then exercise the unchanged user journey:

1. Open `http://localhost:8080/` — landing page renders.
2. Go to `/register`, submit a valid `owner` account — you are redirected to
   `/dashboard`.
3. Log out (`POST /logout`) — you land back on `/`.
4. Log in with the same credentials — you reach `/dashboard` again.
5. Confirm `/dashboard/hello` returns the htmx partial and that every POST
   requires a valid `csrf` field (a missing/invalid token re-renders the form
   with a session-expired message).

Expected outcome: the flow behaves exactly as before the migration; the session
cookie stays `httpOnly`, `SameSite=Lax`; login rotates the session token; RBAC
role checks still run server-side.

## 4. Optional database integration tests

```bash
TEST_DATABASE_URL="$DATABASE_URL" go test ./...
```

Expected outcome: store/session integration tests that were previously skipped
now run and pass; no other test outcome changes.

## Acceptance Mapping

| Success criterion | Validation |
|-------------------|------------|
| SC-001 self-contained slices | architecture test + `find internal` layout above |
| SC-002 service tests without DB/HTTP | `go test ./internal/auth ./internal/home` passes without `TEST_DATABASE_URL` |
| SC-003 thin handlers / SC-004 SQL only in repositories | review of `http_handler.go` vs `postgres_repository.go` during PR |
| SC-005 quality gate | `make check` exits 0 |
| SC-006 new slice = one folder + wiring | add a probe folder and update `internal/app`; no shared changes needed |
| SC-007 smoke flow under 3 minutes | section 3 above |
| SC-008 no duplicated helpers | `rg -n "func (Render|SetSessionCookie|ValidCSRF|ReadSessionCookie)" internal` shows one definition under `internal/shared` |
