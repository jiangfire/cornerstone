[English](CONTRIBUTING.md) | [中文](CONTRIBUTING.zh.md)

# 为 Cornerstone 贡献代码

感谢你对 Cornerstone 项目的关注！

## 入门指南

### 前置要求

- Go 1.26+
- Make（可选，用于便捷命令）
- Docker（可选，用于数据库集成测试）

### 环境搭建

```bash
git clone https://github.com/jiangfire/cornerstone.git
cd cornerstone
cp .env.example .env
make build    # 或: go build -o bin/cornerstone ./cmd
make test     # 运行所有测试
```

## 开发工作流

### 代码风格

- 遵循标准 Go 规范（`gofmt`、`go vet`）
- 使用制表符缩进
- 包名保持小写
- 为新功能添加/更新测试
- 提交前运行 `make check`（fmt + vet + lint + test）

### 提交信息

我们使用 [Conventional Commits](https://www.conventionalcommits.org/)：

```
type(scope): description

[可选正文]

[可选脚注]
```

类型：`feat`、`fix`、`docs`、`test`、`refactor`、`perf`、`chore`

示例：
```
feat(query): 增加 HAVING 子句支持
fix(auth): 修复 token 缓存竞态条件
docs(readme): 更新 API 端点表
test(services): 增加记录批量创建测试
```

### 测试

```bash
# 运行所有测试
go test ./...

# 启用竞态检测运行
make test

# 运行特定包的测试
go test ./internal/services -v

# 运行基准测试
go test ./pkg/query -run ^$ -bench BenchmarkExecutorExecute -benchmem

# 使用特定数据库运行
DB_TYPE=postgres DATABASE_URL="host=localhost ..." go test ./...
```

### Pull Request 流程

1. Fork 本仓库
2. 创建功能分支（`git checkout -b feat/my-feature`）
3. 编写代码并附带测试
4. 运行 `make check` 确保代码质量
5. 使用约定式提交格式提交
6. 推送到你的 Fork 并发起 Pull Request

### PR 检查清单

- [ ] 为新功能添加/更新了测试
- [ ] `make check` 在本地通过
- [ ] 文档已更新（README、docs/、swagger 注解）
- [ ] 没有提交密钥或凭证
- [ ] 提交信息符合约定式格式

## 项目结构

```
cornerstone/
├── cmd/                    # 入口
├── internal/
│   ├── authz/             # 权限系统
│   ├── cli/               # CLI 命令
│   ├── config/            # 配置
│   ├── db/                # 数据库迁移
│   ├── handlers/          # HTTP 处理器
│   ├── mcp/               # MCP 协议实现
│   ├── middleware/        # HTTP 中间件
│   ├── migration/         # 外部数据库迁移
│   ├── models/            # 数据模型
│   ├── services/          # 业务逻辑
│   └── swagger/           # Swagger 类型
├── pkg/                    # 共享包
│   ├── cache/             # 缓存抽象
│   ├── db/                # 数据库连接
│   ├── dto/               # API 响应 DTO
│   ├── jsonx/             # JSON 工具
│   ├── log/               # 日志
│   └── query/             # 查询 DSL 引擎
├── docs/                   # 文档
└── Makefile               # 构建命令
```

## 报告问题

报告 bug 时，请包含以下信息：

- Cornerstone 版本（`cornerstone --version`）
- Go 版本（`go version`）
- 数据库类型和版本
- 复现步骤
- 预期行为与实际行为
- 相关日志（敏感信息请脱敏）

## 安全

如果你发现了安全漏洞，请通过 GitHub Security Advisories 私下报告，而不是公开提交 issue。

## 许可证

通过贡献代码，你同意你的贡献将在 AGPL-3.0 许可证下授权。
