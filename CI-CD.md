# CI/CD Configuration Guide

This document describes the GitHub Actions CI/CD workflows configured for the Cornerstone project.

## Workflows Overview

### 1. CI Workflow (`.github/workflows/ci.yml`)

**Trigger**: Push or Pull Request to `main`/`master` branch

**Jobs**:
- **Backend Tests**: Runs Go tests with coverage report
- **Frontend Build**: Builds frontend and runs type checking

**Usage**: Automatically runs on every push and PR to ensure code quality.

### 2. Release Workflow (`.github/workflows/release.yml`)

**Trigger**: Push tags matching `v*.*.*` (e.g., `v1.0.0`, `v0.1.0-beta`)

**Jobs**:
- **Build**: Cross-platform binary compilation
  - Linux (amd64, arm64)
  - Windows (amd64)
  - macOS (amd64, arm64)
- **Release**: Creates GitHub Release with binaries

**Usage**:
```bash
# Create and push a tag
git tag v1.0.0
git push origin v1.0.0
```

**Output**: GitHub Release with downloadable binaries for all platforms

### 3. Docker Workflow (`.github/workflows/docker.yml`)

**Trigger**:
- Push tags matching `v*.*.*`
- Manual dispatch

**Jobs**:
- **Build Backend**: Multi-platform Docker image (amd64, arm64)
- **Build Frontend**: Multi-platform Docker image (amd64, arm64)

**Images**: Published to GitHub Container Registry (ghcr.io)
- `ghcr.io/<username>/cornerstone/backend:<tag>`
- `ghcr.io/<username>/cornerstone/frontend:<tag>`

## Quick Start

### Creating a Release

1. **Commit all changes**
```bash
git add .
git commit -m "chore: prepare release v1.0.0"
```

2. **Create and push tag**
```bash
git tag v1.0.0
git push origin v1.0.0
```

3. **Wait for workflows**
   - Release workflow: ~5-10 minutes
   - Docker workflow: ~10-15 minutes

4. **Download binaries or pull Docker images**

### Using Docker Images

```bash
# Pull images
docker pull ghcr.io/<username>/cornerstone/backend:v1.0.0
docker pull ghcr.io/<username>/cornerstone/frontend:v1.0.0

# Or use docker-compose
docker-compose up -d
```

## Local Development

### Prerequisites
- Go 1.25.4+
- Node.js 20+
- pnpm 8+
- PostgreSQL 15+

### Setup

1. **Backend**
```bash
cd backend
cp .env.example .env
# Edit .env with your configuration
go mod download
go run cmd/server/main.go
```

2. **Frontend**
```bash
cd frontend
cp .env.example .env
# Edit .env with your configuration
pnpm install
pnpm dev
```

3. **Using Docker Compose**
```bash
# Build and start all services
docker-compose up -d

# View logs
docker-compose logs -f

# Stop services
docker-compose down
```

## Build Locally

### Backend Binary
```bash
cd backend
go build -o cornerstone ./cmd/server
```

### Frontend
```bash
cd frontend
pnpm build
```

### Docker Images
```bash
# Backend
docker build -t cornerstone-backend ./backend

# Frontend
docker build -t cornerstone-frontend ./frontend
```

## Troubleshooting

### CI Tests Failing
- Check test logs in GitHub Actions
- Run tests locally: `go test ./...` (backend) or `pnpm test` (frontend)
- Ensure all dependencies are up to date

### Release Workflow Not Triggering
- Ensure tag format matches `v*.*.*`
- Check GitHub Actions permissions
- Verify workflows are enabled in repository settings

### Docker Build Failing
- Check Dockerfile syntax
- Verify all files are included (check .dockerignore)
- Test build locally first

### Permission Issues
- GitHub Actions requires `write` permission for packages
- Check repository settings → Actions → General → Workflow permissions
- Enable "Read and write permissions"

## Environment Variables

### Backend (`.env`)
See `backend/.env.example` for all available options.

Key variables:
- `DATABASE_URL`: PostgreSQL connection string
- `JWT_SECRET`: JWT signing key (must be changed in production)
- `PORT`: Server port (default: 8080)

### Frontend (`.env`)
See `frontend/.env.example` for all available options.

Key variables:
- `VITE_API_BASE_URL`: Backend API URL

## Security Notes

1. **Never commit `.env` files** - they contain secrets
2. **Change JWT_SECRET in production** - use a strong random key
3. **Use HTTPS in production** - enable SSL/TLS
4. **Review permissions** - ensure GitHub Actions has minimal required permissions
5. **Scan images** - consider adding security scanning to Docker workflow

## Support

For issues or questions:
- Open an issue in the repository
- Check existing documentation in `docs/`
- Review workflow runs in GitHub Actions tab
