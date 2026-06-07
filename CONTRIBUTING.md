# Contributing to Cornerstone

Thank you for your interest in contributing to Cornerstone!

## Getting Started

### Prerequisites

- Go 1.26+
- Make (optional, for convenience commands)
- Docker (optional, for database integration tests)

### Setup

```bash
git clone https://github.com/jiangfire/cornerstone.git
cd cornerstone
cp .env.example .env
make build    # or: go build -o bin/cornerstone ./cmd
make test     # run all tests
```

## Development Workflow

### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use tabs for indentation
- Keep package names lowercase
- Add/update tests for new functionality
- Run `make check` before committing (fmt + vet + lint + test)

### Commit Messages

We use [Conventional Commits](https://www.conventionalcommits.org/):

```
type(scope): description

[optional body]

[optional footer]
```

Types: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`

Examples:
```
feat(query): add HAVING clause support
fix(auth): resolve token cache race condition
docs(readme): update API endpoint table
test(services): add record batch create tests
```

### Testing

```bash
# Run all tests
go test ./...

# Run with race detection
make test

# Run specific package tests
go test ./internal/services -v

# Run benchmarks
go test ./pkg/query -run ^$ -bench BenchmarkExecutorExecute -benchmem

# Run with specific database
DB_TYPE=postgres DATABASE_URL="host=localhost ..." go test ./...
```

### Pull Request Process

1. Fork the repository
2. Create a feature branch (`git checkout -b feat/my-feature`)
3. Make your changes with tests
4. Run `make check` to ensure quality
5. Commit with conventional commit format
6. Push to your fork and open a Pull Request

### PR Checklist

- [ ] Tests added/updated for new functionality
- [ ] `make check` passes locally
- [ ] Documentation updated (README, docs/, swagger annotations)
- [ ] No secrets or credentials committed
- [ ] Commit messages follow conventional format

## Project Structure

```
cornerstone/
├── cmd/                    # Entry point
├── internal/
│   ├── authz/             # Permission system
│   ├── cli/               # CLI commands
│   ├── config/            # Configuration
│   ├── db/                # Database migrations
│   ├── handlers/          # HTTP handlers
│   ├── mcp/               # MCP protocol implementation
│   ├── middleware/        # HTTP middleware
│   ├── migration/         # External DB migration
│   ├── models/            # Data models
│   ├── services/          # Business logic
│   └── swagger/           # Swagger types
├── pkg/                    # Shared packages
│   ├── cache/             # Cache abstractions
│   ├── db/                # Database connection
│   ├── dto/               # API response DTOs
│   ├── jsonx/             # JSON utilities
│   ├── log/               # Logging
│   └── query/             # Query DSL engine
├── docs/                   # Documentation
└── Makefile               # Build commands
```

## Reporting Issues

When reporting bugs, please include:

- Cornerstone version (`cornerstone --version`)
- Go version (`go version`)
- Database type and version
- Steps to reproduce
- Expected vs actual behavior
- Relevant logs (with sensitive info redacted)

## Security

If you discover a security vulnerability, please report it privately via GitHub Security Advisories instead of opening a public issue.

## License

By contributing, you agree that your contributions will be licensed under the AGPL-3.0 License.
