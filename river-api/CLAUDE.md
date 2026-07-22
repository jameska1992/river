# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Run the server
go run ./cmd/server

# Build
go build ./...

# Tests
go test ./...
go test ./internal/services/... -run TestName -v

# Vet
go vet ./...

# Sync dependencies
go mod tidy
```

Environment variables (all optional, defaults shown):
```
PORT=8080
DATABASE_URL=postgres://river:river@localhost:5432/river?sslmode=disable
JWT_SECRET=change-me-in-production
JWT_ACCESS_EXPIRY_MINUTES=15
JWT_REFRESH_EXPIRY_DAYS=7
MEDIA_BASE_PATH=/media
```

## Architecture

Four strict layers. Each layer only imports the one below it.

```
handlers  →  services  →  repository  →  database (GORM/Postgres)
                ↑               ↑
            apperrors       apperrors
```

- **`internal/apperrors`** — sentinel errors (`ErrNotFound`, `ErrConflict`, `ErrUnauthorized`). Zero dependencies. The only package imported by both `repository` and `services`.
- **`internal/repository`** — GORM is confined here. Each file exposes an interface (e.g. `MovieRepository`) and an unexported struct that implements it, returned via `New*Repository(db)`. Translates `gorm.ErrRecordNotFound` → `apperrors.ErrNotFound` before returning.
- **`internal/services`** — Business logic. Holds no `*gorm.DB`; takes repository interfaces as constructor arguments. `services/errors.go` re-exports the `apperrors` sentinels so handlers only need to import `services`.
- **`internal/handlers`** — HTTP only: bind request, call service, map error to status via `serviceStatus()` in `helpers.go`, return JSON. No business logic, no GORM.

Wiring happens in `cmd/server/main.go` in three explicit blocks: repos → services → handlers.

## Key Patterns

**Adding a new media domain** (e.g. podcasts) requires touching four files in order:
1. `internal/models/` — embed `Base`, add GORM tags
2. `internal/repository/` — interface + unexported GORM impl
3. `internal/services/` — service struct taking repo interface(s), Input/Filter types
4. `internal/handlers/` — handler struct taking service, request types with binding tags
5. Wire in `internal/routes/routes.go` and `cmd/server/main.go`

**Models** all embed `Base` (`internal/models/base.go`), which provides a UUID v4 primary key auto-generated in `BeforeCreate`. The `Genres` and `Paths` fields are JSON-encoded `[]string` stored as plain string columns — there is no JSON column type.

**Auth flow** — access tokens are short-lived JWTs (HS256). Refresh tokens are opaque UUIDs stored in the `refresh_tokens` table and rotated on every use (old token is revoked, new one issued). The first user to register is automatically assigned the `admin` role. All mutations (POST/PUT/DELETE) on media and library routes require `admin`; reads require any authenticated user.

**Streaming** — `serveMediaFile` in `handlers/stream.go` uses `http.ServeContent`, which handles `Range` headers automatically, enabling seeking without downloading the full file.

**Error mapping** — `serviceStatus(err)` in `handlers/helpers.go` is the single place that converts `apperrors` sentinels to HTTP status codes. Add new sentinel checks there when adding new error types.

**Pagination** — handlers extract `page`/`limit` query params via `parsePaginationQuery`, pass them to the service, which converts to `offset`/`limit` via `paginationOffsetLimit` (`services/helpers.go`) before calling the repository. Default limit is 50, max 200.
