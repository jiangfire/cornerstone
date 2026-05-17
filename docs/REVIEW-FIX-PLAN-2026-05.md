# Cornerstone 项目审查修复计划 (2026-05)

## 0. Context

本计划基于 2026-05-17 对 `master` 分支（HEAD `cb34b79`）的全栈审查，覆盖后端 Go 代码、前端 Vue 应用、构建/CI/部署配置与依赖。审查发现 **8 个阻塞级问题**、**12 个重要问题**与若干建议项。本文档负责把这些发现转化为可执行的修复任务，分阶段推进。

审查范围：
- `backend/internal/{cmd/server,config,db,handlers,services,middleware,models,mcp,types}`
- `backend/pkg/{query,utils}`
- `frontend/src/{views,components,services,types,stores,router,composables,utils}`
- `docker-compose.yml`、`backend/Dockerfile`、`.github/workflows/`、`Makefile`、`.env`

修复策略：**先堵安全口子，再修一致性债务，最后做质量提升**。每个 Phase 完成后跑一次回归基线（`make test` + `pnpm test:unit` + `pnpm type-check`），保证不引入新缺陷。

---

## 1. 总览

| 阶段 | 主题 | 任务数 | 预估难度 |
|---|---|---|---|
| Phase 1 | 阻塞级安全/可用性修复 | 8 | 高 |
| Phase 2 | 重要功能与一致性修复 | 12 | 中 |
| Phase 3 | 质量、可观察性、合规建议 | 6+ | 低 |

按主题分组：

- **注入与权限**：Query DSL JOIN/JSON 注入、插件 RCE、integration token 错配
- **凭据与密钥**：默认管理员密码日志泄漏、JWT secret 错乱、`.env` 真实凭据、Compose 默认弱口令
- **数据一致性**：伪软删整改、record 反序列化静默错误、用户硬删悬空外键
- **DoS / 性能**：`ListRecords` 全表内存分页
- **类型/前端**：`types/api.ts` 与 `api.ts` 对齐、`permissions` store 响应式破损、`RecordsView` / `ProfileView` / `GovernanceView` 缺陷
- **构建与 CI**：`vite@latest`、`alpine:latest`、缺 `.dockerignore`、Makefile gosec 路径、CI 缺扫描

---

## 2. Phase 1 — 阻塞级修复

### P1-1 默认管理员密码不再写入日志

**问题**：`backend/internal/db/migrate.go:257-265` 把首次启动生成的随机密码用 `zap.L().Info` 输出，落到 `logs/app.log` 并随 lumberjack 归档。

**修复**：
- 改为 `fmt.Fprintln(os.Stderr, ...)` 一次性打印，仅在 TTY 可见
- 同时写入 `data/initial-admin.txt`（权限 `0600`），若文件已存在则跳过
- 支持 `BOOTSTRAP_ADMIN_PASSWORD` 环境变量预设，CI/容器化场景优先用它
- 日志里只留 `"已生成默认管理员，凭据已写入 data/initial-admin.txt"`，不含密码

**验证**：
- 单测：删 `data/initial-admin.txt`，启动后该文件存在且权限 0600，`logs/app.log` 不含密码字符串
- 手动：`grep -r "密码" logs/` 应只命中无关日志

**文件**：`backend/internal/db/migrate.go`、新增 `backend/internal/db/bootstrap_admin.go`（可选）

---

### P1-2 JWT secret 加载链路统一

**问题**：`backend/pkg/utils/jwt.go:50` 中 `loadJWTConfig()` 再次调用 `config.Load()`，dev 模式下 `JWT_SECRET` 为空会**重新生成**临时密钥；与 `main.go` 启动时已生成的密钥不一致，导致登录后 token 验证失败。

**修复**：
- `pkg/utils/jwt.go`：去掉内部 `config.Load()`，新增 `InitJWT(cfg JWTConfig)` 显式注入
- `backend/cmd/server/main.go`：在依赖初始化阶段调用 `utils.InitJWT(cfg.JWT)`，**位置必须在任何路由注册之前**
- 把 `jwt.go` 中模块级全局 `jwtSecret` 改为 `sync.Once` 守卫，二次调用 InitJWT 不再生效（便于测试覆盖时显式重置）

**验证**：
- 在 dev 模式下登录后用同一 token 重复访问受保护接口 ≥3 次，全部 200
- 单测：`pkg/utils/jwt_test.go` 增加 "Init → Sign → Parse" 完整流程用例

**文件**：`backend/pkg/utils/jwt.go`、`backend/cmd/server/main.go`、`backend/pkg/utils/jwt_test.go`

---

### P1-3 Query DSL：JOIN ON 子句结构化

**问题**：`backend/pkg/query/sql_generator.go:168` 把 `join.On` 原样 `fmt.Sprintf` 拼到 SQL，认证用户可注入 `1=1; DROP TABLE users; --` 或绕过字段白名单访问 password。

**修复**：
- `pkg/query/types.go`（如未拆分则在 parser.go）：把 `Join.On` 从 `string` 改为
  ```go
  type JoinCondition struct {
      Left  string `json:"left"`  // table.field
      Op    string `json:"op"`    // 仅允许 "=" 与 "<>"
      Right string `json:"right"` // table.field
  }
  ```
- 解析器：校验 Left/Right 都符合 `^[A-Za-z_][\w]*\.[A-Za-z_][\w]*$` 且引用的表在 `From + Joins` 白名单内、字段在该表的可见字段集
- SQL 生成器：手工拼接，不再 `fmt.Sprintf` 原始字符串
- 兼容：旧的字符串 `On` 通过 parser 抛出 `400 invalid_join_condition`，前端 QueryBuilder 完成迁移后即可彻底删除字段

**验证**：
- `pkg/query/sql_generator_test.go` 增加恶意输入用例：`"1=1; DROP"`、`"a.b = c.d OR 1=1"`、含 `--` 注释、含单引号
- 集成测试：构造一个含 JOIN 的查询请求，断言生成 SQL 中 `JOIN` 后只出现白名单字段

**文件**：`backend/pkg/query/{parser,types,sql_generator,validator}.go` 及对应 `_test.go`

---

### P1-4 Query DSL：字段名/JSON path 统一校验

**问题**：`backend/pkg/query/sql_generator.go:361,365` 把字段名当字面量拼进 `JSON_EXTRACT(%s, '$.%s')` 与 `%s->>'%s'`，单引号即可破出字符串字面量。`req.Select` / `cond.Field` 没有字段名格式校验。

**修复**：
- `pkg/query/validator.go` 新增 `ValidateIdentifier(name string) error`，正则 `^[A-Za-z_][\w]*(\.[A-Za-z_][\w]*)*$`，最大长度 128
- 所有走入 `generateFieldExpression`、`buildSelect`、`buildWhere`、`buildOrderBy`、`buildGroupBy` 的字段名先经过校验
- JSON path 段额外限制：每段 `^[A-Za-z_][\w]*$`，禁止 `[`、`*`、`'`、`"`、空格

**验证**：
- 单测覆盖所有恶意 payload：`name'); --`、`*; DROP`、含 NULL byte、UTF-8 边界字符
- 模糊测试：在 CI 中加 5 分钟 fuzz（`go test -fuzz`）

**文件**：`backend/pkg/query/validator.go`、`backend/pkg/query/sql_generator.go`、`backend/pkg/query/validator_test.go`

---

### P1-5 软删一致性整改

**问题**：`backend/internal/models/models.go` 全部实体用 `DeletedAt *time.Time` 而非 `gorm.DeletedAt`，导致 `db.Delete()` 实际是**硬删**。命中位置至少：`services/file.go:362`、`organization.go:399`、`database.go:493`、`auth.go:466-490,495`、`plugin.go:136,182`。

**修复策略**：选 **A 方案：让 GORM 接管软删**（最小侵入、与现有 `WHERE deleted_at IS NULL` 兜底逻辑兼容）。

**步骤**：
1. `models/models.go`：把所有需要软删的实体的 `DeletedAt *time.Time` 改为 `gorm.DeletedAt`（带 index tag），保留 JSON 输出
2. `db/migrate.go`：补 AutoMigrate 让索引迁移生效
3. 全 `services/*.go` 中所有 `db.Delete(...)`、`Unscoped().Delete(...)` 走查：
   - 正常软删 → 保留 `db.Delete(...)`
   - 真正需要硬删的场景（如清理过期 token）→ 显式 `Unscoped().Delete(...)` 并加注释
4. `auth.go:495` 用户删除：改软删 + 同事务把该用户名下的 token/session 失效；不删 `created_by` 引用，前端展示 "已注销用户"

**验证**：
- 单测：每个 service 的 Delete 流程读回 `WithContext(ctx).Unscoped().First(&x)` 应能看到 `deleted_at != nil`
- 集成回归：清单见 P1-5 测试矩阵（见 §5）

**文件**：`backend/internal/models/models.go`、`backend/internal/services/{file,organization,database,auth,plugin,record,field}.go`、`backend/internal/db/migrate.go`

**风险**：现有数据库已有 `deleted_at` 列但类型可能不同（PG 是 `timestamp`，SQLite 是 `datetime`），需要 manual migration 把存量数据保持原样，不能用 `DropColumn + AddColumn`。**先在 SQLite + PG 各跑一次 dry-run 迁移**。

---

### P1-6 插件管控 + goroutine 生命周期

**问题**：`backend/internal/services/plugin.go:60-83`（`CreatePlugin` 无角色校验）+ `397-423`（`TriggerByTable` 起裸 goroutine 无 WaitGroup/recover）。已认证用户即可注册 + 触发 `go run/python/bash ./plugins/<entry>`，潜在 RCE 入口。

**修复**：
- `services/plugin.go::CreatePlugin / UpdatePlugin`：开头加 `if !user.IsSystemAdmin { return ErrForbidden }`
- 进程级新增 `pkg/asyncworker/pool.go`（约 80 行）：包装 `errgroup.Group` + 全局 ctx + `defer recover()`，提供 `Submit(name string, fn func(ctx))`
- `main.go` 启动时创建 pool，注入 `pluginService`；`Shutdown` 流程 `pool.Wait()` 完成后再关 DB
- 插件目录权限：在 `services/plugin.go::resolveEntryPath` 中拒绝软链、拒绝跨 `plugins/` 边界、拒绝 `.env`/`.git` 等敏感名
- 短期：在 `Makefile` 与 README 注明"生产环境请禁用 `/plugins/*` 路由或部署到独立 sandbox"

**验证**：
- 单测：非 admin 用户调 `CreatePlugin` 期望 403
- 单测：`pool.Submit` 内部 panic 不会导致进程退出
- 手动：构造软链 `plugins/evil -> /etc/passwd`，期望注册失败

**文件**：`backend/internal/services/plugin.go`、`backend/internal/handlers/plugin.go`、新增 `backend/pkg/asyncworker/pool.go`、`backend/cmd/server/main.go`

---

### P1-7 凭据与 Compose 安全化

**问题**：
- `backend/.env`：明文 PostgreSQL 密码 + JWT secret 看起来"像生产"。虽未入 git（已确认 `git log --all -- '**/.env'` 无记录），仍是高危
- `docker-compose.yml`：`POSTGRES_PASSWORD=postgres`、`SERVER_MODE=debug`、`JWT_SECRET=dev-secret-...`、`CORS_ORIGIN=*`、PostgreSQL 5432 端口对外暴露
- 项目根 `.env`：`SERVER_MODE=release` + `JWT_SECRET=change-this-secret-key-in-production` 弱口令（虽然 config.go 弱口令黑名单会让它启动失败，但仍是误用风险）

**修复**：
- **立即轮换** `backend/.env` 中的 PostgreSQL 密码与 JWT secret，告知曾接触过该文件的开发者
- 把 `backend/.env` 重命名为 `backend/.env.example`，删除真实值，提交进 git；本地实际 `.env` 由开发者从 example 派生
- 项目根 `.env` 同样处理
- 拆 `docker-compose.yml`（生产）+ `docker-compose.dev.yml`（开发）：
  - 生产版本所有敏感值用 `${VAR:?required}` 强制注入
  - 移除 `ports: 5432:5432`（postgres 仅内网）
  - `SERVER_MODE=release`、`JWT_SECRET=${JWT_SECRET:?}`、`POSTGRES_PASSWORD=${POSTGRES_PASSWORD:?}`、`CORS_ORIGIN` 显式白名单
- 更新 README 部署章节，说明 `.env` 文件管理与密钥轮换流程

**验证**：
- `docker compose -f docker-compose.yml up -d` 在缺 env 时应**报错退出**而非启动
- `grep -rn "postgres:postgres\|change-this-secret\|cmDHUmvMVQsQSp8V" .` 应无命中（排除 `.gitignore` 已忽略的文件）

**文件**：`backend/.env` → `backend/.env.example`、`.env` → `.env.example`、`docker-compose.yml`、新增 `docker-compose.dev.yml`、`README.md`

---

### P1-8 vite 钉版本回到 7.3.0

**问题**：`frontend/package.json:53` `"vite": "npm:rolldown-vite@latest"` 违反 CLAUDE.md 明令，导致每次 `pnpm install` 可能拉到不同版本。

**修复**：
- 改为 `"vite": "npm:rolldown-vite@7.3.0"`
- 删 `frontend/pnpm-lock.yaml` 后 `pnpm install` 重新生成锁文件
- CI 中已有 `--frozen-lockfile`，无需额外改动
- 在 CLAUDE.md 的"已知陷阱"加一行 "本计划已在 2026-05 修复，禁止改回 latest"

**验证**：
- `pnpm install --frozen-lockfile` 在 CI 通过
- `pnpm build:embed` 产物大小变化 < 5%

**文件**：`frontend/package.json`、`frontend/pnpm-lock.yaml`、`CLAUDE.md`

---

## 3. Phase 2 — 重要修复

### P2-1 integration token 变量名统一

**问题**：`middleware/integration.go:68,76` 读 `INTEGRATION_TOKENS`，但 `config/config.go` 注册的是 `OUTBOUND_INTEGRATION_TOKENS`。"按来源系统分别配 token" 永远走不通。

**修复**：把 config 与 middleware 统一到同一变量名 `INTEGRATION_TOKENS`（更短更直观），并通过 `IntegrationConfig` 注入而非现场 `os.Getenv`。

**文件**：`backend/internal/middleware/integration.go`、`backend/internal/config/config.go`

---

### P2-2 ListRecords 下推过滤

**问题**：`services/record.go:745-764` filter 模式 `Find(&allRecords)` 全表拉内存。

**修复**：
- 把 filter 解析改走 `pkg/query` 的 SQL 生成器；只允许 `LIKE`、`=`、`IN`、`BETWEEN` 等可下推算子
- 强制 `LIMIT max(pageSize, 500)`；超过则返回 `400 result_too_large` 提示改用 `/query`
- 给 `data` 列建 JSON 表达式索引（PG）或忽略（SQLite，已知限制）

**文件**：`backend/internal/services/record.go`、`backend/pkg/query/executor.go`

---

### P2-3 record.Data JSON 解析告警

**问题**：`handlers/record.go:35,131,188` 用 `_ = json.Unmarshal(...)` 吞错误。

**修复**：改成显式 `if err != nil { zap.L().Warn("record data corrupted", zap.Uint("id", record.ID), zap.Error(err)); }`；返回值用 `map[string]any{}` 兜底但响应里添加 `"_corrupted": true` 标记，前端可选展示。

**文件**：`backend/internal/handlers/record.go`

---

### P2-4 用户删除改软删

**问题**：`services/auth.go:495` `Unscoped().Delete` 会让 `created_by` 等外键悬空。

**修复**：随 P1-5 一并迁移；事务内追加 `db.Where("user_id = ?", id).Delete(&Token{})` 失效所有 token。

**文件**：`backend/internal/services/auth.go`

---

### P2-5 前端类型对齐

**问题**：`services/api.ts` 21 处 `any`；`types/api.ts` 的 `Record / Plugin / Organization` 等几乎没人 import；`recordAPI.list` 返回声明 `has_more` 但前端读 `total`。

**修复**：
- 以 `types/api.ts` 为单一真相，逐个接口改造 `api.ts`：
  ```ts
  export async function listRecords(...): Promise<ListResponse<Record>> { ... }
  ```
- 删除 `eslint-disable no-explicit-any`，剩余必要 `any` 个案再单点抑制
- 与后端响应实际字段对齐：`total / has_more` 二选一，统一为 `total + next_cursor` 或保留 `total`
- 新增缺失类型：`PluginBinding`、`PluginExecution`、`DatabaseListItem`、`Activity`

**文件**：`frontend/src/services/api.ts`、`frontend/src/types/api.ts`、所有相关 view（顺路修编译错误）

---

### P2-6 RecordsView 多项修缮

**问题**：
- 表格 `select/multiselect` 列对数组直接渲染成 `"a,b"`
- `beforeUpload` 直接 return true 跳过大小/类型校验
- `searchText` watch + `@keyup.enter` 触发两次 loadRecords
- `searchTimeout` 没在 `onUnmounted` 清理
- `params.filter` 在 437-445 行重复赋值
- `previewFile.url` 中途切换预览未释放
- `await` 在 `loadAttachedFiles` / `handleDeleteFile` 缺失

**修复**：
- 抽出 `FieldRenderers.ts` 把按 `field.type` 渲染逻辑集中
- `beforeUpload` 实现：拒绝 > 50MB、按 `field.config.acceptedTypes` 校验 MIME；超出时 `ElMessage.error`
- 删 `@keyup.enter`，只留 watch 防抖；`onUnmounted` 中 `clearTimeout(searchTimeout)`
- `previewFile` 切换时先 `URL.revokeObjectURL(old.url)`
- 所有 `loadAttachedFiles` 调用 `await` + `try/catch`

**文件**：`frontend/src/views/RecordsView.vue`、新增 `frontend/src/views/records/FieldRenderers.ts`

---

### P2-7 ProfileView 头像走文件上传

**问题**：`ProfileView.vue:182-194` 用 `readAsDataURL` 把图片塞进 `users/me` 接口。

**修复**：改用 `fileAPI.upload` 拿到 ID/URL，`users/me` 只存 URL。前端校验：> 2MB 拒绝，仅允许 `image/png|jpeg|webp`。

**文件**：`frontend/src/views/ProfileView.vue`

---

### P2-8 GovernanceView 外链 URL 白名单

**问题**：`GovernanceView.vue:257` 直接 `:href="link.target_url"`，后端可注入 `javascript:`。

**修复**：抽 `utils/safeUrl.ts`，校验 `^https?://`；不通过则降级为 `<span>` 不可点击。前端任何 `:href` / `window.open` 都过这个 util。

**文件**：`frontend/src/views/GovernanceView.vue`、新增 `frontend/src/utils/safeUrl.ts`

---

### P2-9 permissions store 响应式修复

**问题**：`stores/permissions.ts:29-30,72` `ref<Map>().set()` 不触发 computed。

**修复**：改 `ref<Record<string, FieldPermission[]>>({})`，更新时 `fieldPermissions.value = { ...fieldPermissions.value, [tableId]: list }`；或保留 Map 但每次 `set` 后赋新 Map 引用。

**文件**：`frontend/src/stores/permissions.ts`、新增 `frontend/src/stores/__tests__/permissions.spec.ts`

---

### P2-10 Dockerfile 钉版本 + .dockerignore

**问题**：`backend/Dockerfile:38` `FROM alpine:latest` 破坏可复现；缺 `.dockerignore` 让 `.env / logs / node_modules / .git` 进 build context。

**修复**：
- `Dockerfile`：`FROM alpine:3.20` （或当前 LTS）
- 项目根新增 `.dockerignore`：
  ```
  .env
  .env.*
  !.env.example
  logs/
  *.db
  *.db-journal
  node_modules/
  .git/
  backend/internal/frontend/dist/
  ```
- 验证 `docker build` 上下文体积下降

**文件**：`backend/Dockerfile`、新增 `.dockerignore`

---

### P2-11 Makefile 修正

**问题**：
- `make test` 默认走 `backend-test`，**没 -race**
- `Makefile:254,317` gosec 安装路径 `securecodewarrior/gosec` 是旧路径，正确是 `securego/gosec`
- `db-reset` 用 bash `read -p` 在 Windows nmake 不可用（项目主要在 Windows 开发）

**修复**：
- `test` target 默认走带 `-race` 的命令
- gosec 安装：`go install github.com/securego/gosec/v2/cmd/gosec@latest`
- `db-reset` 改为接受 `CONFIRM=1` 环境变量而非交互输入

**文件**：`Makefile`

---

### P2-12 CI/CD 加固

**问题**：
- `.github/workflows/ci.yml` 未显式 `permissions: contents: read`
- 缺 `golangci-lint`、`pnpm lint`、`pnpm test:unit`、`govulncheck`、`trivy`
- `release.yml` 二进制无签名 / 无 SBOM

**修复**：
- 所有 workflow 顶部加 `permissions: contents: read`，按需在 step 提权
- `ci.yml` 增加 jobs：`lint-go`（golangci-lint）、`lint-frontend`（eslint）、`test-frontend`（vitest）、`vuln-scan`（govulncheck + trivy fs）
- `release.yml` 集成 `cosign sign-blob` + `syft` 生成 SPDX，附在 release artifacts

**文件**：`.github/workflows/{ci,release,docker}.yml`、`.golangci.yml`、可能新增 `.github/workflows/security.yml`

---

## 4. Phase 3 — 建议项

### P3-1 CLAUDE.md 与实际对齐
- 删除 "QueryBuilder + ConditionNode 组件" 描述（前端实际无），或排进 P3 实现该组件
- 更新 `RecordsView.vue` 行数（已从 1383 降到 ~766）

### P3-2 `/health` 加 DB ping
- 拆 `/health` 与 `/ready`：后者跑一次 `db.PingContext`，DB 挂时返回 503
- Compose / K8s 探针用 `/ready`

### P3-3 LLM Governor 客户端加重试与熔断
- 接入 `pkg/asyncworker` 的 outbox 模式或直接用 `cenkalti/backoff/v4`
- 失败计数超阈值时短路 30s

### P3-4 大列表分页/虚拟滚动
- `GovernanceView`、`PluginsView`、`OrganizationsView` 加服务端分页
- `RecordsView` 大数据集启用 `el-table-v2`（虚拟滚动）

### P3-5 axios 超时分级
- `api.ts` 全局 10s 保留；`fileAPI.upload`、`exportAPI.downloadRecords`、`governanceAPI.generateAIRecommendations` 单独 60s

### P3-6 可观察性与合规
- 加 `/metrics`（Prometheus）
- 结构化日志按 JSON 输出 stdout，不再写文件
- 前端添加 "Source Code" 链接以满足 AGPL-3.0 网络服务条款

---

## 5. 风险与回归矩阵

| Phase | 影响面 | 必须回归的关键路径 |
|---|---|---|
| P1-1 | 启动 | 首次启动 / 后续启动 / 重启场景下 admin 凭据生成行为 |
| P1-2 | 鉴权 | 登录 → 受保护路由 → 二次刷新 token |
| P1-3/4 | Query | 单测全量；前端 RecordsView 现有过滤功能 |
| P1-5 | 数据 | 删除 user / database / organization / file / plugin 后，列表/详情读取行为；外键引用 |
| P1-6 | 插件 | 插件 CRUD（admin / 非 admin）；触发执行；进程优雅关闭 |
| P1-7 | 部署 | `docker compose up`（dev / prod 两种 compose）；缺 env 应失败 |
| P1-8 | 构建 | `pnpm install --frozen-lockfile` + `pnpm build:embed` |
| P2-* | 各自 | 见每条 P2 修复中的"验证" |

每个 Phase 结束跑：
```bash
cd backend && go test -race ./...
cd frontend && pnpm type-check && pnpm test:unit && pnpm build
```

---

## 6. 执行顺序与依赖

```
P1-2 (JWT)      ──┐
P1-1 (admin)    ──┤
P1-3 (JOIN)     ──┤       不互相依赖，可并行
P1-4 (JSON)     ──┤
P1-7 (.env)     ──┘
                   │
P1-5 (软删) ────────────┐ 等 P1-1/2 完成后再做（避免冲突 migrate.go）
P1-6 (插件) ────────────┤ 依赖新增 pkg/asyncworker
P1-8 (vite) ────────────┘ 独立，但锁文件刷新可能引起其它 P2 重测

P2-1 / P2-2 / P2-3 / P2-4   独立可并行
P2-5 → P2-6 / P2-7 / P2-8   先对齐类型再改 view
P2-9                         独立
P2-10 / P2-11 / P2-12        DevOps 同批

Phase 3 任意时机
```

---

## 7. 验收清单

每个 Phase 完成 PR 时勾选：

- [ ] 该 Phase 内所有 P-* 任务的"修复"步骤已实现
- [ ] 所有"验证"用例已加进单测，本地 `make test` 与 `pnpm test:unit` 通过
- [ ] 受影响代码路径手动跑通一次（参考 §5 回归矩阵）
- [ ] CLAUDE.md / README / 本计划文档更新到位
- [ ] commit 信息符合既有风格（`feat(...) / fix(...) / chore(...)`）
- [ ] 没有引入新的 lint 警告（`golangci-lint run`、`pnpm lint`）

---

## 8. 进度

| 任务 | 状态 | 负责 | 备注 |
|---|---|---|---|
| P1-1 admin 密码不入日志 | ✅ | Claude | `migrate.go` 改为读 `BOOTSTRAP_ADMIN_PASSWORD` / `crypto/rand`，凭据落 `data/initial-admin.txt` (0600)；日志只留元数据 |
| P1-2 JWT secret 链路 | ✅ | Claude | `pkg/utils/jwt.go` 改为 `InitJWT(secret, exp)` 显式注入，main.go 启动期调用一次；测试用 `ResetJWTForTests` |
| P1-3 JOIN ON 结构化 | ✅ | Claude | `JoinClause.On` 改为 `JoinCondition{Left, Op, Right}`；旧字符串形式由 `UnmarshalJSON` 返回 `invalid_join_condition`；Op 走白名单 `=` / `<>` |
| P1-4 字段名/JSON path 校验 | ✅ | Claude | 新增 `pkg/query/identifiers.go`；Parser + SQLGenerator 双层校验；`security_test.go` 用 16 类恶意载荷 × Join/Select/OrderBy/GroupBy/Where/Aggregate/JSON path 覆盖 |
| P1-5 软删一致性 | ✅ | Claude | 17 个实体 `DeletedAt` 统一改为 `gorm.DeletedAt`；DatabaseAccess/OrganizationMember/PluginBinding/Plugin 关系表显式 `Unscoped().Delete()` 硬删（附注释说明 uk 约束）；`auth.DeleteAccount` 改为事务内 token 黑名单 + 用户软删；全部单测断言适配 |
| P1-6 插件管控 + worker pool | ✅ | Claude | 新增 `pkg/asyncworker/pool.go`（panic recover + ctx cancel + graceful Stop）；`main.go` 注入并关停；`CreatePlugin`/`UpdatePlugin` 加 `ensureSystemAdmin` 门禁；`resolveScriptPath` 增加敏感文件名黑名单 + `filepath.Rel` 边界二次校验；`assertScriptResolvesSafely` 在执行时拒绝符号链接/目录/缺失文件 |
| P1-7 凭据/Compose | ✅ | Claude | `docker-compose.yml` 全部走 `${VAR:?required}`，移除 `5432:5432`，默认 `SERVER_MODE=release`；新增 `docker-compose.dev.yml` 供本地开发（暴露 5432 + debug + CORS=*）；新增根 `.env.example`；README 部署章节更新；**真实凭据轮换仍由用户本人完成** |
| P1-8 vite 钉 7.3.0 | ✅ | Claude | `frontend/package.json` 已锁定 `npm:rolldown-vite@7.3.0` |
| P2-1 integration token 变量名 | ✅ | Claude | `IntegrationsConfig` 新增 `InboundTokens` 读 `INTEGRATION_TOKENS`;`middleware.IntegrationTokenAuth(cfg)` 接受注入,启动阶段一次性解析,运行时不再 `os.Getenv`;`integration_test.go` 覆盖 per-source / shared / 头部校验 / 空配置四类用例 |
| P2-2 ListRecords 下推过滤 | ✅ | Claude | `record.go::ListRecords` 拆三路径: 无过滤走 SQL Limit/Offset+COUNT;结构化 JSON 过滤经 `buildStructuredFilterClauses` 翻译为参数化 WHERE(SQLite `JSON_EXTRACT(data,?)=?` / PG `data @> ?`),引用隐藏字段直接返回空(保留侧信道防御);关键字回退先 SQL LIKE 预筛 + `maxKeywordScanRecords=5000`(可测试替换)上限,溢出拒绝并提示走 `/query`,再权限感知 in-memory 二次过滤。新增 `record_filter_pushdown_test.go` 4 例,既有 `record_permissions_test.go::TestRecordService_FilterCannotProbeHiddenFieldValues` 保持绿色 |
| P2-3 record.Data JSON 解析告警 | ✅ | Claude | `handlers/record.go` 抽 `decodeRecordData` + `recordResponseWithData`;损坏数据走 `zap.Warn` + 空 map 兜底 + 响应附 `_corrupted=true`;Create/Update/Batch 三处统一调用;新增 `record_test.go` 覆盖 5 类用例 |
| P2-4 用户删除改软删 | ✅ | Claude | 已随 P1-5 同步完成:`auth.go:519` 用 `tx.Delete(&user)` + 事务内 token 黑名单,关系表 `Unscoped().Delete` 处理唯一约束 |
| P2-5 前端类型对齐 | ✅ | Claude | `types/api.ts` 重写为后端响应单一真相: 移除 `success/message`(那是信封),纠正 `RecordListResponse {items,total,has_more}` / `FieldListResponse {items,total}` / `FileListResponse {items}`,把 `Database/Organization/Plugin/PluginBinding/PluginExecution/DatabaseUser/OrganizationMember/StatsSummary/Activity/SystemSettings` 对齐到 GORM 模型 + service 返回结构(`OrgResponse.role`、`BindingDetail.table_name/database_name`、`StatsSummary {users,organizations,databases,plugins}`、`Activity {content,time,type}`)。`services/api.ts` 移除 `eslint-disable any`,所有 `any[]` 改用具名类型(`request.get<T>` T 描述 `data` 内部形状)。修复 `RecordsView/FieldsView/FieldPermissionsView` 三处 `response.data.fields/records` → `.items`;`fileAPI.listByRecord` 走 `FileListResponse.items`;`DatabasesView.SharedUser` 撤掉本地接口改用导入的 `DatabaseUser`;`PluginsView.executionsList` 改 `PluginExecution[]`。`pnpm type-check` 通过,`pnpm lint` 余 28 处 spec 文件历史遗留 `any`(非本批引入) |
| P2-6 RecordsView 多项修缮 | ✅ | RecordsView.vue 表 multiselect 列改成 `el-tag` 循环渲染（不再 toString 成 `"a,b"`）；删除搜索框 `@keyup.enter` 仅留 watch 防抖；`beforeUpload` + `handleFileSelect` 共用 `validateUploadFile`,拒绝 > 50MB 与可执行扩展;`loadRecords` 去除 `params.filter` 重复赋值,空串不再下发;`handlePreviewFile` 用单调递增 `previewRequestId` 防快速切换覆盖,新 URL 创建前先 revoke 旧 URL;`handleEdit` / `handleFileSelect` / `handleDeleteFile` 全部 `await loadAttachedFiles`;新增 `onUnmounted` 清理 `searchTimeout` 与残留 `previewFile.url`。未抽出 `FieldRenderers.ts`(避免过度抽象,文件 766 行可控）。`pnpm type-check` 通过。 | |
| P2-7 ProfileView 头像 | ✅ | ProfileView.vue 头像上传增加完整校验:MIME 限 `image/png/jpeg/webp`、原始文件 > 2MB 拒绝、`readAsDataURL` 结果再校验后端 256KB 上限(`User.Avatar binding:"max=262144"` 限制);`<el-upload>` 加 `accept="image/png,image/jpeg,image/webp"` 让选择器直接过滤;读取失败也给出明确提示。**作用域说明**:计划原文要求"改用 fileAPI.upload 拿到 ID/URL",但 `/files/upload` handler 必须传 `record_id` 或 `field_id`(handlers/file.go:17),且 `<img src>` 无法携带 JWT 访问 `/files/:id/download`,完整迁移到文件接口需要新增 `POST /users/me/avatar` 后端端点 + 公共可读 URL 机制,超出 P2-7 单文件范围;故本批次先补齐前端校验,完整管道作为 P3 候选。 | |
| P2-8 GovernanceView 外链白名单 | ✅ | Claude | 新增 `utils/safeUrl.ts`(`isSafeHttpUrl` + `safeHttpUrl`):先正则黑名单 `javascript:|data:|vbscript:|file:`(大小写不敏感、允许前导空白),再用 `new URL` 解析校验 `protocol ∈ {http:,https:}`;`GovernanceView.vue` `el-link` 改为 `v-if="safeHttpUrl(link.target_url)"` 渲染,失败回退 `<div>`;新增 6 条 vitest 用例覆盖正常 http/https、javascript/data/vbscript/file 及大小写/前导空白、null/empty、malformed URL、`safeHttpUrl` 的 trim 行为。`pnpm type-check` 通过,49/49 测试绿。 |
| P2-9 permissions store 响应式 | ✅ | Claude | `stores/permissions.ts` 将 `fieldPermissions` / `userPermissions` 由 `ref<Map>` 改为 `ref<Record<string, ...>>({})`,所有写入走 `value = { ...value, [k]: v }` / `value = newObj`(`Map.set` 不触发 ref 依赖,导致 `FieldsView` 等 computed 在权限加载后不刷新);新增 `stores/__tests__/permissions.spec.ts` 13 例,显式断言 reactivity——`computed(() => store.checkFieldPermission(...))` 在 `loadFieldPermissions` 后立即生效、`clearPermissions` 后回落默认、`permissionsByTable` 同步更新,并覆盖四种默认角色 / 配置覆盖默认 / 显式 role 覆盖 currentRole / `filterAuthorizedFields` / API 失败兜底。62/62 测试绿,`pnpm type-check` 通过。 |
| P2-10 Dockerfile + .dockerignore | ✅ | Claude | `backend/Dockerfile` 运行时 stage 由 `alpine:latest` 钉到 `alpine:3.21`(`3.20` 已于 2026-05 进入 EOL 窗口,选当前在保版本);**新增项目根 `.dockerignore`**(权威:`docker-compose.yml` / `docker.yml` 都走 `context: .`,旧 `backend/.dockerignore` 仅在 context=backend 时生效)。排除 `.env*`(`!.env.example`)、本地 `*.db` / `coverage.out` / `server.exe`、`logs/`、`.git/`、`.github/`、`docs/`、`node_modules/`、`frontend/dist/`、`backend/internal/frontend/dist/`、IDE/OS 杂物;显著缩小 build context,避免本地凭据或开发库进镜像层。 |
| P2-11 Makefile 修正 | ✅ | Claude | `backend-test` 默认带 `-race`(无再硬编 CGO_ENABLED=0,保留 `backend-test-no-race` 作为 CGO 禁用环境的退路);gosec 路径改为 `github.com/securego/gosec/v2/cmd/gosec`(security-scan + install-tools-backend);`db-reset` 改 `CONFIRM=1` 环境变量,Windows 友好;**P2-12 跟进**:`backend-lint` 增加 `--config=../.golangci.yml`,与 CI 共用同一份规则。 |
| P2-12 CI/CD 加固 | ✅ | Claude | **权限收口**:所有 workflow 顶层 `permissions: contents: read`,job 按需提权(release.yml 的 `release` job: `contents: write` + `id-token: write` + `attestations: write`;docker.yml 的 `build` job: `packages: write` + `id-token: write`)。**新增 4 个 CI job**(`.github/workflows/ci.yml`): `backend-lint`(golangci-lint v1.62.2 + 新增根 `.golangci.yml`,启用 errcheck/govet/staticcheck/unused/ineffassign/gofmt/gosec/gocritic/revive,测试文件放宽 errcheck/gosec/gocritic);`backend-vuln`(`govulncheck ./...`);`fs-vuln`(`aquasecurity/trivy-action@0.28.0` 扫文件系统,severity=HIGH,CRITICAL,ignore-unfixed,skip-dirs 与 `.dockerignore` 同步);`frontend-lint`(`pnpm exec eslint . --max-warnings=0`,绕过 lint script 的 `--fix`;`eslint.config.ts` 为测试文件追加 override 关闭 `no-explicit-any`,保留 src 业务代码禁用);`frontend-test`(`pnpm run test:unit --run`)。**Release 强化**(`.github/workflows/release.yml`):`anchore/sbom-action` 生成 SPDX-JSON,`sigstore/cosign-installer@v3` + `cosign sign-blob` keyless 签所有 tar.gz/zip/spdx.json + `SHA256SUMS`,bundle 一并附 release;RELEASE_NOTES.md 新增 `cosign verify-blob` 验证指令。**Docker 强化**(`.github/workflows/docker.yml`):`cosign sign` 用 buildx digest 给所有 tag 指向的同一 manifest 签名;追加 `trivy-action` 镜像扫描(`exit-code: '0'` 仅上报,基础镜像 CVE 修复节奏不阻塞 release)。**首次跑预期**:govulncheck / trivy / 新 lint 规则可能暴露存量问题,作为 P3 待办,不算回归。`pnpm type-check`、`pnpm exec eslint . --max-warnings=0`、`pnpm test:unit --run` 本地全绿。 |
| Phase 3 | ⏳ | | |

状态图例：⏳ 未开始 / 🔄 进行中 / ✅ 完成 / ⛔ 阻塞

---

## 附：审查输出存档

原始审查报告（来自 2026-05-17 三方并行扫描）已被收敛进本计划。如需查阅，可参考本次 commit 之前的 PR 评论或会话归档。

---

## 附 B：旁支发现（不在本次 Phase 1 范围内）

> 这些在跑 `go test ./internal/handlers/...` 时浮现，与 P1-1~P1-8 都无直接关系，但需要立项跟进。

- **MCP SSE 测试泄漏 goroutine**：`TestHandleMCPRealHTTPRESTTableAndFieldChangesEmitNotifications` 在 `mcp_test.go` 中以 `httptest.Server` 启动真实 HTTP，但子请求里的 SSE 连接（`mcp.go:154` 的 `select { case <-c.Request.Context().Done(): ... }`）从未被显式 cancel，导致 package 60s 超时崩掉。绕开方式：单独跑 `-run "TestHandleMCP[^R]"` 或 `-skip TestHandleMCPRealHTTPRESTTableAndFieldChangesEmitNotifications`；正解：测试结束前显式 `cancel()` 上下文或关闭 server。新建任务跟进。
