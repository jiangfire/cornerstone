# Repository Guidelines

## Project Overview
Go CLI + REST API server. Users interact via the `cornerstone` binary (CLI commands or `serve` mode for HTTP API).

## Build Commands
Build binary: `make build`. Run tests: `make test`. Start server: `make dev`. Generate swagger docs: `make swagger`. Docker: `docker compose up -d --build`.

## Module & Import Path
Module: `github.com/jiangfire/cornerstone`. Import paths use this prefix directly (no `backend/` subdirectory). CLI entry: `cmd/main.go`. Commands: `internal/cli/`. Handlers: `internal/handlers/`. Services: `internal/services/`. Shared utilities: `pkg/`. Swagger types: `docs/swagger/`.

## Coding Style & Naming Conventions
Follow Go conventions. Code should remain `gofmt`-compatible, use tabs, and keep package names lowercase. CLI commands use Cobra framework with subcommands (db, table, field, record, token, serve, migrate). Validate with `make fmt`, `make vet`, `make lint`, and `make test` before opening a PR.

## Testing Guidelines
Tests use Go's testing package plus `testify`; keep files named `*_test.go` near the code they verify. Add or update tests for permission rules, API changes, and data-query behavior when touching those areas.

## Commit & PR Guidelines
Conventional Commits. Keep the type lowercase and add a scope when the change is isolated. PRs should include a short summary, affected areas, linked issue if any, and the exact verification commands you ran.

## Swagger
Handler files contain swag annotations. Types live in `docs/swagger/models.go`. After changing annotations or types, regenerate: `make swagger` (runs `swag init`).

## Security
Keep secrets in local `.env` files and never commit real credentials. Use `MASTER_TOKEN` to pre-set a master token in production; leave it empty to auto-generate on startup. Prefer `SERVER_MODE=release` outside local development.
