# river-api

Central REST API for River. Owns Postgres, handles auth, exposes CRUD for every media type, and serves media byte-range streams to clients.

## Role in the platform

Every other backend service is an HTTP client of this one — `river-scan` reads library config from here, the transcoders and metadata services register / update records here, and every client app (`river-web`, `river-tv`, `river-tv-android`) reads and writes state through here.

## Commands

```bash
go build ./...          # build
go test ./...           # run all tests
go test ./internal/services/... -run TestName -v
go vet ./...
go run ./cmd/server     # local run (requires Postgres reachable)
```

## Architecture

Strict top-down layering — see [`CLAUDE.md`](./CLAUDE.md) for the long version.

```
handlers → services → repository → GORM/Postgres
                ↑
          apperrors (sentinel errors only)
```

Auth is HS256 JWT access tokens (short-lived) + opaque rotated refresh tokens. The first registered user becomes admin.

Media streaming uses `http.ServeContent` for HTTP Range support (seeking without full download).

## Environment variables

| Variable | Default | Notes |
|---|---|---|
| `PORT` | `8080` | |
| `DATABASE_URL` | `postgres://river:river@localhost:5432/river?sslmode=disable` | |
| `JWT_SECRET` | *(required)* | HS256 signing key. |
| `JWT_ACCESS_EXPIRY_MINUTES` | `15` | |
| `JWT_REFRESH_EXPIRY_DAYS` | `7` | |
| `MEDIA_BASE_PATH` | `/media` | Prefix applied to relative file paths. |
| `CORS_ALLOWED_ORIGINS` | `*` | Comma-separated. |

## API docs

Swagger UI mounted at `/swagger/index.html` when the server is running (auth optional for the docs themselves).
