# Cornerstone Performance Plan 2026-06

## 背景与目标
> **背景**：Cornerstone 后续要作为数据源对外提供持续读写能力，当前代码在本地 SQLite 基线上已经暴露出权限判定重复查库、字段权限 N+1、记录列表排序未被索引覆盖、JSON 查询依赖逐行解析等问题。
>
> **目标**：先固定一套可复现的本地性能基线，再按影响优先级逐步优化核心热点路径，在不破坏现有行为和权限语义的前提下，降低高频读接口的延迟、分配和数据库扫描成本。

## 方案概览

| 维度 | 内容 |
|---|---|
| 适用对象 | `serve` 模式下的 HTTP API、Query DSL、后续数据源接入链路 |
| 核心诉求 | 降低高频读请求延迟，控制内存分配，避免 SQLite/PostgreSQL 在数据放大后出现明显退化 |
| 关键约束 | 现有权限模型不能被破坏；优先做根因修复；每一步都要能被 benchmark / pprof / 查询计划验证 |
| 本次文档范围 | 只定义计划、基线、实施顺序、验收标准；暂不在本轮文档确认前改动生产逻辑 |

## 当前基线与证据

### 已完成的本地性能基线准备

| 项目 | 状态 | 说明 |
|---|---|---|
| Benchmark fixture | 已完成 | 新增本地 SQLite 灌数辅助，用于 benchmark、pprof、查询计划复用 |
| Service benchmark | 已完成 | 覆盖 `RecordService.ListRecords`、`FieldService.ListFields` |
| Query benchmark | 已完成 | 覆盖 `Query Executor` 的常规记录查询和 JSON 过滤查询 |
| Auth benchmark | 已完成 | 覆盖 `validateToken` 的认证入口固定成本 |
| SQLite 查询计划 | 已完成 | 对记录列表 SQL 跑通 `EXPLAIN QUERY PLAN` |

### 当前基线结果

| 路径 | 基线结果 | 观察 |
|---|---|---|
| `BenchmarkValidateToken` | `35521 ns/op`, `4709 B/op`, `89 allocs/op` | 单次不算极慢，但会对每个受保护请求收一次固定税 |
| `BenchmarkFieldServiceListFields` | `2851033 ns/op`, `586528 B/op`, `10000 allocs/op` | 字段权限判定存在明显 N+1，且分配很高 |
| `BenchmarkRecordServiceListRecords/no_filter` | `5364924 ns/op`, `673196 B/op`, `9903 allocs/op` | 无过滤列表也有较高权限和 JSON 处理成本 |
| `BenchmarkRecordServiceListRecords/structured_filter` | `23053384 ns/op`, `679566 B/op`, `10012 allocs/op` | 结构化过滤开销显著放大 |
| `BenchmarkExecutorExecute/records_by_table` | `4632147 ns/op`, `386916 B/op`, `5240 allocs/op` | 常规 Query DSL 查询已具备一定固定开销 |
| `BenchmarkExecutorExecute/records_json_filter` | `14019920 ns/op`, `160325 B/op`, `2568 allocs/op` | JSON 过滤明显慢于普通过滤 |

### 当前查询计划结论

```text
SEARCH records USING INDEX idx_records_table_id (table_id=?)
USE TEMP B-TREE FOR ORDER BY
```

结论：

- `records` 列表查询已命中 `table_id` 索引。
- `ORDER BY created_at DESC` 仍需临时排序。
- 当前单列索引不足以覆盖 `WHERE table_id = ? AND deleted_at IS NULL ORDER BY created_at DESC` 的主路径。

### 当前 pprof 结论

| 类型 | 主要热点 | 结论 |
|---|---|---|
| Query CPU | `sqlite jsonParseValue`, `jsonExtractFunc`, `Rows.Next` | SQLite 对 JSON 过滤主要消耗在逐行 JSON 解析与扫描 |
| Query Memory | `Executor.scanRows`, `SQLGenerator.generateWhere`, `NewAuthorizer` | 结果扫描、SQL 拼装和权限过滤存在明显分配成本 |
| Field CPU | `CanAccessField`, `lookupField`, `gorm Scan` | 字段权限判定是最明确的 N+1 根因 |
| Field Memory | `lookupField`, `gorm Scan`, `Statement.AddClause` | 每字段一次查询导致大量对象创建和 GORM 语句构建 |

## 问题清单与优先级

| 编号 | 问题 | 影响范围 | 优先级 | 根因定位 |
|---|---|---|---|---|
| P1 | 字段列表权限检查 N+1 | 字段列表、记录读写前的权限映射 | 高 | `FieldService.ListFields -> CheckFieldPermission -> CanAccessField -> lookupField` |
| P2 | 记录列表复合索引缺失 | 记录列表、分页、导出、Query DSL 记录查询 | 高 | `records` 只有 `table_id` 单列索引 |
| P3 | 认证入口未复用权限缓存 | 全部受保护请求 | 高 | `middleware.Auth -> validateToken` 每请求查一次 `tokens` |
| P4 | Query/Record 路径权限和字段解析重复 | 记录列表、字段权限、Query DSL | 中 | 同一请求内重复构建 `Authorizer`、重复查可访问范围 |
| P5 | SQLite JSON 过滤逐行解析 | Query DSL JSON 条件、记录结构化过滤 | 中 | `JSON_EXTRACT` / JSON 文本过滤无法利用结构化索引 |
| P6 | 导出与关键词过滤存在全量读倾向 | 导出、模糊过滤 | 中 | 先查后内存过滤，结果集大时会放大内存占用 |
| P7 | 请求上下文未真正透传到 Query 执行 | 超时、中断、尾延迟 | 中 | `Executor.executeQuery/executeCount` 未 `WithContext(ctx)` |
| P8 | 表访问权限动作映射存在语义问题 | 读接口权限准确性 | 中 | `checkTableAccess` 根据角色列表内容而不是最低动作需求选权限 |

## 备选方案对比

| 方案 | 核心思路 | 优点 | 风险/成本 | 适用场景 | 结论 |
|---|---|---|---|---|---|
| 方案A | 先做热点根因修复，保持现有模型和 JSON 存储结构 | 风险可控，改动渐进，可持续复测 | 需要多轮迭代，短期内不能一次性消灭全部热点 | 当前阶段，需要稳定迭代 | 推荐 |
| 方案B | 直接重构为专门的物化列/派生表/查询缓存体系 | 理论性能上限更高 | 改动大，权限和一致性风险高，超出当前需求 | 数据量已远超单机中小规模 | 备选 |
| 方案C | 先不改代码，只依赖更换数据库或调大资源 | 实施快 | 根因不变，成本上升，SQLite 问题仍会暴露 | 临时缓冲 | 不推荐 |

## 推荐方案与落地步骤
> **推荐结论**：采用“基线固定 -> 根因修复 -> 每步复测 -> 再决定是否追加结构性优化”的渐进方案，先解决权限 N+1、复合索引和认证固定税，再评估 JSON 查询的进一步优化是否需要引入派生列或特殊索引策略。

### 阶段 0：固定基线与测试工具

| 阶段 | 动作 | 输出物 | 验收标准 |
|---|---|---|---|
| 0.1 | 保留并整理本地 benchmark / pprof / explain 入口 | 基准测试文件、命令清单 | 能稳定复现当前基线结果 |
| 0.2 | 补充文档化执行说明 | 本文档 + 命令列表 | 新成员能按文档独立跑出基线 |

### 阶段 1：权限链路去重与 N+1 修复

| 阶段 | 动作 | 输出物 | 验收标准 |
|---|---|---|---|
| 1.1 | 改造 `FieldService.ListFields`，批量做字段权限判定，移除逐字段 `CheckFieldPermission` | 代码改动 + 单测 | `BenchmarkFieldServiceListFields` 明显下降，alloc 明显下降 |
| 1.2 | 复用 `Authorizer` 或请求级权限上下文，减少同一请求重复构建 | 代码改动 + 回归测试 | 记录列表和 Query DSL 的 alloc 下降 |
| 1.3 | 评估并改造 `middleware.Auth`，减少每请求对 `tokens` 表的固定查询 | 代码改动 + benchmark | `BenchmarkValidateToken` 或认证整体路径开销下降 |
| 1.4 | 修正 `checkTableAccess` 的动作映射语义问题 | 代码改动 + 权限测试 | 读接口不再错误抬高到 `manage` 权限 |

### 阶段 2：索引与查询路径优化

| 阶段 | 动作 | 输出物 | 验收标准 |
|---|---|---|---|
| 2.1 | 为 `records` 增加覆盖主路径的复合索引，候选为 `(table_id, deleted_at, created_at DESC)` | 迁移逻辑 + explain 验证 | `EXPLAIN QUERY PLAN` 不再出现 `USE TEMP B-TREE FOR ORDER BY`，或显著减少排序代价 |
| 2.2 | 评估 `files`、`fields`、`tables` 是否需要补充复合索引 | 迁移逻辑 + explain | 高频条件组合能命中索引 |
| 2.3 | 对 `COUNT` 与分页查询进行复测 | benchmark 数据 | `ListRecords/no_filter` 和 `records_by_table` 结果下降 |

### 阶段 3：JSON 查询成本控制

| 阶段 | 动作 | 输出物 | 验收标准 |
|---|---|---|---|
| 3.1 | 区分“结构化精确过滤”和“模糊关键字过滤”的优化路径 | 设计说明 + 代码改动 | 精确过滤优先走更窄 SQL 路径 |
| 3.2 | 评估是否为高频 JSON 字段引入派生列或可选索引策略 | 设计文档 | 明确是否需要结构性改造 |
| 3.3 | 继续使用 SQLite 基线验证 JSON 查询收益 | benchmark + pprof | `records_json_filter` 或 `structured_filter` 明显下降 |

### 阶段 4：大结果集与取消传播

| 阶段 | 动作 | 输出物 | 验收标准 |
|---|---|---|---|
| 4.1 | 为导出和关键词过滤增加更明确的限制、分页或流式策略 | 代码改动 + 文档 | 大表导出不再一次性占用过高内存 |
| 4.2 | 在 Query 执行路径使用 `WithContext(ctx)` 透传取消信号 | 代码改动 + 测试 | 中断请求时数据库查询能够及时停止 |

## 实施顺序

1. 先做 **阶段 1.1 + 1.4**：这是当前最明确、收益最高、风险最可控的一组。
2. 再做 **阶段 2.1**：复合索引可以直接改善列表主路径。
3. 接着做 **阶段 1.2 + 1.3**：压缩同请求内的重复权限和认证成本。
4. 之后再进入 **阶段 3**：处理 JSON 过滤的结构性成本。
5. 最后处理 **阶段 4**：大结果集和取消传播。

## 验收指标

### 第一阶段验收指标

| 指标 | 当前基线 | 目标 |
|---|---|---|
| `BenchmarkFieldServiceListFields` | `2.85 ms/op`, `586 KB/op`, `10000 allocs/op` | 至少下降 `40%`，并把 alloc 降到明显低于 `6000` |
| `BenchmarkValidateToken` | `35.5 us/op`, `4.7 KB/op`, `89 allocs/op` | 至少下降 `20%` 或合并到请求级缓存路径 |
| 权限行为一致性 | 存在读权限动作映射问题 | 修正后补齐单测，确保语义清晰 |

### 第二阶段验收指标

| 指标 | 当前基线 | 目标 |
|---|---|---|
| `ListRecords/no_filter` | `5.36 ms/op` | 下降 `20%-35%` |
| `records_by_table` | `4.63 ms/op` | 下降 `15%-30%` |
| 查询计划 | `USE TEMP B-TREE FOR ORDER BY` | 尽量消除，或至少证明复合索引生效 |

### 第三阶段验收指标

| 指标 | 当前基线 | 目标 |
|---|---|---|
| `structured_filter` | `23.05 ms/op` | 下降 `25%-40%` |
| `records_json_filter` | `14.02 ms/op` | 下降 `20%-35%` |
| pprof 热点 | `jsonParseValue/jsonExtractFunc` 占比较高 | 热点占比下降，或明确该成本由 SQLite 本身决定 |

## 验证命令清单

```powershell
go test ./internal/middleware -run ^$ -bench BenchmarkValidateToken -benchmem -count 1
go test ./internal/services -run ^$ -bench BenchmarkFieldServiceListFields -benchmem -count 1
go test ./internal/services -run ^$ -bench BenchmarkRecordServiceListRecords -benchmem -count 1
go test ./pkg/query -run ^$ -bench BenchmarkExecutorExecute -benchmem -count 1
go test ./internal/services -run TestSQLiteExplainPlan_ListRecords -v
go test ./pkg/query -run ^$ -bench "BenchmarkExecutorExecute/records_json_filter$" -benchtime=2s -count 1 -cpuprofile query_cpu.pprof -memprofile query_mem.pprof
go test ./internal/services -run ^$ -bench BenchmarkFieldServiceListFields$ -benchtime=2s -count 1 -cpuprofile field_cpu.pprof -memprofile field_mem.pprof
```

## 风险与配套措施

| 风险 | 影响 | 应对措施 |
|---|---|---|
| 权限优化改坏现有行为 | 读写授权错误 | 每次修改都补权限单测，并保留基准前后的功能回归 |
| 复合索引增加写放大 | 写入略变慢、迁移耗时增加 | 先只加主路径索引，验证收益后再决定是否继续 |
| JSON 查询优化引入结构性改造 | 复杂度上升 | 先做轻量优化，不提前上派生列 |
| 只看 SQLite 基线导致误判 | 与 PostgreSQL/MySQL 行为有差异 | 先以 SQLite 做方向判断，必要时再补 PostgreSQL 二次验证 |

## CI/CD 扩展计划

- SQLite 继续作为本地开发和 PR 前快速回归基线，保证每次优化都有低成本复测入口。
- MySQL（MySQL 数据库）和 PostgreSQL（PostgreSQL 数据库）建议放入 CI/CD（持续集成 / 持续交付）做定期或按需性能回归，不建议作为每次本地开发的强依赖。
- CI/CD 可分两层：
  - 轻量层：在 PR 或 nightly job 中启动 MySQL/PostgreSQL 容器，跑固定 benchmark 子集与关键 `EXPLAIN`，主要验证索引命中和相对趋势。
  - 深度层：在定时任务中跑更长时间的 benchmark / pprof，对跨数据库差异较大的 JSON 查询、分页、排序路径做趋势追踪。
- 第一批建议纳入 CI/CD 的检查项：
  - `BenchmarkRecordServiceListRecords`
  - `BenchmarkExecutorExecute`
  - 关键列表 SQL 的 `EXPLAIN` / `EXPLAIN ANALYZE`
  - 迁移后索引存在性校验
- 这样分层的原因是：SQLite 更适合快速发现代码级回退，MySQL/PostgreSQL 更适合验证执行计划和跨数据库真实表现，两者职责不同，不应混为一套门禁。

### 已落地的 CI/CD 性能入口

| 项目 | 状态 | 说明 |
|---|---|---|
| `.github/workflows/perf.yml` | 已完成 | 新增独立 Performance workflow，不污染现有功能性 CI |
| SQLite perf job | 已完成 | 跑 `auth` / `services` / `query` benchmark，并输出 `EXPLAIN QUERY PLAN` |
| MySQL perf job | 已完成 | 跑同一组 benchmark，并输出 `EXPLAIN ANALYZE` 文本计划 |
| PostgreSQL perf job | 已完成 | 跑同一组 benchmark，并输出 `EXPLAIN ANALYZE` 文本计划 |
| Artifact / Summary | 已完成 | 每个数据库后端都会上传文本产物，并把关键 benchmark 摘要写入 GitHub Actions Summary |

补充说明：

- 本地 benchmark 夹具已改为按 `DB_TYPE` / `DATABASE_URL` 自动切换后端；未设置环境变量时默认使用临时 SQLite 文件。
- 因此开发阶段仍可直接执行原有 benchmark 命令，而 CI/CD 中只需设置数据库环境变量即可复用同一套基准代码。

## 当前实施进展（2026-06-06）

### 已完成改造

| 阶段 | 状态 | 实际改动 |
|---|---|---|
| 1.1 | 已完成 | `FieldService.ListFields` 改为批量 `CheckFieldPermissions`，移除逐字段权限查询 |
| 1.2 | 已完成 | `pkg/query.Validator` 增加单请求 `access scope`，复用 `Authorizer` 与可访问 ID 集合，避免一次 Prepare 内重复取权限范围 |
| 1.3 | 已完成 | `middleware.Auth -> validateToken` 接入 `authz` Token 缓存，避免每请求查一次 `tokens` 表 |
| 1.4 | 已完成 | `FieldService` / `RecordService` 的 `checkTableAccess` 改为按最低所需动作映射权限 |
| 2.1 | 已完成 | `records` 主路径索引调整为 `idx_records_table_deleted_created(table_id, deleted_at, created_at DESC)` |
| 3.x | 持续推进 | `pkg/query.SQLGenerator.GenerateCount` 对简单查询走直接 `COUNT(*)`，避免把完整投影字段和 JSON 表达式包进子查询；`Executor` 复用请求对象时不再累积权限过滤条件；新增 `pkg/jsonx` 作为可回退 JSON 封装，当前仅在 `record` 热路径保留 `sonic` 接入，其余路径按本地 benchmark 结果保留标准库 |
| 4.2 | 已完成 | `pkg/query.Executor.executeQuery/executeCount` 已使用 `WithContext(ctx)`，数据库层可接收取消/超时上下文 |

### 已完成验证

| 项目 | 基线 | 当前结果 | 结论 |
|---|---|---|---|
| `BenchmarkFieldServiceListFields` | `2851033 ns/op`, `586528 B/op`, `10000 allocs/op` | `1101129 ns/op`, `424580 B/op`, `6534 allocs/op` | **明显改善**，耗时约下降 `61.4%` |
| `BenchmarkRecordServiceListRecords/no_filter` | `5364924 ns/op`, `673196 B/op`, `9903 allocs/op` | `1847971 ns/op`, `671328 B/op`, `10008 allocs/op` | **明显改善**，主收益来自复合索引消除排序 |
| `BenchmarkRecordServiceListRecords/structured_filter` | `23053384 ns/op`, `679566 B/op`, `10012 allocs/op` | `8681548 ns/op`, `676782 B/op`, `10119 allocs/op` | **明显改善**，列表主路径排序成本已下降 |
| `BenchmarkValidateToken` | `35521 ns/op`, `4709 B/op`, `89 allocs/op` | `79 ns/op`, `112 B/op`, `1 allocs/op` | **极大改善**，认证固定税基本被缓存消除 |
| `BenchmarkExecutorExecute/records_by_table` | `4632147 ns/op`, `386916 B/op`, `5240 allocs/op` | `1449974 ns/op`, `69785 B/op`, `1466 allocs/op` | **明显改善**，直接 COUNT 路径 + 请求级权限复用收益非常明显 |
| `BenchmarkExecutorExecute/records_json_filter` | `14019920 ns/op`, `160325 B/op`, `2568 allocs/op` | `13214673 ns/op`, `65665 B/op`, `1355 allocs/op` | **中等改善**，JSON 过滤耗时仍主要受 SQLite JSON 解析约束，但固定分配已明显下降 |
| `BenchmarkRecordServiceListRecords/no_filter` | `5364924 ns/op`, `673196 B/op`, `9903 allocs/op` | `1407436 ns/op`, `688022 B/op`, `7115 allocs/op` | **明显改善**，列表主路径已降到 `~1.4 ms`，JSON 序列化/反序列化相关 alloc 进一步下降 |
| `BenchmarkRecordServiceListRecords/structured_filter` | `23053384 ns/op`, `679566 B/op`, `10012 allocs/op` | `8514015 ns/op`, `754092 B/op`, `7202 allocs/op` | **明显改善**，结构化过滤耗时仍受 SQLite JSON 过滤限制，但固定分配显著下降 |
| `BenchmarkFieldServiceListFields` | `2851033 ns/op`, `586528 B/op`, `10000 allocs/op` | `972549 ns/op`, `424063 B/op`, `6533 allocs/op` | **明显改善**，批量权限 + JSON 热路径替换后稳定在 `~1.0 ms` |

补充观察：

- `pkg/query` 路径在本轮主要收益体现在 **alloc / B/op 下降**，尤其是 `records_by_table` 和 `records_json_filter` 的固定分配有所收缩。
- `pkg/query` 还修复了一个功能稳定性问题：复用同一个 `QueryRequest` 多次执行时，原实现会不断叠加权限过滤条件，长跑后可触发 SQLite `Expression tree is too large`；当前已改为在 `Execute/Validate/ExplainAuthorized` 内部克隆请求，避免污染调用方对象。
- 高性能 JSON 库替换的本地结论是：**只在字符串 JSON 的 record 热路径保留收益比较稳定**；对 `pkg/query` 的 JSON 标量扫描和 `field` 配置解析，并没有形成稳定的耗时优势，因此本轮没有继续扩大替换范围。
- 该路径的 **ns/op 抖动仍然较大**，本地 SQLite 单机基准容易受缓存、调度和混跑影响，因此后续应把 MySQL/PostgreSQL 的分数据库 benchmark 纳入 CI/CD 趋势观察，而不是只看单次本地结果。

### 当前查询计划复测结果

```text
SEARCH records USING INDEX idx_records_table_deleted_created (table_id=? AND deleted_at=?)
```

结论：

- `USE TEMP B-TREE FOR ORDER BY` 已消失。
- 说明 SQLite 已使用复合索引覆盖 `WHERE table_id = ? AND deleted_at IS NULL ORDER BY created_at DESC` 主路径。
- 当前剩余瓶颈更偏向 Query Executor 固定分配以及 JSON 过滤本身。

## 当前工作边界说明

- 当前仓库中已新增本地性能辅助文件，并已开始实施 **阶段 1.1 + 1.4**：
  - `FieldService.ListFields` 已改为批量字段权限判定，消除逐字段权限查询。
  - `FieldService` / `RecordService` 的 `checkTableAccess` 已按“最低所需动作”映射权限，而不是错误抬高到 `manage`。
- `validateToken` 已接入缓存，认证入口不再每次命中 `tokens` 表。
- `records` 主路径复合索引也已落地并通过 `EXPLAIN QUERY PLAN` 验证。
- `pkg/query` 侧已补上请求内权限上下文复用、数据库 `ctx` 透传和 `scanRows` 分配收缩。
- benchmark 夹具已泛化为跨数据库版本；本地默认 SQLite，CI/CD 可直接复用到 MySQL/PostgreSQL。
- 独立性能工作流 `perf.yml` 已补齐，可在推送到 `main/master` 后查看三套数据库的 benchmark 与 explain 结果。
- 下一步建议进入 **阶段 3.x**，优先继续处理 Query / JSON 过滤本身的结构性成本，并根据 GitHub Actions 中的跨数据库趋势结果决定是否做更激进的结构化索引或派生列设计。
