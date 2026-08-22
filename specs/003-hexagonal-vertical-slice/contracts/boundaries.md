# Contracts & Boundaries: Hexagonal Architecture & Vertical Slice Migration

**Feature**: `003-hexagonal-vertical-slice`
**Date**: 2026-08-22

This document defines the contracts the migration must preserve and introduce.
"External" contracts are the server-rendered HTTP routes and form behavior
users already see; "internal" contracts are the Go ports and shared-package
APIs that enforce the hexagonal boundaries.

## External Contract (unchanged)

The migration MUST NOT change any of the following.

### Routes

| Method | Path | Auth | Behavior |
|--------|------|------|----------|
| GET | `/` | optional (`LoadUser`) | landing page; redirects logged-in users to `/dashboard` |
| GET | `/login` | none | renders login form with a fresh anonymous session + CSRF |
| POST | `/login` | none | validates CSRF, verifies credentials, rotates session token, redirects to `/dashboard` |
| GET | `/register` | none | renders register form with a fresh anonymous session + CSRF |
| POST | `/register` | none | validates CSRF, creates user (`owner`/`tenant`), auto-login, redirects to `/dashboard` |
| POST | `/logout` | none (best-effort) | deletes session, clears cookie, redirects to `/` |
| GET | `/dashboard` | required (`RequireAuth`) | renders authenticated dashboard |
| GET | `/dashboard/hello` | required (`RequireAuth`) | renders htmx partial |
| GET | `/static/` | none | serves vendored HTMX and compiled CSS |

### Form fields

- `POST /login`: `email`, `password`, `csrf`
- `POST /register`: `name`, `email`, `password`, `role`, `csrf`
- `POST /logout`: no body fields required (cookie identifies the session)

### Session & CSRF contract

- Cookie name: `stratum_session`; httpOnly, `SameSite=Lax`; `Secure` follows
  `COOKIE_SECURE`; `MaxAge` follows `SESSION_TTL`.
- Session token is stored only as a SHA-256 hash in Postgres.
- Login binds the session to the user and rotates the token (fixation
  prevention).
- Every state-changing form posts a `csrf` field that must match the session's
  CSRF token in constant time.
- htmx requests (`HX-Request: true`) receive `HX-Redirect` headers instead of
  `3xx` redirects; forms re-render themselves (partial for htmx, full page
  otherwise).

## Internal Feature Ports

Each feature owns the interfaces it depends on in its `ports.go`. Concrete
adapters are wired in `internal/app`. Go's structural typing lets one concrete
adapter satisfy several consumer ports without wrapper code.

### `auth` feature

`internal/auth/ports.go`:

```go
type UserStore interface {
    CreateUser(ctx context.Context, u *model.User) (int64, error)
    GetUserByEmail(ctx context.Context, email string) (*model.User, error)
    GetUserByID(ctx context.Context, id int64) (*model.User, error)
}
```

Error contract (kept in `internal/auth/domain.go`):

- `auth.ErrNotFound` — requested user does not exist
- `auth.ErrEmailTaken` — email already registered
- `auth.ErrInvalidCredentials` — unknown email or wrong password (same error for
  both, to avoid user enumeration)
- `auth.ValidationError{Message string}` — user-input validation failure

The `auth` handler additionally uses the shared session contract
(`shared/session.Store`) for CSRF/session binding, and `shared/httputil` for
rendering, cookies and htmx helpers. `auth` MUST NOT import `home`.

### `home` feature

`internal/home/ports.go`:

```go
type UserReader interface {
    GetUserByID(ctx context.Context, id int64) (*model.User, error)
}
```

`home` handlers obtain the current user from the request context populated by
`shared/httputil` middleware. `home` MUST NOT import `auth`; its `UserReader`
is satisfied by the same concrete user repository that `auth` owns, wired via
`internal/app`.

## Shared Package Contracts

Shared packages may be imported by any feature. They MUST NOT import any
feature package.

### `internal/shared/database` (from `internal/db`)

```go
func Open(dsn string) (*sql.DB, error)
func Migrate(ctx context.Context, db *sql.DB, dir string) error
```

### `internal/shared/httputil` (from `internal/web`)

Rendering / htmx:

```go
func Render(w http.ResponseWriter, r *http.Request, c templ.Component)
func IsHTMX(r *http.Request) bool
func HXRedirect(w http.ResponseWriter, url string)
```

Cookies:

```go
const SessionCookieName = "stratum_session"
func SetSessionCookie(w http.ResponseWriter, token string, ttl time.Duration, secure bool)
func ReadSessionCookie(r *http.Request) (string, bool)
func ClearSessionCookie(w http.ResponseWriter, secure bool)
```

CSRF & context:

```go
func ValidCSRF(r *http.Request, s *session.Session) bool
func WithUser(ctx context.Context, u *model.User) context.Context
func UserFrom(ctx context.Context) (*model.User, bool)
func WithSession(ctx context.Context, s *session.Session) context.Context
func SessionFrom(ctx context.Context) (*session.Session, bool)
```

Middleware (the `web.UserStore` contract is renamed to `UserReader`):

```go
type UserReader interface {
    GetUserByID(ctx context.Context, id int64) (*model.User, error)
}
func RequireAuth(sessions session.Store, users UserReader, secure bool) func(http.Handler) http.Handler
func LoadUser(sessions session.Store, users UserReader) func(http.Handler) http.Handler
```

### `internal/shared/session` (from `internal/session`)

```go
type Store interface {
    Create(ctx context.Context) (token, csrf string, err error)
    Get(ctx context.Context, token string) (*Session, error)
    BindUser(ctx context.Context, token string, userID int64) (newToken string, err error)
    Delete(ctx context.Context, token string) error
}
type Session struct { UserID int64; CSRF string }
func New(db *sql.DB, ttl time.Duration) *Manager
```

### `internal/shared/model` (from `internal/model`)

```go
type Role string
const ( RoleSyndic Role = "syndic"; RoleOwner Role = "owner"; RoleTenant Role = "tenant" )
type User struct { ID int64; Email, Name string; Role Role; PasswordHash string; CreatedAt, UpdatedAt time.Time }
```

### `internal/shared/password` (from `internal/password`)

```go
type Params struct { ... }
var DefaultParams Params
func New(pepper string, params Params) *Hasher
func (h *Hasher) Hash(plain string) (string, error)
func (h *Hasher) Verify(encoded, plain string) bool
```

### `internal/shared/config` (from `internal/config`)

```go
type Config struct { HTTPAddr, DatabaseURL, MigrationsDir, PasswordPepper string; SessionTTL time.Duration; CookieSecure bool }
func Load() (Config, error)
```

### `internal/shared/templates` (from `internal/web/templates`)

- Exposes the shared base layout component used by feature templates.
- Feature templates import it; it MUST NOT import feature templates.

## Boundary Rules (enforced by the architecture test)

1. Feature packages (`internal/auth`, `internal/home`) MUST NOT import each
   other.
2. Feature packages MUST NOT import legacy shared paths: `internal/config`,
   `internal/db`, `internal/model`, `internal/password`, `internal/session`,
   `internal/web`.
3. Feature packages MAY import `internal/shared/*` and their own subpackages
   (`templates`).
4. Shared packages MUST NOT import feature packages.
5. `internal/app` is the composition root and is the only package that may
   import every feature.

## Wiring Contract

`internal/app.New(cfg config.Config, db *sql.DB) http.Handler` remains the
single place where concrete adapters are built and connected:

```text
cfg, db
  ├── shared/session.New(db, cfg.SessionTTL)
  ├── auth.NewPostgresUserRepository(db)
  ├── auth.New(auth.Deps{Users: repo, Sessions: sessions, Hasher: hasher, ...})
  ├── home.New(home.Deps{Users: repo, Sessions: sessions, Secure: ...})
  └── mux: authModule.RegisterRoutes(mux); homeModule.RegisterRoutes(mux); /static/
```

No feature or shared package may construct another feature's dependencies.
