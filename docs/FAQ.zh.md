[English](FAQ.md) | [中文](FAQ.zh.md)

# 常见问题与故障排查

## 通用

### 问：Cornerstone 启动后没有生成主令牌

**答：** 主令牌在首次启动时会自动生成并打印到控制台。如果你没有看到它：

1. 检查日志输出，搜索 `master token` 或 `token`
2. 如果数据库已初始化但令牌表为空，重启服务会重新生成
3. 或者通过环境变量预设：`MASTER_TOKEN=cs_your_custom_token`

### 问：如何查看当前主令牌

**答：**

```bash
cornerstone token list    # 第一个是主令牌
cornerstone db create test  # 如果无需主令牌就能创建，说明当前就是主令牌
```

### 问：支持哪些数据库

**答：** SQLite（默认，零配置）、MySQL 8.0+、PostgreSQL。推荐：

- **开发/测试**：SQLite
- **生产环境（大量 JSON 查询）**：PostgreSQL
- **生产环境（MySQL 生态）**：MySQL 8.0+

---

## 认证

### 问：令牌过期了怎么办

**答：**

1. 检查令牌是否已过期（`expires_at` 字段）
2. 使用主令牌重新创建
3. 如果你忘记了主令牌，检查服务启动日志或在数据库 `tokens` 表中查找 `is_master = true` 的记录

### 问："permission denied: cannot access this database"（权限不足：无法访问此数据库）

**答：**

1. 确认令牌的 Scope 包含目标数据库：`scopes.databases.db_xxx = "viewer"`
2. 确认数据库存在且未被删除
3. 主令牌拥有完整权限；你可以临时使用它来排查问题

### 问：Scope 的格式是什么

**答：** JSON 对象，示例：

```json
{"databases":{"db_xxx":"editor"},"tables":{"tbl_yyy":{"role":"viewer"}}}
```

详见 [Token Scopes](TokenScopes.md)。

---

## 查询 DSL

### 问：Query DSL 查询没有返回数据

**答：**

1. 检查令牌是否有权限访问目标表
2. 检查 `from` 表名是否在允许的列表中：`records`、`tables`、`databases`、`fields`、`files`、`tokens`
3. 检查查询条件是否正确，尤其是 `data.xxx` 路径
4. 使用 `/api/v1/query/explain` 查看执行计划

### 问：JSON 字段查询很慢

**答：**

- **SQLite**：JSON 查询性能天生受限；考虑减少数据量或添加索引
- **PostgreSQL**：使用 `data->>status` 语法；PostgreSQL 会自动优化
- **MySQL**：考虑使用生成列 + 索引。详见 [README](README.md) 的性能部分。

### 问：JOIN 查询返回 "invalid JOIN operator" 错误

**答：** JOIN 的 `on` 条件只允许 `=` 和 `<>` 运算符；不支持 `>`、`<`、`like` 等。

---

## MCP / AI

### 问：Claude Desktop 无法连接

**答：**

1. 确认 Cornerstone 服务正在运行且端口可访问
2. 检查令牌是否有效
3. 确认 `claude_desktop_config.json` 中的 URL 和 Token 正确
4. 查看 Claude Desktop 的 MCP 日志（Developer -> MCP Logs）

### 问：AI 助手返回 "AI agent not configured"

**答：** 在 `.env` 中配置 `LLM_API_KEY`，然后重启服务。

### 问：MCP 工具调用失败

**答：**

1. 检查令牌是否有足够权限（Scope）
2. 检查参数格式是否正确
3. 查看 Cornerstone 服务器日志获取详细错误

---

## 数据迁移

### 问：迁移中断后如何恢复

**答：**

```bash
cornerstone migration run --config ./migration.yaml --resume mig_xxx
```

状态文件保存在 `~/.cornerstone/migrations/`。

### 问：迁移后数据不一致

**答：**

1. 使用 `migration run --validate` 进行验证
2. 检查 `type_mapping_warnings` 中是否有未处理的类型映射
3. 检查源数据库和目标数据库的时区设置

---

## 性能

### 问：记录列表查询很慢

**答：**

1. 确认数据库索引已创建：检查是否存在 `idx_records_table_deleted_created`
2. MySQL 用户：确认查询使用了正确的执行计划（`EXPLAIN`）
3. 减小 `size` 参数（页面大小）
4. 使用数据库级字段投影（只在 `select` 中返回需要的字段）

### 问：如何清除缓存

**答：**

```bash
cornerstone cache clear
```

这会清除所有内存/Redis 缓存。通常在修改令牌 Scope 或权限后使用。

---

## Docker

### 问：Docker 启动后数据库连接失败

**答：**

1. 确认 `.env` 中的数据库连接字符串正确
2. 如果使用 Docker Compose，确认服务启动顺序（`depends_on`）
3. PostgreSQL/MySQL 用户：确认数据库已初始化且用户有权限

### 问：容器重启后上传的文件丢失

**答：** 确保 `uploads` 目录已挂载为持久化卷：

```yaml
volumes:
  - ./uploads:/app/uploads
```

---

## 开发

### 问：测试失败，提示 "database is locked"

**答：** SQLite 并发问题。运行测试时：

```bash
go test ./... -p 1    # 串行运行
```

或者使用 PostgreSQL/MySQL 进行测试。

### 问：Swagger 文档没有更新

**答：** 修改处理器注解后重新生成：

```bash
make swagger
```

---

## 仍有问题？

- 查看 [GitHub Issues](https://github.com/jiangfire/cornerstone/issues)
- 查看 [Architecture](Architecture.md) 了解系统组件
- 启用 `LOG_LEVEL=debug` 获取详细日志
