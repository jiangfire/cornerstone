# Cornerstone CLI Build Makefile
# Windows compatible (uses pwsh for cross-platform commands)

.PHONY: help build clean run dev

.DEFAULT_GOAL := help

# ============================================
# 项目配置
# ============================================
BINARY_NAME=cornerstone
BUILD_DIR=./bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-s -w -X github.com/jiangfire/cornerstone/internal/cli.Version=$(VERSION)

GO=go
CGO_ENABLED=0
GOOS=linux
GOARCH=amd64

# Docker 配置
DOCKER_IMAGE=cornerstone:latest
DOCKER_COMPOSE_FILE=docker-compose.yml

# 跨平台命令（pwsh）
MKDIR_P=@pwsh -Command "New-Item -ItemType Directory -Path $(BUILD_DIR) -Force -ErrorAction SilentlyContinue | Out-Null"

# ============================================
# 帮助信息
# ============================================

help: ## Show this help information
	@echo Cornerstone - Data Asset Platform CLI
	@echo.
	@echo Usage:
	@echo   make [target]
	@echo.
	@pwsh -Command "Get-Content $(MAKEFILE_LIST) | Select-String '^[a-zA-Z_-]+:.*?##' | ForEach-Object { $m = [regex]::Match($_, '^([a-zA-Z_-]+):.*?##\s*(.+)' ); Write-Host ('  {0,-20} {1}' -f $m.Groups[1].Value, $m.Groups[2].Value) }"

##@ Build commands

build: ## Build binary (current platform)
	@echo Building cornerstone...
	$(MKDIR_P)
	@pwsh -Command "$$env:CGO_ENABLED='$(CGO_ENABLED)'; go build -trimpath -ldflags='$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd"
	@echo Build complete: $(BUILD_DIR)/$(BINARY_NAME)

build-linux: ## Build Linux static binary
	@echo Building Linux static binary...
	$(MKDIR_P)
	@pwsh -Command "$$env:CGO_ENABLED='0'; $$env:GOOS='linux'; $$env:GOARCH='amd64'; go build -trimpath -ldflags='$(LDFLAGS)' -tags=sqlite_omit_load_extension -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd"
	@echo Build complete: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64

build-win: ## Build Windows binary
	@echo Building Windows binary...
	$(MKDIR_P)
	@pwsh -Command "$$env:CGO_ENABLED='$(CGO_ENABLED)'; go build -trimpath -ldflags='$(LDFLAGS)' -o $(BUILD_DIR)/$(BINARY_NAME).exe ./cmd"
	@echo Build complete: $(BUILD_DIR)/$(BINARY_NAME).exe

##@ Development commands

dev: ## Start development server (HTTP API + MCP)
	@echo Starting development server...
	@$(GO) run ./cmd serve

dev-hot: ## Start server (hot reload, requires air)
	@echo Starting server (hot reload)...
	@air

##@ Test commands

test: ## Run all tests
	@echo Running tests (race detection)...
	@$(GO) test -race -v ./...

test-no-race: ## Run tests (no race detection, for CGO-disabled environments)
	@echo Running tests (no race detection)...
	@pwsh -Command "$$env:CGO_ENABLED='0'; go test -v ./..."

test-cover: ## Run tests (coverage analysis)
	@echo Running tests (coverage analysis)...
	@pwsh -Command "$$env:CGO_ENABLED='0'; go test -coverprofile=coverage.out -covermode=atomic ./..."
	@$(GO) tool cover -html=coverage.out -o coverage.html
	@echo Coverage report: coverage.html

test-clean: ## Clean test cache and orphaned test artifacts
	@echo Cleaning test cache...
	@$(GO) clean -testcache
	@echo Cleaning orphaned test databases...
	@-pwsh -Command "Get-ChildItem -Path . -Filter 'test_*.db' -Recurse | Remove-Item -Force"
	@echo Done.

##@ Quality checks

lint: ## Run code checks
	@echo Running code checks...
	@golangci-lint run --config=.golangci.yml ./...

fmt: ## Format code
	@echo Formatting code...
	@$(GO) fmt ./...

vet: ## Run go vet static analysis
	@echo Running go vet...
	@pwsh -Command "$$env:CGO_ENABLED='0'; go vet ./..."

check: fmt vet test ## Full code checks

##@ Database commands

migrate: ## Run database migrations
	@echo Running database migrations...
	@$(GO) run ./cmd migrate

db-reset: ## Reset database (requires CONFIRM=1)
	@pwsh -Command "if ('$(CONFIRM)' -ne '1') { Write-Host 'Warning: this will delete all data!'; Write-Host 'Confirmation required, please run: make db-reset CONFIRM=1'; exit 1 }"
	@pwsh -Command "Remove-Item -Force -ErrorAction SilentlyContinue 'cornerstone.db'"
	@echo Database reset

##@ Swagger commands

swagger: ## Generate swagger docs
	@echo Generating swagger docs...
	@go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g internal/cli/serve.go -o internal/swagger -d ./,./internal/handlers,./internal/swagger --parseDependency --parseInternal --packagePrefix github.com/jiangfire/cornerstone
	@echo Swagger docs generated: internal/swagger/

##@ Docker commands

docker-build: ## Build Docker image
	@echo Building Docker image...
	@docker build -t $(DOCKER_IMAGE) .
	@echo Docker image build complete: $(DOCKER_IMAGE)

docker-up: ## Start Docker containers
	@echo Starting Docker containers...
	@docker compose -f $(DOCKER_COMPOSE_FILE) up -d
	@echo Containers started

docker-down: ## Stop Docker containers
	@echo Stopping Docker containers...
	@docker compose -f $(DOCKER_COMPOSE_FILE) down
	@echo Containers stopped

docker-logs: ## View Docker logs
	@docker compose -f $(DOCKER_COMPOSE_FILE) logs -f

docker-restart: docker-down docker-up ## Restart Docker containers

docker-clean: ## Clean Docker resources
	@echo Cleaning Docker resources...
	@docker compose -f $(DOCKER_COMPOSE_FILE) down -v
	@docker system prune -f
	@echo Cleanup complete

##@ Clean commands

clean: ## Clean all build files
	@echo Cleaning build files...
	@pwsh -Command "Remove-Item -Recurse -Force -ErrorAction SilentlyContinue '$(BUILD_DIR)'; Remove-Item -Force -ErrorAction SilentlyContinue 'coverage.out','coverage.html'"
	@echo Cleanup complete

##@ Dependency commands

deps: ## Download dependencies
	@echo Downloading dependencies...
	@$(GO) mod download

deps-tidy: ## Tidy dependencies
	@echo Tidying dependencies...
	@$(GO) mod tidy

##@ Info commands

info: ## Show project information
	@echo Project information:
	@echo   Project name: $(BINARY_NAME)
	@echo   Version: $(VERSION)
	@echo   Build directory: $(BUILD_DIR)
	@echo   Docker image: $(DOCKER_IMAGE)

version: ## Show version information
	@echo Version: $(VERSION)
	@echo Go version:
	@$(GO) version

##@ Release commands

release: clean check build ## Release pipeline (clean + check + build)

release-all: clean check ## Release for all platforms
	@echo Releasing for all platforms...
	$(MKDIR_P)
	@pwsh -Command "$$env:CGO_ENABLED='0'; $$env:GOOS='linux'; $$env:GOARCH='amd64'; go build -trimpath -ldflags='$(LDFLAGS)' -tags=sqlite_omit_load_extension -o $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 ./cmd"
	@pwsh -Command "$$env:CGO_ENABLED='0'; $$env:GOOS='windows'; $$env:GOARCH='amd64'; go build -trimpath -ldflags='$(LDFLAGS)' -tags=sqlite_omit_load_extension -o $(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe ./cmd"
	@pwsh -Command "$$env:CGO_ENABLED='0'; $$env:GOOS='darwin'; $$env:GOARCH='amd64'; go build -trimpath -ldflags='$(LDFLAGS)' -tags=sqlite_omit_load_extension -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 ./cmd"
	@pwsh -Command "$$env:CGO_ENABLED='0'; $$env:GOOS='darwin'; $$env:GOARCH='arm64'; go build -trimpath -ldflags='$(LDFLAGS)' -tags=sqlite_omit_load_extension -o $(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 ./cmd"
	@echo Release complete:
	@pwsh -Command "Get-ChildItem $(BUILD_DIR)/$(BINARY_NAME)-* | Format-Table Name, Length"

##@ Install development tools

install-tools: ## Install development tools
	@echo Installing development tools...
	@$(GO) install github.com/air-verse/air@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/securego/gosec/v2/cmd/gosec@latest
	@echo Development tools installation complete
	@echo   - air: hot reload
	@echo   - golangci-lint: code checking
	@echo   - gosec: security scanning
