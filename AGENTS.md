# Repository Guidelines

## Project Structure & Module Organization
`backend/` contains the Go API server. Entry point is `backend/cmd/server/main.go`; business logic lives in `backend/internal/services`, HTTP handlers in `backend/internal/handlers`, and shared utilities in `backend/pkg`. `frontend/` is a Vue 3 + TypeScript app with pages in `frontend/src/views`, state in `frontend/src/stores`, routing in `frontend/src/router`, and API clients in `frontend/src/services`. `docs/` stores reference docs. Do not hand-edit `backend/internal/frontend/dist`; refresh it from the frontend build instead.

## Build, Test, and Development Commands
Use Docker for a full stack boot: `docker compose up -d --build`. For backend-only work: `cd backend && go run ./cmd/server/main.go`. Run backend tests with `cd backend && go test ./...`. For frontend development: `cd frontend && pnpm install && pnpm dev`. Validate frontend quality with `pnpm type-check`, `pnpm lint`, `pnpm test:unit`, and `pnpm test:e2e`. To refresh the embedded frontend assets used by the Go server, run `cd frontend && pnpm run build:embed`.

## Coding Style & Naming Conventions
Follow the existing style of each stack. Go code should remain `gofmt`-compatible, use tabs, and keep package names lowercase. Vue and TypeScript files use 2-space indentation, `PascalCase` for view components such as `DashboardView.vue`, and camelCase for stores, composables, and helpers such as `usePageMeta.ts`. Prefer descriptive service and handler names that match resource names (`database`, `field`, `organization`). Use `frontend/eslint.config.ts`, `prettier`, and `vue-tsc` before opening a PR.

## Testing Guidelines
Backend tests use Go's testing package plus `testify`; keep files named `*_test.go` near the code they verify, for example `backend/internal/services/auth_service_test.go`. Frontend end-to-end tests live in `frontend/e2e` and follow `*.spec.ts`. Frontend unit tests should live under `src/**/__tests__`. Add or update tests for permission rules, API changes, and data-query behavior when touching those areas.

## Commit & Pull Request Guidelines
Recent history follows Conventional Commits, for example `feat: prepare v1.1.0 release`, `fix(frontend): resolve all vue-tsc type-check errors`, and `ci: fix release workflow go cache path`. Keep the type lowercase and add a scope when the change is isolated. PRs should include a short summary, affected areas (`backend`, `frontend`, `docs`), linked issue if any, and the exact verification commands you ran. Include screenshots for UI changes and note any `.env` or deployment impact.

## Security & Configuration Tips
Keep secrets in local `.env` files and never commit real credentials. Replace the development `JWT_SECRET` before production use, and prefer `SERVER_MODE=release` outside local development.
