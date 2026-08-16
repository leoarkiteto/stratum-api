# Stratum

**Smart condominium management platform** for residents and administrators —
a server-rendered **modular monolith** built on the **GOTTH stack**.

## Tech stack

- **Go 1.26+** — `net/http` + `ServeMux` (stdlib), no web frameworks
- **Templ** — type-safe server-side HTML templates (`.templ` → generated Go)
- **HTMX** — progressive-enhancement interactivity (vendored static asset)
- **TailwindCSS 4** — utility CSS, compiled with the standalone CLI (no Node)
- **PostgreSQL 16** — `database/sql` + the `pgx` driver, no ORM
- **Server-side sessions** — Postgres-backed, httpOnly cookies, CSRF-protected

## Features (current)

The auth domain is implemented end-to-end: registration, login, logout and an
authenticated dashboard, all rendered server-side with HTMX form handling.

- Roles: `syndic` / `owner` / `tenant` (self-service is `owner`/`tenant`).
- Passwords hashed with Argon2id + an HMAC-SHA256 pepper (see
  `internal/password`).
- Sessions stored in Postgres, token rotated on login (fixation-safe), CSRF
  tokens bound to the session.

## Getting started

Prerequisites: Go 1.26+, Docker (for local Postgres), and
[`templ`](https://templ.guide) (`go install github.com/a-h/templ/cmd/templ@latest`).

```bash
# 1. Install local build tools (standalone tailwindcss CLI; templ assumed installed)
make deps

# 2. Start Postgres 16 (matches .env.example)
make db-up

# 3. Configure the environment
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/stratum?sslmode=disable"
export PASSWORD_PEPPER="$(openssl rand -hex 32)"   # min 32 chars, required
# optional: HTTP_ADDR, SESSION_TTL, COOKIE_SECURE, MIGRATIONS_DIR (see .env.example)

# 4. Run — generates templates, builds CSS, applies migrations, serves on :8080
make run
```

Open http://localhost:8080.

## Configuration

Environment variables only (12-factor). `DATABASE_URL` and `PASSWORD_PEPPER`
are required; the rest have defaults. See `.env.example`.

| Variable          | Required | Default     |
| ----------------- | -------- | ----------- |
| `DATABASE_URL`    | yes      | —           |
| `PASSWORD_PEPPER` | yes      | — (min 32)  |
| `HTTP_ADDR`       | no       | `:8080`     |
| `SESSION_TTL`     | no       | `24h`       |
| `COOKIE_SECURE`   | no       | `false`     |
| `MIGRATIONS_DIR`  | no       | `migrations` |

Set `COOKIE_SECURE=true` behind HTTPS in production. `PASSWORD_PEPPER` is the
HMAC-SHA256 key applied before Argon2id hashing — keep it secret and stable
(rotating it invalidates all stored password hashes).

## Makefile targets

Run `make` (or `make help`) to list them.

| Target      | Description                                        |
| ----------- | -------------------------------------------------- |
| `deps`      | Fetch the standalone tailwindcss CLI into `bin/`   |
| `templ`     | Generate Go code from `.templ` templates           |
| `css`       | Compile `assets/input.css` → `static/css/app.css`  |
| `build`     | `templ` + `css` + `go build ./...`                 |
| `run`       | `templ` + `css` + run (migrations auto-apply)      |
| `test`      | `go test ./...`                                    |
| `check`     | `fmt-check` + `vet` + `test`                       |
| `db-up`     | Start Postgres 16 via docker compose               |
| `db-down`   | Stop Postgres                                      |

## Project layout

A modular monolith: each feature is a vertical slice under `internal/`,
owning its handlers, service, store and templates.

```
backend/
├── cmd/server/            # entrypoint: config → DB → migrate → app
├── internal/
│   ├── app/               # assembly: module wiring, shared middleware, static
│   ├── auth/              # auth feature: register/login/logout + templates
│   ├── home/              # landing + dashboard feature + templates
│   ├── session/           # Postgres-backed server-side sessions + CSRF
│   ├── web/               # shared web helpers: render, cookies, auth middleware, base layout
│   ├── config/            # env-only config, fail fast
│   ├── db/                # pgx pool + versioned migration runner
│   └── model/             # shared domain types (User, Role)
├── assets/input.css       # TailwindCSS source
├── static/                # vendored htmx.min.js + compiled app.css
├── migrations/            # versioned SQL pairs: NNNN_name.{up,down}.sql
├── docker-compose.yaml    # local Postgres 16
├── Makefile
└── .env.example
```

## Database & migrations

Migrations are plain SQL pairs in `migrations/`, applied in ascending order and
tracked in `schema_migrations`. They apply automatically at startup. Each runs
in its own transaction.

## Testing

```bash
make test    # unit + handler tests (no DB needed)
```

Store/session integration tests run against a real Postgres when
`TEST_DATABASE_URL` is set (e.g. a disposable `stratum_test` database); they
skip otherwise.

## License

MIT — see [LICENSE.md](LICENSE.md).

---

Detailed conventions live in [REASONIX.md](REASONIX.md).
