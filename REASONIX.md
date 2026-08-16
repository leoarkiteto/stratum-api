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

A **modular monolith**: features are vertical slices. Each feature module owns
its handlers, service, store and templates. Shared infrastructure lives in
dedicated packages.

```
backend/
├── cmd/server/            # entrypoint: config → DB → migrate → app
├── internal/
│   ├── app/               # assembly: module wiring, shared middleware, static
│   ├── auth/              # auth feature (register/login/logout) + templates
│   ├── home/              # landing + dashboard feature + templates
│   ├── session/           # Postgres-backed sessions, CSRF
│   ├── web/               # shared web helpers: render, cookies, auth middleware, base layout
│   ├── config/            # env-only config, fail fast
│   ├── db/                # pgx pool + versioned migration runner
│   └── model/             # shared domain types (User, Role)
├── assets/input.css       # TailwindCSS source (compiled to static/css/app.css)
├── static/                # vendored htmx.min.js + compiled app.css
├── migrations/            # versioned SQL pairs: NNNN_name.{up,down}.sql
├── docker-compose.yaml    # local Postgres 16
├── Makefile
└── .env.example
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
  its HTTP handlers, service, store and `.templ` templates.
- Shared, cross-feature concerns live in `internal/web` (rendering, cookies,
  CSRF, auth middleware) and `internal/app` (wiring, request logging, recovery).
- Keep handlers thin: parse/validate the request → one service call → render a
  Templ component or redirect. No business rules in handlers.
- Services hold business logic and depend on store interfaces, never HTTP or SQL.
- Stores use `database/sql` + plain SQL and return domain errors
  (`auth.ErrNotFound`, `auth.ErrEmailTaken`) that handlers map to messages.

### Server-rendered HTML + HTMX
- Pages are Templ components; the base layout (`internal/web/templates/base.templ`)
  provides the `<html>` shell, navbar, and the Tailwind CSS + HTMX includes.
- Forms post to the same path and re-render themselves on error; success sets a
  cookie and redirects (or returns an `HX-Redirect` header for htmx requests).
- Detect htmx requests with `web.IsHTMX` and return partial vs full pages.

### Auth & sessions
- Server-side sessions in Postgres; the browser holds a random token in an
  httpOnly, SameSite=Lax cookie. Tokens are stored as SHA-256 hashes.
- Session tokens rotate on login (`BindUser`) to prevent fixation; logout
  deletes the row and clears the cookie.
- Every state-changing form embeds a CSRF token bound to the session and
  validates it (`web.ValidCSRF`).
- RBAC roles `syndic` / `owner` / `tenant`. Middleware puts the current user in
  the request context (`web.RequireAuth` protects; `web.LoadUser` is optional).

### Config
- Environment variables only (12-factor), parsed once in `internal/config`; fail
  fast on missing required variables. No config files.

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

Adding a new feature module: create `internal/<feature>/` with
`module.go` (wiring + `RegisterRoutes`), `handler.go`, `service.go`, `store.go`,
a `templates/` dir, wire it in `internal/app/app.go`, and add a migration if it
owns data.
