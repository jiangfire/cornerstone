# 迭代 1 计划与实现总结

## 迭代目标

完成 Bug 修复与体验优化，包括：
1. 隐藏 API 响应中的时间戳字段并修复零值问题
2. CLI 命令输出降噪
3. Swagger 在线文档页面
4. 创建库/表/字段时支持使用名称（而非仅 ID）

---

## 1. 隐藏时间戳字段并修复零值

### 问题
- `CreateRecord`、`CreateDatabase`、`CreateTable`、`CreateField`、`BatchCreateRecords` 等创建操作返回的 JSON 中 `created_at` / `updated_at` 显示为 `"0001-01-01T00:00:00Z"`（Go `time.Time` 零值）。
- 数据库存储正确，但 GORM `Create` 后未将数据库生成的默认值回写到 Go model 中。

### 实现
- **统一移除**：从所有 API 响应中删除 `created_at`、`updated_at`、`deleted_at` 三个字段。
  - 修改 `internal/swagger/models.go`：从 `RecordObject`、`DatabaseObject`、`TableObject`、`FieldObject`、`FileObject`、`TokenObject`、`TokenCreateResponse` 中删除时间戳字段。
  - 修改 `internal/services/*` 中的 Response struct：`RecordResponse`、`DBResponse`、`TableResponse`、`FieldResponse`、`FileResponse`。
  - 修改所有 Handler（`record.go`、`database.go`、`table.go`、`field.go`、`token.go`）：`gin.H` 中不再返回时间戳字段。
  - 修改 `internal/services/record.go` 的 `ExportRecords`：CSV/JSON 导出不再包含时间戳列。
- **修复零值副作用**：在 `CreateDatabase`、`CreateTable`、`CreateField`、`CreateRecord` 返回前执行 `s.db.First(&entity, "id = ?", entity.ID)` 重新加载，确保内存 model 中的时间戳被正确填充（供内部逻辑使用）。

### 测试调整
- 删除测试中对已移除时间戳字段的断言：
  - `internal/services/field_service_full_test.go`
  - `internal/services/table_service_test.go`
  - `internal/services/perf_benchmark_test.go`

---

## 2. CLI 命令输出降噪

### 问题
- `ensureDB()` 初始化 logger 时使用配置文件中的日志级别（通常为 info/debug），导致每条 CLI 命令都会打印数据库初始化、迁移、连接等日志，污染 stdout。

### 实现
- 在 `internal/cli/db.go` 的 `ensureDB()` 中，当未设置 `--json` 时，默认将日志级别设为 `fatal`：
  ```go
  if !jsonOutput {
      cfg.Logger.Level = "fatal"
  }
  ```
- 错误信息仍然通过 `return err` 传递，最终由 `Execute()` 输出到 `stderr`，用户不会错过真正的错误。

---

## 3. Swagger 在线文档页面

### 问题
- `make swagger` 虽然生成了 `internal/swagger/docs.go` 和 `swagger.json/swagger.yaml`，但路由表未注册 Swagger UI 端点，无法通过浏览器访问在线文档。

### 实现
- 引入依赖：`github.com/swaggo/gin-swagger`、`github.com/swaggo/files`。
- 在 `internal/cli/serve.go` 中：
  - 添加 blank import `_ "github.com/jiangfire/cornerstone/internal/swagger"`（附带注释说明用途）。
  - 注册路由：`r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))`。
- 重新执行 `make swagger` 更新文档。

---

## 4. 名称引用支持（ID 或名称）

### 问题
- CLI 和 API 强制要求传入 `database_id`、`table_id`，用户难以记忆长 ID。

### 实现

#### Service 层解析辅助函数
- `DatabaseService.ResolveDatabase(identifier string)`：先按 ID 查找，失败再按 `name` 查找。
- `TableService.resolveTable(identifier string)`：先按 ID 查找，失败再按 `name` 查找。
- `TableService.ResolveTable(databaseIdentifier, tableIdentifier string)`：先解析 database，再解析 table（用于需要同时指定库和表的场景）。

#### API 请求体兼容
- `TableCreateRequest.DatabaseID`：语义扩展为"数据库 ID 或名称"，在 `TableService.CreateTable` 入口处调用 `ResolveDatabase` 解析。
- `FieldCreateRequest.TableID`：语义扩展为"表 ID 或名称"，在 `FieldService.CreateField` 入口处调用 `resolveTable` 解析。

#### 现有 CRUD 操作支持名称
- `DatabaseService.GetDatabase` / `UpdateDatabase` / `DeleteDatabase`
- `TableService.ListTables` / `GetTable` / `UpdateTable` / `DeleteTable`
- 以上方法均在入口处调用对应的 Resolve 函数，权限检查使用解析后的真实 ID。

#### CLI 体验优化
- `db get/update/delete` 的 `Use` 提示从 `[id]` 改为 `[id-or-name]`。
- `table create/list/get/update/delete` 的 `Use` 提示改为 `[database-id-or-name]` / `[id-or-name]`。
- `field create/list` 的 `Use` 提示改为 `[table-id-or-name]`。

### 已知边界
- `GetField` / `UpdateField` / `DeleteField` 仍仅支持 field ID，因为 field name 需要 table 上下文才能唯一确定。
- `resolveTable` 按名称查找时，若多个数据库中存在同名表，会返回第一个匹配项（实际场景中表名在同一数据库内唯一，跨数据库同名较少见）。

---

## 验证结果

```bash
$ make fmt   # 通过
$ make vet   # 通过
$ make lint  # 通过（0 issues）
$ make test  # 全部通过
```

---

## 涉及文件清单

### Swagger 模型
- `internal/swagger/models.go`

### Handlers
- `internal/handlers/record.go`
- `internal/handlers/database.go`
- `internal/handlers/table.go`
- `internal/handlers/field.go`
- `internal/handlers/token.go`
- `internal/handlers/file.go`

### Services
- `internal/services/record.go`
- `internal/services/database.go`
- `internal/services/table.go`
- `internal/services/field.go`
- `internal/services/file.go`

### CLI
- `internal/cli/db.go`
- `internal/cli/table.go`
- `internal/cli/field.go`
- `internal/cli/serve.go`

### 测试
- `internal/services/field_service_full_test.go`
- `internal/services/table_service_test.go`
- `internal/services/perf_benchmark_test.go`

### 生成的文档
- `internal/swagger/docs.go`
- `internal/swagger/swagger.json`
- `internal/swagger/swagger.yaml`
