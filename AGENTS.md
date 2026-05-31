# Repository Guidelines

## Project Structure & Module Organization
Go CLI + API server. Entry point is `cmd/main.go`; CLI commands live in `internal/cli`, business logic in `internal/services`, HTTP handlers in `internal/handlers`, shared utilities in `pkg`, and swagger type definitions in `docs/swagger`.

## Build, Test, and Development Commands
Use Docker for a full stack boot: `docker compose up -d --build`. Start dev server: `go run ./cmd/main.go serve`. Run tests: `go test ./...`. CLI usage: `go run ./cmd/main.go --help`. Validate code quality with `make fmt`, `make vet`, `make lint`, and `make test`. Build binary: `make build`. Generate swagger docs: `make swagger`.

## Coding Style & Naming Conventions
Follow Go conventions. Code should remain `gofmt`-compatible, use tabs, and keep package names lowercase. CLI commands use Cobra framework with subcommands (db, table, field, record, token, serve, migrate). Validate with `make fmt`, `make vet`, `make lint`, and `make test` before opening a PR.

## Testing Guidelines
Tests use Go's testing package plus `testify`; keep files named `*_test.go` near the code they verify. Add or update tests for permission rules, API changes, and data-query behavior when touching those areas.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commits. Keep the type lowercase and add a scope when the change is isolated. PRs should include a short summary, affected areas, linked issue if any, and the exact verification commands you ran.

## Security & Configuration Tips
Keep secrets in local `.env` files and never commit real credentials. Use `MASTER_TOKEN` to pre-set a master token in production; leave it empty to auto-generate on startup. Prefer `SERVER_MODE=release` outside local development.
