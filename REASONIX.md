# Stratum Backend

The Stratum application is a **server-rendered modular monolith** built on the
**GOTTH stack**: Go, TailwindCSS, Templ and HTMX. It renders HTML on the server
and adds interactivity with HTMX — there is no JSON REST API and no separate
frontend.

**Stack:** Go 1.26+ (`net/http` + `ServeMux`), Templ (server-side templates),
HTMX (vendored static asset), TailwindCSS 4 (standalone CLI), Postgres 16
(`database/sql` + `pgx`), server-side sessions.

---

## Folder structure

A **modular monolith**: features are hexagonal vertical slices. Each feature
owns its domain, ports, service, HTTP handlers, persistence adapter and
templates. Shared infrastructure lives under `internal/shared/`.

```
internal/
├── app/                    # composition root: module wiring, logging, recovery
├── auth/                   # auth feature slice (register/login/logout)
│   ├── domain.go           #   domain types + errors
│   ├── ports.go            #   interfaces the service depends on
│   ├── service.go          #   business logic
│   ├── http_handler.go     #   thin HTTP adapter
│   ├── postgres_repository.go  #   Postgres persistence adapter
│   ├── module.go           #   wiring + RegisterRoutes
│   └── templates/          #   feature-owned .templ files
├── home/                   # home feature slice (landing + dashboard)
│   ├── domain.go
│   ├── ports.go
│   ├── service.go
│   ├── http_handler.go
│   ├── module.go
│   └── templates/
└── shared/                 # shared infrastructure (no feature owns it)
    ├── config/             # env-only config, fail fast
    ├── database/           # pgx pool + versioned migration runner
    ├── httputil/           # render, cookies, CSRF, auth middleware, context
    ├── model/              # shared domain types (User, Role)
    ├── password/           # Argon2id + pepper hashing
    ├── session/            # Postgres-backed sessions, CSRF
    └── templates/          # shared base layout
```

**Intentionally excluded** (do not add back): `pkg/`, `api/`, `web/`,
`deployments/`, `scripts/`, `test/`, `configs/`, `docs/` (OpenAPI). There is no
public library API, no separate web assets repo, no external test utilities,
and no API spec to generate. Tests live next to the code (`*_test.go`).

---

## Rules

### Standard library first
- Use stdlib: `net/http`, `encoding/json`, `log/slog`, `database/sql`, `errors`, `testing`.
- No web frameworks (no Gin/Echo/chi) and no ORMs (no GORM). `ServeMux` handles
  method-aware routes (`GET /login`, `GET /dashboard`).
- Allowed libraries where stdlib has no answer: `pgx` (driver), `a-h/templ`
  (templates), `golang.org/x/crypto/bcrypt` (passwords). Nothing heavier — no
  query builders, no validation frameworks.

### Modular monolith (feature slices)
- Organize by feature under `internal/` (e.g. `auth`, `home`). A feature owns
  its `domain.go`, `ports.go`, `service.go`, `http_handler.go` and `.templ`
  templates; `postgres_repository.go` is added when the feature persists data.
- Shared, cross-feature concerns live under `internal/shared/` (`httputil`,
  `session`, `database`, `config`, `model`, `password`, `templates`) and
  `internal/app` (wiring, request logging, recovery). Features must never
  import another feature's internals.
- Keep handlers thin: parse/validate the request → one service call → render a
  Templ component or redirect. No business rules in handlers.
- Services hold business logic and depend only on ports (interfaces), never
  HTTP or SQL.
- Repositories use `database/sql` + plain SQL and return domain errors
  (`auth.ErrNotFound`, `auth.ErrEmailTaken`) that handlers map to messages.
- Boundary rules (no feature-to-feature imports, SQL only in repositories) are
  enforced by `internal/app/arch_test.go`.

### Server-rendered HTML + HTMX
- Pages are Templ components; the base layout (`internal/shared/templates/base.templ`)
  provides the `<html>` shell, navbar, and the Tailwind CSS + HTMX includes.
- Forms post to the same path and re-render themselves on error; success sets a
  cookie and redirects (or returns an `HX-Redirect` header for htmx requests).
- Detect htmx requests with `httputil.IsHTMX` and return partial vs full pages.

### Auth & sessions
- Server-side sessions in Postgres; the browser holds a random token in an
  httpOnly, SameSite=Lax cookie. Tokens are stored as SHA-256 hashes.
- Session tokens rotate on login (`BindUser`) to prevent fixation; logout
  deletes the row and clears the cookie.
- Every state-changing form embeds a CSRF token bound to the session and
  validates it (`httputil.ValidCSRF`).
- RBAC roles `syndic` / `owner` / `tenant`. Middleware puts the current user in
  the request context (`httputil.RequireAuth` protects; `httputil.LoadUser` is
  optional).

### Config
- Environment variables only (12-factor), parsed once in
  `internal/shared/config`; fail fast on missing required variables. No config
  files.

### Database
- Versioned SQL migrations in `migrations/` (up + down pairs), applied ascending
  at startup and tracked in `schema_migrations`. Rollback is manual via the
  `.down.sql` files.
- Table names: snake_case plural. Foreign keys: explicit, with meaningful names.

### Tooling & hygiene
- `gofmt -w .`, `go vet ./...`, `templ generate`, `make css`.
- Generated `*_templ.go` files and the compiled `static/css/app.css` are
  committed so the app builds without the templ/tailwind tools present.
- Conventional Commits (`feat:`, `fix:`, `chore:`, ...).
- Structured logging via `log/slog`.

---

## Commands

```bash
make deps        # fetch standalone tailwindcss CLI into bin/
make templ       # templ generate
make css         # compile TailwindCSS
make build       # templ + css + go build ./...
make run         # templ + css + go run ./cmd/server
make test        # go test ./...
make check       # fmt-check + vet + test
make db-up       # start Postgres (docker compose)
```

Adding a new feature module: create `internal/<feature>/` with `domain.go`,
`ports.go`, `service.go`, `http_handler.go`, `module.go` (wiring +
`RegisterRoutes`), a `templates/` dir, and a `postgres_repository.go` when the
feature persists data. Wire it in `internal/app/app.go`, and add a migration if
it owns data. The architecture test (`internal/app/arch_test.go`) must stay
green.
