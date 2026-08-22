<!--
# Sync Impact Report
- Version change: (unfilled template) → 1.0.0
- Modified principles: none (first adoption; no previously ratified principles)
- Added sections: Core Principles (I–V), Technology & Infrastructure
  Constraints, Development Workflow & Quality Gates, Governance
- Removed sections: none
- Follow-up TODOs: none
-->

# Stratum Constitution

## Core Principles

### I. GOTTH Stack — Server-Rendered Modular Monolith

The application MUST be a server-rendered modular monolith on the GOTTH stack:
Go (`net/http` + `ServeMux`), TailwindCSS, Templ and HTMX. Pages are rendered as
HTML on the server; there MUST be no JSON REST API and no separate frontend/SPA.
Features are vertical slices under `internal/`: each feature owns its handlers,
service, store and templates. Shared, cross-feature concerns live in
`internal/web` (rendering, cookies, CSRF, auth middleware) and `internal/app`
(wiring, request logging, recovery). One deployable, one rendering pipeline —
the simplest path from database to pixels.

### II. Standard Library First

Use the Go standard library wherever it has an answer: `net/http`,
`encoding/json`, `log/slog`, `database/sql`, `errors`, `testing`. Web frameworks
(Gin/Echo/chi) and ORMs (GORM) MUST NOT be added. Libraries are allowed only
where the stdlib has no answer: `pgx` (Postgres driver), `a-h/templ`
(templates), `golang.org/x/crypto` (password hashing). Nothing heavier — no
query builders, no validation frameworks. Every new dependency MUST be justified
against this list. Rationale: a small, stable dependency surface keeps the
monolith maintainable over years.

### III. CSS First over JavaScript

Styling MUST be done with TailwindCSS (utility classes), compiled with the
standalone CLI; no Node toolchain is required to build the app. Interactivity
MUST be progressive enhancement via HTMX (vendored static asset); JavaScript
frameworks MUST NOT be used. Pages MUST remain usable without JavaScript where
feasible: content and core flows must not depend on JS, and `noscript` behavior
falls back to standard form POST + redirect. Rationale: the server-rendered HTML
stays the source of truth and the client stays a dumb terminal.

### IV. Cookie-Based Server-Side Sessions

Authentication MUST use server-side sessions stored in Postgres; the browser
holds only a random opaque token in an httpOnly, SameSite=Lax cookie. Tokens
MUST be stored as SHA-256 hashes. Sessions MUST rotate the token on login
(fixation prevention); logout MUST delete the session row and clear the cookie.
Every state-changing form MUST embed and validate a CSRF token bound to the
session (`web.ValidCSRF`). Rationale: no session material in the browser, and
revocation is immediate and server-enforced.

### V. Hexagonal Architecture (Ports & Adapters)

Business logic MUST live in services that depend only on ports (Go interfaces)
— never on HTTP or SQL. Adapters implement those ports: stores (plain SQL via
`database/sql`) and HTTP handlers (transport). Handlers MUST be thin:
parse/validate the request → one service call → render a Templ component or
redirect; handlers contain no business rules. Stores return domain errors
(e.g. `auth.ErrNotFound`) that handlers map to user-facing messages. This
applies per feature module, composing cleanly with the modular monolith.
Rationale: the domain is testable without HTTP or a database, and adapters can
be swapped without touching business logic.

## Technology & Infrastructure Constraints

- **Runtime**: Go 1.26+; PostgreSQL 16 via `database/sql` + the `pgx` driver.
- **Migrations**: versioned SQL pairs in `migrations/` (`NNNN_name.{up,down}.sql`),
  applied ascending at startup and tracked in `schema_migrations`; rollback is
  manual via the `.down.sql` files. Table names are snake_case plural; foreign
  keys are explicit with meaningful names.
- **Configuration**: environment variables only (12-factor), parsed once in
  `internal/config`, failing fast on missing required variables. No config files.
- **Committed artifacts**: generated `*_templ.go` files, the compiled
  `static/css/app.css`, and the vendored `htmx.min.js` MUST be committed so the
  app builds without the templ/tailwind toolchains installed.
- **Local dev**: Postgres runs via `docker-compose.yaml`; the Makefile owns
  `deps`, `templ`, `css`, `build`, `run`, `test`, `check`, `db-up`, `db-down`.

## Development Workflow & Quality Gates

- **Quality gates**: `make check` (fmt-check + vet + test) MUST pass before a
  change is merged; after touching `.templ` files or `assets/input.css`, run
  `templ generate` and `make css` so generated artifacts stay current.
- **Tests**: tests live next to the code (`*_test.go`). Store/session
  integration tests run only when `TEST_DATABASE_URL` is set and skip otherwise;
  unit and handler tests MUST NOT require a database.
- **Commits**: Conventional Commits (`feat:`, `fix:`, `chore:`, ...). Logging is
  structured via `log/slog`.
- **Handler hygiene**: one service call per handler; no business rules, no SQL,
  and no framework imports in handlers (see Principle V).

## Governance

This constitution supersedes all other practices; where repository guidance
conflicts with it, the constitution wins. The current runtime guidance for
day-to-day development is `REASONIX.md`.

- **Amendment procedure**: propose a change, document its rationale and impact
  (added/removed/modified principles), then update this file and bump the
  version per the policy below. Amendments MUST be recorded in the Sync Impact
  Report comment at the top of this file.
- **Versioning policy** (semantic): MAJOR for backward-incompatible principle
  removals or redefinitions; MINOR for a new principle/section or materially
  expanded guidance; PATCH for clarifications, wording and non-semantic
  refinements.
- **Compliance review**: every PR MUST be checked against these principles;
  reviewers flag violations and block merge on any breach of Principles I–V.
  A principle may be waived only by an amendment that says so explicitly.

**Version**: 1.0.0 | **Ratified**: 2026-08-22 | **Last Amended**: 2026-08-22
