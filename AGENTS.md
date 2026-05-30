# Repository Guidelines

## Project Structure & Module Organization
`backend/` contains the Go CLI + API server. Entry point is `backend/cmd/main.go`; CLI commands live in `backend/internal/cli`, business logic in `backend/internal/services`, HTTP handlers in `backend/internal/handlers`, and shared utilities in `backend/pkg`. `docs/` stores reference docs.

## Build, Test, and Development Commands
Use Docker for a full stack boot: `docker compose up -d --build`. For backend-only work: `cd backend && go run ./cmd/main.go serve`. Run backend tests with `cd backend && go test ./...`. For CLI usage: `cd backend && go run ./cmd/main.go --help`. Validate code quality with `make fmt`, `make vet`, `make lint`, and `make test`. Build binary: `make build`.

## Coding Style & Naming Conventions
Follow Go conventions. Code should remain `gofmt`-compatible, use tabs, and keep package names lowercase. CLI commands use Cobra framework with subcommands (db, table, field, record, token, serve, migrate). Validate with `make fmt`, `make vet`, `make lint`, and `make test` before opening a PR.

## Testing Guidelines
Backend tests use Go's testing package plus `testify`; keep files named `*_test.go` near the code they verify. Add or update tests for permission rules, API changes, and data-query behavior when touching those areas.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commits. Keep the type lowercase and add a scope when the change is isolated. PRs should include a short summary, affected areas (`backend`, `docs`), linked issue if any, and the exact verification commands you ran.

## Security & Configuration Tips
Keep secrets in local `.env` files and never commit real credentials. Use `MASTER_TOKEN` to pre-set a master token in production; leave it empty to auto-generate on startup. Prefer `SERVER_MODE=release` outside local development.
