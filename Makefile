# Cornerstone 完整构建 Makefile
# 支持前端 + 后端 + Embed 流程

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

help: ## 显示此帮助信息
	@echo 'Cornerstone - 企业数据平台'
	@echo ''
	@echo '使用方法:'
	@echo '  make [target]'
	@echo ''
	@awk 'BEGIN {FS = ":.*##"} /^[a-zA-Z_-]+:.*?##/ { printf "  %-20s %s\n", $$1, $$2 } /^##@/ { printf "\n%s\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ 完整构建流程

all: clean frontend-build backend-embed-build ## 完整构建（清理 + 前端 + 嵌入式后端）

quick: frontend-backend-embed ## 快速构建（前端 + 后端嵌入）

##@ 前端命令

frontend-dev: ## 启动前端开发服务器
	@echo "启动前端开发服务器..."
	@cd $(FRONTEND_DIR) && $(PNPM) dev

frontend-build: ## 构建前端（生产模式）
	@echo "构建前端..."
	@cd $(FRONTEND_DIR) && $(PNPM) build-only
	@echo "前端构建完成: $(FRONTEND_DIST)"

frontend-build-embed: ## 构建前端并复制到后端（用于嵌入）
	@echo "构建前端（嵌入模式）..."
	@cd $(FRONTEND_DIR) && $(PNPM) build:embed
	@echo "前端已嵌入到后端: $(FRONTEND_EMBED)"

frontend-lint: ## 检查前端代码
	@echo "检查前端代码..."
	@cd $(FRONTEND_DIR) && $(PNPM) lint

frontend-format: ## 格式化前端代码
	@echo "格式化前端代码..."
	@cd $(FRONTEND_DIR) && $(PNPM) format

frontend-test: ## 运行前端测试
	@echo "运行前端测试..."
	@cd $(FRONTEND_DIR) && $(PNPM) test:unit

frontend-type-check: ## 前端类型检查
	@echo "前端类型检查..."
	@cd $(FRONTEND_DIR) && $(PNPM) type-check

frontend-install: ## 安装前端依赖
	@echo "安装前端依赖..."
	@cd $(FRONTEND_DIR) && $(PNPM) install

frontend-clean: ## 清理前端构建文件
	@echo "清理前端构建文件..."
	@rm -rf $(FRONTEND_DIST)
	@rm -rf $(FRONTEND_EMBED)
	@echo "前端清理完成"

##@ 后端命令

backend-dev: ## 启动后端开发服务器
	@echo "启动后端开发服务器..."
	@cd $(BACKEND_DIR) && $(GO) run $(BACKEND_MAIN)

backend-dev-hot: ## 启动后端（热重载，需要 air）
	@echo "启动后端（热重载）..."
	@cd $(BACKEND_DIR) && which air > /dev/null || (echo "错误: 未安装 air，运行: go install github.com/air-verse/air@latest" && exit 1)
	@cd $(BACKEND_DIR) && air

backend-build: ## 构建后端（当前平台）
	@echo "构建后端..."
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY_NAME) $(BACKEND_MAIN)
	@echo "后端构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

backend-build-linux: ## 构建 Linux 静态二进制文件
	@echo "构建 Linux 静态二进制文件..."
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BACKEND_MAIN)
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME)-linux-amd64"

backend-build-win: ## 构建 Windows 二进制文件
	@echo "构建 Windows 二进制文件..."
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY_NAME).exe $(BACKEND_MAIN)
	@echo "构建完成: $(BUILD_DIR)/$(BINARY_NAME).exe"

backend-embed-build: ## 构建嵌入式后端（包含前端资源）
	@echo "构建嵌入式后端..."
	@test -d $(FRONTEND_EMBED) || (echo "错误: 前端资源未嵌入，请先运行 'make frontend-build-embed'" && exit 1)
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=$(CGO_ENABLED) $(GO) build -trimpath -ldflags="$(LDFLAGS)" -o ../$(BUILD_DIR)/$(BINARY_NAME) $(BACKEND_MAIN)
	@echo "嵌入式后端构建完成: $(BUILD_DIR)/$(BINARY_NAME)"

backend-test: ## 运行后端测试
	@echo "运行后端测试..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) test -v ./...

backend-test-race: ## 运行后端测试（竞态检测）
	@echo "运行后端测试（竞态检测）..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) test -race -v ./...

backend-test-cover: ## 运行后端测试（覆盖率分析）
	@echo "运行后端测试（覆盖率分析）..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) test -coverprofile=coverage.out -covermode=atomic ./...
	@cd $(BACKEND_DIR) && $(GO) tool cover -html=coverage.out -o coverage.html
	@echo "覆盖率报告: backend/coverage.html"

backend-lint: ## 运行后端代码检查
	@echo "运行后端代码检查..."
	@cd $(BACKEND_DIR) && which golangci-lint > /dev/null || (echo "警告: 未安装 golangci-lint" && exit 0)
	@cd $(BACKEND_DIR) && golangci-lint run ./...

backend-fmt: ## 格式化后端代码
	@echo "格式化后端代码..."
	@cd $(BACKEND_DIR) && $(GO) fmt ./...

backend-vet: ## 运行 go vet 静态分析
	@echo "运行 go vet..."
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 $(GO) vet ./...

backend-deps: ## 下载后端依赖
	@echo "下载后端依赖..."
	@cd $(BACKEND_DIR) && $(GO) mod download

backend-deps-tidy: ## 整理后端依赖
	@echo "整理后端依赖..."
	@cd $(BACKEND_DIR) && $(GO) mod tidy

backend-clean: ## 清理后端构建文件
	@echo "清理后端构建文件..."
	@rm -rf $(BUILD_DIR)
	@cd $(BACKEND_DIR) && rm -f coverage.out coverage.html
	@echo "后端清理完成"

##@ 组合命令

frontend-backend-embed: frontend-build-embed backend-build ## 构建前端（嵌入）+ 后端

dev: frontend-dev ## 启动前端开发服务器

dev-backend: backend-dev ## 启动后端开发服务器

dev-all: ## 同时启动前后端开发服务器
	@echo "启动前后端开发服务器..."
	@make -j2 frontend-dev backend-dev-hot

##@ Docker 命令

docker-build: ## 构建 Docker 镜像
	@echo "构建 Docker 镜像..."
	@docker build -t $(DOCKER_IMAGE) .
	@echo "Docker 镜像构建完成: $(DOCKER_IMAGE)"

docker-build-embed: ## 构建嵌入式 Docker 镜像（包含前端）
	@echo "构建嵌入式 Docker 镜像..."
	@make frontend-build-embed
	@docker build -t $(DOCKER_IMAGE) .
	@echo "嵌入式 Docker 镜像构建完成"

docker-up: ## 启动 Docker 容器
	@echo "启动 Docker 容器..."
	@docker compose -f $(DOCKER_COMPOSE_FILE) up -d
	@echo "容器已启动"

docker-down: ## 停止 Docker 容器
	@echo "停止 Docker 容器..."
	@docker compose -f $(DOCKER_COMPOSE_FILE) down
	@echo "容器已停止"

docker-logs: ## 查看 Docker 日志
	@docker compose -f $(DOCKER_COMPOSE_FILE) logs -f

docker-restart: docker-down docker-up ## 重启 Docker 容器

docker-clean: ## 清理 Docker 资源
	@echo "清理 Docker 资源..."
	@docker compose -f $(DOCKER_COMPOSE_FILE) down -v
	@docker system prune -f
	@echo "清理完成"

##@ 清理命令

clean: frontend-clean backend-clean ## 清理所有构建文件
	@echo "所有清理完成"

clean-all: clean ## 清理所有构建文件和依赖
	@echo "完整清理完成"

##@ 数据库命令

db-migrate: ## 运行数据库迁移
	@echo "运行数据库迁移..."
	@cd $(BACKEND_DIR) && $(GO) run $(BACKEND_MAIN) migrate

db-reset: ## 重置数据库
	@echo "警告: 这将删除所有数据！"
	@read -p "确认重置数据库？[y/N] " confirm; \
	if [ "$$confirm" = "y" ] || [ "$$confirm" = "Y" ]; then \
		rm -f cornerstone.db backend/cornerstone.db; \
		echo "数据库已重置"; \
	else \
		echo "已取消"; \
	fi

##@ 质量检查

check-frontend: frontend-lint frontend-type-check ## 前端代码检查

check-backend: backend-fmt backend-vet backend-test ## 后端代码检查

check: check-frontend check-backend ## 前后端代码检查

security-scan: ## 运行安全扫描
	@echo "运行安全扫描..."
	@cd $(BACKEND_DIR) && which gosec > /dev/null || (echo "警告: 未安装 gosec，请运行: go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest" && exit 0)
	@cd $(BACKEND_DIR) && gosec ./...

##@ 快速命令

quick-backend: backend-build ## 快速构建后端

quick-frontend: frontend-build ## 快速构建前端

test: frontend-test backend-test ## 运行所有测试

test-all: frontend-test backend-test-cover ## 运行所有测试（含覆盖率）

##@ 信息命令

info: ## 显示项目信息
	@echo "项目信息:"
	@echo "  项目名称: $(BINARY_NAME)"
	@echo "  版本: $(VERSION)"
	@echo "  前端目录: $(FRONTEND_DIR)"
	@echo "  后端目录: $(BACKEND_DIR)"
	@echo "  构建目录: $(BUILD_DIR)"
	@echo "  Docker 镜像: $(DOCKER_IMAGE)"

version: ## 显示版本信息
	@echo "版本: $(VERSION)"
	@echo "Go 版本:"
	@cd $(BACKEND_DIR) && $(GO) version
	@echo "Node 版本:"
	@$(NODE) --version
	@echo "pnpm 版本:"
	@$(PNPM) --version

deps-tree: ## 显示后端依赖树
	@echo "后端依赖树:"
	@cd $(BACKEND_DIR) && $(GO) mod graph | head -20

deps-why: ## 分析后端依赖关系
	@echo "使用方法: make deps-why PACKAGE=package.name"
	@test -n "$(PACKAGE)" || (echo "错误: 请指定 PACKAGE=package.name" && exit 1)
	@cd $(BACKEND_DIR) && $(GO) mod why $(PACKAGE)

##@ 发布命令

release: clean check frontend-backend-embed ## 发布流程（清理 + 检查 + 嵌入式构建）

release-all: clean check ## 发布所有平台
	@echo "发布所有平台..."
	@make frontend-build-embed
	@mkdir -p $(BUILD_DIR)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-linux-amd64 $(BACKEND_MAIN)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=windows GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-windows-amd64.exe $(BACKEND_MAIN)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-darwin-amd64 $(BACKEND_MAIN)
	@cd $(BACKEND_DIR) && CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 $(GO) build -trimpath -ldflags="$(LDFLAGS)" -tags=sqlite_omit_load_extension -o ../$(BUILD_DIR)/$(BINARY_NAME)-darwin-arm64 $(BACKEND_MAIN)
	@echo "发布完成:"
	@ls -lh $(BUILD_DIR)/$(BINARY_NAME)-*

##@ 开发工具安装

install-tools-backend: ## 安装后端开发工具
	@echo "安装后端开发工具..."
	@$(GO) install github.com/air-verse/air@latest
	@$(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	@$(GO) install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest
	@echo "后端开发工具安装完成"
	@echo "  - air: 热重载"
	@echo "  - golangci-lint: 代码检查"
	@echo "  - gosec: 安全扫描"

install-tools-frontend: ## 安装前端开发工具
	@echo "前端开发工具通过 pnpm 管理"
	@echo "请运行: cd frontend && pnpm install"

install-tools: install-tools-backend install-tools-frontend ## 安装所有开发工具
