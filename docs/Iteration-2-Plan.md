# 迭代 2 计划：字段选择与 YAML 导入

## 目标

1. CRUD 端点支持字段选择（select fields）
2. 支持 YAML 导入数据库/表/字段，并提供模板下载接口

---

## 1. Query / ListRecords / GetRecord 支持字段选择

### 现状
- Query DSL 端点（`POST /api/v1/query`）已原生支持 `select` 字段，可精确控制返回列。
- 但标准 CRUD 端点 `GET /api/v1/records`、`GET /api/v1/records/:id` 始终返回完整 `data` JSON，不支持子集选择。

### 实施方案

#### API 设计
- `GET /api/v1/records?table_id=xxx&fields=name,status`
- `GET /api/v1/records/:id?fields=name,status`

#### Service 层过滤
- 在 `RecordService.ListRecords` 和 `GetRecord` 中：
  1. 解析 `fields` 参数（逗号分隔）。
  2. 若指定了 `fields`，在 `filterReadableData` 之后，进一步把 `data` map 过滤为仅保留指定键。
  3. 系统字段 `id`、`table_id`、`version` 始终保留在响应中（不在 `fields` 控制范围内）。
- **向后兼容**：`fields` 为空时行为不变。

#### 涉及文件
- `internal/handlers/record.go`
- `internal/services/record.go`

---

## 2. YAML 导入与模板下载

### 现状
- 已有 `POST /api/v1/databases/with-tables`（JSON 批量创建），但没有 YAML 支持，也没有模板下载。

### 实施方案

#### YAML 结构定义
与 `CreateDBWithTablesRequest` 同构：
```yaml
name: "My App"
description: "App database"
tables:
  - name: "users"
    description: "User accounts"
    fields:
      - name: "title"
        type: "string"
        description: "The record title"
        required: false
```

#### 新增接口
- `POST /api/v1/databases/import/yaml`
  - Content-Type: `application/x-yaml` 或 `text/yaml`
  - 接收 YAML body，解析为 `CreateDBWithTablesRequest`，复用现有 `CreateDatabaseWithTables` service 方法。
- `GET /api/v1/databases/import/template`
  - 返回带注释的 YAML 模板
  - HTTP Header: `Content-Type: application/x-yaml`
  - 支持浏览器直接下载

#### CLI 支持
- `cornerstone db import --file schema.yaml`

#### 依赖
- `gopkg.in/yaml.v3`（或 `sigs.k8s.io/yaml`，优先 `gopkg.in/yaml.v3`）

#### 涉及文件
- `internal/handlers/database.go`
- `internal/services/database.go`
- `internal/cli/db.go`

---

## 验证清单

- [ ] `make test` 通过
- [ ] `make fmt` / `make vet` / `make lint` 无新增警告
- [ ] `make swagger` 后文档更新无报错
- [ ] 手工验证：`GET /api/v1/records?table_id=xxx&fields=a,b` 仅返回指定字段
- [ ] 手工验证：`GET /api/v1/records/:id?fields=a,b` 仅返回指定字段
- [ ] 手工验证：YAML 导入成功创建数据库、表、字段
- [ ] 手工验证：模板下载接口返回正确 YAML 格式
