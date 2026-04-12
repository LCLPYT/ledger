# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

All common dev tasks are defined in the `justfile` and use `just`:

```sh
just serve          # Start postgres (Docker) and run the server
just db             # Start postgres only
just stop_db        # Stop postgres
just delete_db      # Stop postgres and delete the volume
just psql           # Open a psql shell in the container
just create_user    # Interactive CLI to create a user in the DB
just build          # Build optimized binary
just test           # run tests (uses port 5433 — test DB)
just test TestFoo   # run only tests matching "TestFoo"
```

> **WARNING — never run tests against port 5432.** Tests call `TRUNCATE` on startup and will wipe the development database. Always use `just test`, which targets the dedicated test container on port 5433.

Run the server manually:
```sh
DATABASE_URL="postgres://db:db@localhost:5432/db?sslmode=disable" go run .
```

The server also requires `JWT_SECRET` to be set (used by `auth/jwt.go`).

## Architecture

This is a Go REST API using:
- **Gin** — HTTP router
- **database/sql + lib/pq** — raw SQL against PostgreSQL (no ORM)
- **golang-migrate** — runs `migrations/` on startup automatically
- **Casbin** — RBAC/ABAC authorization, backed by a PostgreSQL adapter
- **golang-jwt** — HS256 JWT tokens

### Request lifecycle

1. `main.go` initializes the DB (and runs migrations), then the Casbin enforcer, then calls `routes.SetupRoutes`.
2. Protected routes wrap handlers with `middleware.AuthRequired(enforcer, "permission")`.
3. The middleware validates the Bearer JWT, sets `userID` in the Gin context, then asks Casbin whether the user has the required permission.
4. Handlers receive `*sql.DB` via closure (injected in `routes/routes.go`).

### Authorization model (`casbin_model.conf`)

The Casbin model uses a four-part request: `sub, obj, act, scope`. The matcher enforces:
- The subject has the role/permission via `g(sub, role)` and `p` policies.
- The token's `scope` field in the JWT must match `obj.act` (i.e., a token is scope-limited to the permissions it was created with).

Casbin policies are stored in the database (not files).

### Token flow

- `POST /api/v1/user/login` → returns a short-lived JWT (7 days) with hardcoded `user.read` and `user.create_token` permissions.
- `POST /api/v1/user/token` (auth required, `user.create_token`) → creates a named long-lived token (max 1 year) stored in the `access_tokens` table with JSONB scopes.
- `auth.GenerateToken` writes a row to `access_tokens` and embeds the row ID as `token_id` in the JWT claims.

### Database schema

- `users` — id, username, email, created_at, password_hash (bcrypt)
- `access_tokens` — id, user_id (FK), name, created_at, expires_at, revoked, scopes (JSONB)

Migrations live in `migrations/` and run automatically on server startup.

> **Keep `routes/openapi.yaml` in sync** when adding or changing routes. Update paths, request bodies, response schemas, and any new component schemas.

### Package layout

| Package | Role |
|---|---|
| `auth` | JWT generation and `Claims` struct |
| `middleware` | `AuthRequired` Gin middleware (JWT parse + Casbin enforce) |
| `handlers` | HTTP handler functions, each taking `*sql.DB` |
| `routes` | Wires handlers and middleware to Gin routes |
| `models` | Request/response structs |
| `util` | bcrypt helpers |
| `db` | DB and Casbin initialization |
| `cmd/create_user` | CLI tool to provision users |
