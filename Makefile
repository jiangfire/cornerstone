# Cornerstone full build Makefile
# Supports frontend + backend + embed workflow

.PHONY: help build clean run dev

# 默认目标
.DEFAULT_GOAL := help

# ============================================
# 项目配置
# ============================================
BINARY_NAME=cornerstone
BUILD_DIR=./bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X main.version=$(VERSION)

# 后端配置
BACKEND_DIR=./backend
BACKEND_MAIN=$(BACKEND_DIR)/cmd/server/main.go
GO=go
CGO_ENABLED=0
GOOS=linux
GOARCH=amd64

# 前端配置
FRONTEND_DIR=./frontend
FRONTEND_DIST=$(FRONTEND_DIR)/dist
FRONTEND_EMBED=$(BACKEND_DIR)/internal/frontend/dist
NODE=node
NPM=npm
PNPM=pnpm

# Docker 配置
DOCKER_IMAGE=cornerstone:latest
DOCKER_COMPOSE_FILE=docker-compose.yml

# ============================================
# 帮助信息
# ============================================

help: ## Show this help information
	@echo 'Cornerstone - Enterprise data platform'
	@echo ''
	@echo 'Usage:'
	@echo '  make [target]'
	@echo ''
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Full build workflow

all: clean frontend-build backend-embed-build ## Full build (clean + frontend + embedded backend)

quick: frontend-backend-embed ## Quick build (frontend + backend embed)

##@ Frontend commands

frontend-dev: ## Start frontend development server
	@echo "Starting frontend development server..."
	@cd $(FRONTEND_DIR) && $(PNPM) dev

frontend-build: ## Build frontend (production mode)
	@echo "Building frontend..."
	@cd $(FRONTEND_DIR) && $(PNPM) build-only
	@echo "Frontend build complete: $(FRONTEND_DIST)"

frontend-build-embed: ## Build frontend and copy into backend (embed use)
	@echo "Building frontend (embed mode)..."
	@cd $(FRONTEND_DIR) && $(PNPM) build:embed
	@echo "Frontend embedded into backend: $(FRONTEND_EMBED)"

frontend-lint: ## Check frontend code
	@echo "Checking frontend code..."
	@cd $(FRONTEND_DIR) && $(PNPM) lint

frontend-format: ## Format frontend code
	@echo "Formatting frontend code..."
	@cd $(FRONTEND_DIR) && $(PNPM) format

frontend-test: ## Run frontend tests
	@echo "Running frontend tests..."
	@cd $(FRONTEND_DIR) && $(PNPM) test:unit

frontend-type-check: ## Frontend type check
	@echo "Performing frontend type check..."
	@cd $(FRONTEND_DIR) && $(PNPM) type-check

frontend-install: ## Install frontend dependencies
	@echo "Installing frontend dependencies..."
	@cd $(FRONTEND_DIR) && $(PNPM) install

frontend-clean: ## Clean frontend build files
	@echo "Cleaning frontend build files..."
	@rm -rf $(FRONTEND_DIST)
	@rm -rf $(FRONTEND_EMBED)
	@echo "Frontend cleanup complete"

##@ Backend commands

backend-dev: ## Start backend development server
	@echo "Starting backend development server..."
	@cd $(BACKEND_DIR) && $(GO) run $(BACKEND_MAIN)

backend-dev-hot: ## Start backend (hot reload, requires air)
	@echo "Starting backend (hot reload)..."
	@cd $(BACKEND_DIR) && which air > /dev/null || (echo "Error: air is not installed, run: go install github.com/air-verse/air@latest" && exit 1)
	@cd $(BACKEND_DIR) && air

backend-build: ## Build backend (current platform)
	@echo "Building backend..."
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY_NAME) $(BACKEND_MAIN)
	@echo "Backend build complete: $(BUILD_DIR)/$(BINARY_NAME)"

backend-build-linux: ## Build Linux static binary
	@echo "Building Linux static binary..."
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BACKEND_MAIN)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

backend-build-win: ## Build Windows binary
	@echo "Building Windows binary..."
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY_NAME).exe $(BACKEND_MAIN)
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME).exe"

backend-embed-build: ## Build embedded backend (includes frontend resources)
	@echo "Building embedded backend..."
	@test -d $(FRONTEND_EMBED) || (echo "Error: frontend assets are not embedded, please run 'make frontend-build-embed' first" && exit 1)
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY_NAME) $(BACKEND_MAIN)
	@echo "Embedded backend build complete: $(BUILD_DIR)/$(BINARY_NAME)"

backend-test: ## Run backend tests (race detection, default entry)
	@echo "Running backend tests (race detection)..."
	@cd $(BACKEND_DIR) && $(GO) test -race -v ./...

backend-test-no-race: ## Run backend tests (no race detection, for CGO-disabled environments)
	@echo "Running backend tests (no race detection)..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) test -v ./...

backend-test-race: backend-test ## Alias: equivalent to backend-test (kept for compatibility)

backend-test-cover: ## Run backend tests (coverage analysis)
	@echo "Running backend tests (coverage analysis)..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	@cd $(BACKEND_DIR) && $(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: backend/coverage.html"

backend-lint: ## Run backend code checks
	@echo "Running backend code checks..."
	@cd $(BACKEND_DIR) && which golangci-lint > /dev/null || (echo "Warning: golangci-lint is not installed" && exit 0)
	@cd $(BACKEND_DIR) && golangci-lint run --config=../.golangci.yml ./...

backend-fmt: ## Format backend code
	@echo "Formatting backend code..."
	@cd $(BACKEND_DIR) && $(GO) fmt ./...

backend-vet: ## Run go vet static analysis
	@echo "Running go vet..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) vet ./...

backend-deps: ## Download backend dependencies
	@echo "Downloading backend dependencies..."
	@cd $(BACKEND_DIR) && $(GO) mod download

backend-deps-tidy: ## Tidy backend dependencies
	@echo "Tidying backend dependencies..."
	@cd $(BACKEND_DIR) && $(GO) mod tidy

backend-clean: ## Clean backend build files
	@echo "Cleaning backend build files..."
	@rm -rf $(BUILD_DIR)
	@cd $(BACKEND_DIR) && rm -f coverage.out coverage.html
	@echo "Backend cleanup complete"

##@ Composite commands

frontend-backend-embed: frontend-build-embed backend-build ## Build frontend (embed) + backend

dev: frontend-dev ## Start frontend development server

dev-backend: backend-dev ## Start backend development server

dev-all: ## Start frontend and backend development servers together
	@echo "Starting frontend and backend development servers..."
	@make -j2 frontend-dev backend-dev-hot

##@ Docker commands

docker-build: ## Build Docker image
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Docker image build complete: $(DOCKER_IMAGE)"

docker-build-embed: ## Build embedded Docker image (includes frontend)
	@echo "Building embedded Docker image..."
	@make frontend-build-embed
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Embedded Docker image build complete"

docker-up: ## Start Docker containers
	@echo "Starting Docker containers..."
	@docker compose -f $(DOCKER_COMPOSE_FILE) up -d
	@echo "Containers started"

docker-down: ## Stop Docker containers
	@echo "Stopping Docker containers..."
	@docker compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "Containers stopped"

docker-logs: ## View Docker logs
	@docker compose -f $(DOCKER_COMPOSE_FILE) logs -f

docker-restart: docker-down docker-up ## Restart Docker containers

docker-clean: ## Clean Docker resources
	@echo "Cleaning Docker resources..."
	@docker compose -f $(DOCKER_COMPOSE_FILE) down -v
	@docker system prune -f
	@echo "Cleanup complete"

##@ Clean commands

clean: frontend-clean backend-clean ## Clean all build files
	@echo "All cleanup complete"

clean-all: clean ## Clean all build files and dependencies
	@echo "Full cleanup complete"

##@ Database commands

db-migrate: ## Run database migrations
	@echo "Running database migrations..."
	@cd $(BACKEND_DIR) && $(GO) run $(BACKEND_MAIN) migrate

db-reset: ## Reset database (requires CONFIRM=1, e.g.: make db-reset CONFIRM=1)
	@if [ "$(CONFIRM)" != "1" ]; then \
		echo "Warning: this will delete all data!"; \
		echo "Confirmation required, please run: make db-reset CONFIRM=1"; \
		exit 1; \
	fi
	@rm -f cornerstone.db backend/cornerstone.db
	@echo "Database reset"

##@ Quality checks

check-frontend: frontend-lint frontend-type-check ## Frontend code checks

check-backend: backend-fmt backend-vet backend-test ## Backend code checks

check: check-frontend check-backend ## Full frontend/backend code checks

security-scan: ## Run security scan
	@echo "Running security scan..."
	@cd $(BACKEND_DIR) && which gosec > /dev/null || (echo "Warning: gosec is not installed, please run: go install github.com/securego/gosec/v2/cmd/gosec@latest" && exit 0)
	@cd $(BACKEND_DIR) && gosec ./...

##@ Quick commands

quick-backend: backend-build ## Quick build backend

quick-frontend: frontend-build ## Quick build frontend

test: frontend-test backend-test ## Run all tests

test-all: frontend-test backend-test-cover ## Run all tests (with coverage)

##@ Info commands

info: ## Show project information
	@echo "Project information:"
	@echo "  Project name: $(BINARY_NAME)"
	@echo "  Version: $(VERSION)"
	@echo "  Frontend directory: $(FRONTEND_DIR)"
	@echo "  Backend directory: $(BACKEND_DIR)"
	@echo "  Build directory: $(BUILD_DIR)"
	@echo "  Docker image: $(DOCKER_IMAGE)"

version: ## Show version information
	@echo "Version: $(VERSION)"
	@echo "Go version:"
	@cd $(BACKEND_DIR) && $(GO) version
	@echo "Node version:"
	@$(NODE) --version
	@echo "pnpm version:"
	@$(PNPM) --version

deps-tree: ## Show backend dependency tree
	@echo "Backend dependency tree:"
	@cd $(BACKEND_DIR) && $(GO) mod graph | head -20

deps-why: ## Analyze backend dependency relationships
	@echo "Usage: make deps-why PACKAGE=package.name"
	@test -n "$(PACKAGE)" || (echo "Error: please specify PACKAGE=package.name" && exit 1)
	@cd $(BACKEND_DIR) && $(GO) mod why $(PACKAGE)

##@ Release commands

release: clean check frontend-backend-embed ## Release pipeline (clean + check + embedded build)

release-all: clean check ## Release for all platforms
	@echo "Releasing for all platforms..."
	@make frontend-build-embed
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BACKEND_MAIN)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(BACKEND_MAIN)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(BACKEND_MAIN)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BACKEND_MAIN)
	@echo "Release complete:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-*

##@ Install development tools

install-tools-backend: ## Install backend development tools
	@echo "Installing backend development tools..."
	@$(GO) install github.com/air-verse/air@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo "Backend development tools installation complete"
	@echo "  - air: hot reload"
	@echo "  - golangci-lint: code checking"
	@echo "  - gosec: security scanning"

install-tools-frontend: ## Install frontend development tools
	@echo "Frontend development tools are managed via pnpm"
	@echo "Please run: cd frontend && pnpm install"

install-tools: install-tools-backend install-tools-frontend ## 安装所有开发工具
