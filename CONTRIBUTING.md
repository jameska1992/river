# Contributing to River

Thanks for your interest in improving River! This guide covers how to get set up,
what's expected of a change, and how to get it merged.

By contributing, you agree that your contributions are licensed under the project's
[AGPL-3.0](./LICENSE) license.

## Repository layout

River is a monorepo of independent components. Each backend service is its own Go
module (its own `go.mod`); the clients are separate JS/TS or Android projects.

- **Go services:** `river-api`, `river-scan`, `river-video-trans`, `river-audio-trans`,
  `river-meta-movie`, `river-meta-tv`, `river-meta-book`, `river-meta-music`
- **Clients:** `river-web`, `river-tv` (React + Vite), `river-tv-android` (Kotlin/Gradle)

Each directory has its own `README.md`, and each Go service also has a `CLAUDE.md`
that doubles as an architecture reference — read the one for the component you're
touching before making changes.

## Prerequisites

- **Go 1.26+** (matching the `go` directive in each service's `go.mod`)
- **Node.js 20+** for the web clients
- **Docker + Docker Compose** to run the full stack locally (see the root
  [`README.md`](./README.md))
- **FFmpeg/FFprobe** on `PATH` if working on the transcoders

## Development workflow

Work from within the specific component's directory — there is no root-level build.

**Go services:**

```bash
cd river-<service>
go build ./...   # build
go vet ./...     # static analysis
go test ./...    # tests
go mod tidy      # keep dependencies clean
```

**Web clients (`river-web`, `river-tv`):**

```bash
cd river-<client>
npm ci
npm run lint     # eslint
npm run build    # type-check + production build
npm run dev      # local dev server (override the API target with RIVER_API_TARGET)
```

## Before you open a PR

Your change should pass the same checks CI runs (`.github/workflows/ci.yml`):

- **Go:** `go build ./...`, `go vet ./...`, and `go test ./...` pass in every
  service you touched.
- **Web:** `npm run lint` and `npm run build` pass in every client you touched.
- Add or update tests when you change behaviour. Follow the existing test patterns
  in the package (e.g. `river-scan`, `river-meta-movie`).
- Keep changes scoped to one component where possible. If a change spans the API and
  a client, call that out in the PR description.

## Pull requests

1. Fork the repo (or create a branch if you have write access) and branch off `main`.
2. Make your change with clear, focused commits.
3. Open a PR against `main` describing **what** changed and **why**.
4. CI must be green, and `main` is protected: PRs require **one approving review** and
   all review conversations resolved before merging.

Keep PRs as small as reasonably possible — smaller changes are reviewed and merged faster.

## Reporting bugs and requesting features

Open a GitHub issue with enough detail to reproduce (for bugs: steps, expected vs.
actual, relevant logs, and which service/client). For **security vulnerabilities**, do
**not** open a public issue — use GitHub's private vulnerability reporting on the repo's
Security tab.
