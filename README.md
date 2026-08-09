# Stratum Backend

REST API backend for the Stratum condo-management platform (voting, elections,
finances, tickets). JSON over HTTP, resource-oriented, JWT-authenticated.
Currently implements the auth domain; more aggregates will land as the platform
grows.

## Tech stack

- **Go 1.26+**, stdlib-first: `net/http` + `ServeMux` (method-aware routing), no web frameworks
- **PostgreSQL 16** via `database/sql` + the `pgx` driver, no ORMs
- **JWT** bearer auth (`golang-jwt/jwt/v5`), RBAC roles `syndic` / `owner` / `tenant`
- Structured logging with `log/slog`

## API

All endpoints live under `/api/v1`. Responses are JSON; property names are
camelCase; schemas are PascalCase. Errors use a consistent envelope:

```json
{"error": {"code": "invalid_input", "message": "..."}}
```

### Current endpoints

| Method | Path                    | Auth | Description                                    |
| ------ | ----------------------- | ---- | ---------------------------------------------- |
| POST   | `/api/v1/auth/register` | —    | Create an account → `201` `{token, user}`      |
| POST   | `/api/v1/auth/login`    | —    | Log in → `200` `{token, user}`                 |
| GET    | `/api/v1/auth/me`       | JWT  | Current user from token → `200` `user`         |

Notes:

- Registration accepts `role: "owner" | "tenant"` (`syndic` is assigned out of
  band); role defaults to `owner`.
- Credential endpoints are rate-limited per client IP: `10/min` login,
  `5/min` register.
- Send the token as `Authorization: Bearer <token>`.

## Getting started

Prerequisites: Go 1.26+, Docker (for local Postgres).

```bash
# 1. Start Postgres 16 (background; matches .env.example DATABASE_URL)
make db-up

# 2. Configure the environment — required vars come from the environment only
export DATABASE_URL="postgres://postgres:postgres@localhost:5432/stratum?sslmode=disable"
export JWT_SECRET="$(openssl rand -hex 32)"   # min 32 chars
# optional: HTTP_ADDR, JWT_TTL, JWT_ISSUER, MIGRATIONS_DIR (see .env.example)

# 3. Run — applies pending migrations at startup
make run
```

## Configuration

Environment variables only (12-factor), parsed once at startup; missing
required variables fail fast. See `.env.example` for the full list.

| Variable        | Required | Default             |
| --------------- | -------- | ------------------- |
| `DATABASE_URL`  | yes      | —                   |
| `JWT_SECRET`    | yes      | — (min 32 chars)    |
| `HTTP_ADDR`     | no       | `:8080`             |
| `JWT_TTL`       | no       | `24h`               |
| `JWT_ISSUER`    | no       | `stratum-backend`   |
| `MIGRATIONS_DIR`| no       | `migrations`        |

## Makefile targets

Run `make` (or `make help`) to list them.

| Target      | Description                                   |
| ----------- | --------------------------------------------- |
| `run`       | Run the API (migrations auto-apply on start)  |
| `build`     | Compile-check all packages (no artifacts)     |
| `test`      | Run all tests                                 |
| `vet`       | Run `go vet ./...`                            |
| `lint`      | Run `staticcheck ./...` (install it first)    |
| `fmt`       | `gofmt -w .`                                  |
| `fmt-check` | Fail if any file isn't formatted              |
| `check`     | `fmt-check` + `vet` + `test`                  |
| `tidy`      | `go mod tidy`                                 |
| `db-up`     | Start Postgres 16 via docker compose          |
| `db-down`   | Stop Postgres                                 |

## Project layout

```
backend/
├── cmd/server/          # entrypoint: config → DB → store → service → handler → server
├── internal/
│   ├── apierr/          # typed API errors mapped to the error envelope
│   ├── auth/            # JWT issuing/verification, password hashing, principals
│   ├── config/          # env-only config, fail fast on missing vars
│   ├── db/              # pgx pool + versioned migration runner
│   ├── handler/         # thin HTTP handlers (parse → one service call → JSON)
│   ├── httpx/           # JSON writer + error envelope
│   ├── middleware/      # JWT auth, rate limiting
│   ├── model/           # domain types
│   ├── server/          # ServeMux routes + wiring
│   ├── service/         # business logic (no HTTP, no SQL)
│   └── store/           # plain-SQL Postgres access (database/sql)
├── migrations/          # versioned SQL pairs: NNNN_name.{up,down}.sql
├── docker-compose.yaml  # local Postgres 16
├── Makefile
└── .env.example
```

## Database & migrations

- Migrations are plain SQL pairs in `migrations/`, applied in ascending order
  and tracked in the `schema_migrations` table. Each runs in its own
  transaction.
- **They apply automatically at server startup** — no separate CLI. To roll
  back, restore the matching `NNNN_name.down.sql` against your database.
- Store tests run against the real Postgres from `make db-up` (no mocks).

## Testing

```bash
make check   # fmt + vet + tests
make test    # just the tests
```

Handlers are tested against the real `ServeMux` with `httptest`; store tests
need a running Postgres.

## License

MIT — see [LICENSE.md](LICENSE.md).

---

Detailed conventions (layering, REST rules, tooling hygiene) live in
[REASONIX.md](REASONIX.md).
