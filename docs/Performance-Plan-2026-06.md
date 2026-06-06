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

## 跨数据库对比分析补充（2026-06-06）

> **一句话结论**：当前已经可以在 CI/CD 中稳定产出 SQLite、MySQL、PostgreSQL 三套 benchmark；如果后续数据源场景以 JSON 过滤和记录列表为主，**PostgreSQL（PostgreSQL 数据库）目前明显更适合作为主生产后端**，而 MySQL/SQLite 的 JSON 路径应优先考虑派生列（generated/derived columns，指从 JSON 中拆出来的可索引物理列）或专用索引策略。

### 跨数据库基准快照

说明：

- 数据来自 GitHub Actions `Performance` workflow 成功运行 `27053743084`。
- 当前数据集是 **可复现的 benchmark 灌数集**，适合看相对趋势，不应直接等同于真实生产绝对延迟。
- `ns/op` 越低越好；`B/op` 和 `allocs/op` 越低说明 Go 侧固定开销越小。

| 场景 | SQLite | MySQL 8.0 | PostgreSQL 16 | 观察 |
|---|---|---|---|---|
| `BenchmarkValidateToken` | `176.2 ns/op` | `175.8 ns/op` | `183.4 ns/op` | 三者接近，认证缓存已把数据库差异基本抹平 |
| `RecordServiceListRecords/no_filter` | `2.42 ms/op` | `21.27 ms/op` | `4.46 ms/op` | **MySQL 明显最慢**，即使不带 JSON 过滤也落后 PostgreSQL |
| `RecordServiceListRecords/structured_filter` | `15.88 ms/op` | `25.72 ms/op` | `5.68 ms/op` | PostgreSQL 对结构化 JSON 过滤优势最明显 |
| `FieldServiceListFields` | `1.67 ms/op` | `2.93 ms/op` | `1.81 ms/op` | MySQL 略慢，但不是主要瓶颈 |
| `ExecutorExecute/records_by_table` | `2.72 ms/op` | `6.40 ms/op` | `2.59 ms/op` | Query DSL 普通列表里 MySQL 也明显落后 |
| `ExecutorExecute/records_json_filter` | `22.26 ms/op` | `21.58 ms/op` | `3.10 ms/op` | **JSON 过滤能力差异非常大**，PostgreSQL 显著领先 |

### 查询计划对比

| 数据库 | 计划摘要 | 当前判断 |
|---|---|---|
| SQLite | `SEARCH records USING INDEX idx_records_table_deleted_created (table_id=? AND deleted_at=?)` | 主路径复合索引已命中，但 `JSON_EXTRACT` 仍然是行级解析成本 |
| MySQL | 不同 run 曾出现两类形态：`27057165213` 有过 `idx_records_deleted_at` 路线；`27057515626` 的 plain explain 则走 `idx_records_table_deleted_created`，但 `FORCE INDEX` 后仍能显著降低总耗时 | **MySQL 当前慢因是组合问题**：计划/访问路径、JSON 模型、以及 MySQL 路径上的额外扫描开销都在叠加 |
| PostgreSQL | `Index Scan using idx_records_table_deleted_created on records`，JSON 路径可额外受益于 `idx_records_data_gin` | 普通列表和 JSON 过滤都具备更强索引基础 |

### 为什么 MySQL 记录列表明显慢于 PostgreSQL

#### 已确认事实

| 事实 | 证据 |
|---|---|
| MySQL 的普通列表计划在不同 run 间出现过波动 | `27057165213` 与 `27057515626` 两次 GitHub Actions 结果对比 |
| 差距不只出现在 JSON 过滤场景 | `no_filter` 下 MySQL `20.89 ms`，PostgreSQL `4.69 ms` |
| 窄投影下差距仍然明显 | `no_filter_db_narrow_projection` 下 MySQL `8.30 ms`，PostgreSQL `1.45 ms` |
| JSON 过滤场景差距更大 | `records_json_filter` 下 MySQL `20.48 ms`，PostgreSQL `3.22 ms` |
| MySQL raw SQL 上强制复合索引可显著降低普通列表成本 | `mysql_no_filter_db_narrow_projection_raw_sql` `8.26 ms` -> `mysql_no_filter_db_narrow_projection_force_composite_index` `0.50 ms` |
| MySQL raw SQL 上 generated column 可显著降低 JSON 过滤成本 | `mysql_structured_filter_raw_sql` `11.30 ms` -> `mysql_structured_filter_generated_columns` `0.52 ms` |
| PostgreSQL 具备 `jsonb` + `GIN`（广义倒排索引，用于加速包含匹配和键值检索）基础设施 | `internal/models/models.go`、`internal/db/migrate.go` |
| MySQL/SQLite 当前路径是 `JSON_EXTRACT(data, ?) = ?` | `internal/services/record.go` |

#### 当前最可信的原因拆解

| 原因 | 解释 | 置信度 |
|---|---|---|
| MySQL 当前基础列表路径存在计划/访问形态不稳定问题 | 不同 run 中 plain explain 曾出现不同形态；且即便 plain explain 已走复合索引，`FORCE INDEX` raw SQL 仍能把总耗时从 `8.26 ms` 压到 `0.50 ms` | 高 |
| JSON 存储与索引模型差异 | PostgreSQL 使用 `jsonb`（二进制 JSON）且已建 `GIN` 索引；MySQL 当前只有 `json` 原列，没有针对高频 JSON 键的专门索引 | 高 |
| 宽投影会增加 MySQL 成本，但不是主因 | MySQL `no_filter_db_narrow_projection` `8.37 ms`，`no_filter_db_wide_projection` `10.61 ms`；宽行有放大，但基础扫描已偏慢 | 高 |
| 当前 JSON 谓词开销主要留在数据库层 | `records_json_filter_id_only` 与 `records_json_filter_full_data_projection` 接近；同时 raw `JSON_EXTRACT` `11.30 ms` 对比 generated column `0.52 ms`，说明主耗时仍在数据库 JSON 谓词路径 | 高 |
| 当前 MySQL 热路径还叠加了 Go/GORM 侧固定税 | `EXPLAIN ANALYZE` 里的 MySQL engine 执行时间只有亚毫秒到数毫秒级，但 benchmark 端到端明显更高，说明 round-trip / scan / ORM 路径还有额外成本 | 中 |
| PostgreSQL 对当前查询形状更友好 | 当前 `WHERE table_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT ?` 加上 `data @> ?`，PostgreSQL 的执行器和索引模型明显更契合 | 中 |

#### 不应过早下结论的点

- **目前还不能把 MySQL 慢完全归结为驱动问题**。现有证据首先指向 MySQL 执行计划选择和 JSON 行处理成本，而不是 Go MySQL driver 本身。
- **当前也不能把 MySQL 的问题完全归结为 JSON**。普通列表和 JSON 过滤都存在明显差距，而且普通列表差距并未因为“命中复合索引”就自然消失。
- **也不能仅凭一次 CI 结果就决定全面放弃 MySQL**。但至少在“JSON-heavy（以 JSON 条件为主）读路径”上，MySQL 需要额外结构化优化，否则很难接近 PostgreSQL。
- **当前 benchmark 是合成数据集**。如果真实线上过滤键更集中，MySQL 通过派生列和复合索引仍可能把差距显著缩小。

### JSON 过滤是否要拆派生列或专用索引

#### 结论先行

| 数据库 | 建议 |
|---|---|
| PostgreSQL | **先不急着拆派生列**。优先继续利用 `jsonb` + `GIN`，只对极高频、强选择性的字段补表达式索引或普通 BTREE（B-tree，平衡树索引） |
| MySQL | **建议对高频 JSON 过滤键拆派生列 / generated column 并建索引**，否则 `JSON_EXTRACT(data, ...) = ?` 很难在数据量增长后维持稳定延迟 |
| SQLite | 若只是本地开发和 CI 快速回归，可保留当前通用 JSON 路径；若 SQLite 未来也承担实际生产过滤压力，同样要把高频键拆成物化列 |

#### 原因说明

| 数据库 | 当前实现 | 主要问题 | 更合适的策略 |
|---|---|---|---|
| PostgreSQL | `data @> ?` + `jsonb` + `idx_records_data_gin` | 仍需确认所有高频谓词都能稳定命中 `GIN`，少数排序/范围条件未必适合仅靠 `GIN` | 保持 JSONB 主体模型，只为热点键补充定向索引 |
| MySQL | `JSON_EXTRACT(data, ?) = ?` | 对 JSON 路径做函数求值，且 `JSON` 原列不能直接索引；若不转成 generated column / `JSON_VALUE()` 表达式索引，优化空间有限 | 把 `status`、`category`、`owner_id`、数值型评分等高频键拆成 generated column，或改为 `JSON_VALUE()` 函数索引，再建 `(table_id, deleted_at, derived_col, created_at DESC)` |
| SQLite | `JSON_EXTRACT(data, ?) = ?`，底层 `data` 为 `TEXT` | JSON1（SQLite JSON 扩展）本质上更偏函数式解析，数据量变大时 CPU 成本很难压住 | 继续作为 correctness/perf smoke baseline；若必须承压，再考虑影子列 |

#### 派生列的使用门槛

只有满足下面条件的 JSON 键，才值得升级为派生列：

1. 该键出现在较高比例的线上/目标查询里。
2. 该键选择性足够高，索引后能明显缩小候选行。
3. 该键语义稳定，不会频繁改名或变更类型。
4. 该键值得承担额外写放大和迁移复杂度。

## 后续详细执行计划

### 目标

把“已经能稳定跑 benchmark”推进到“能解释差异、能持续比较、能针对不同数据库给出可实施优化方案”。

### 阶段化计划

| 阶段 | 动作 | 输出物 | 验收标准 |
|---|---|---|---|
| P0 | 固化跨数据库基线，把 SQLite/MySQL/PostgreSQL 结果写入文档并沉淀 PR summary 模板 | 文档更新、PR summary 模板 | 团队可以直接看到单次变更前后的跨库快照 |
| P1 | 拆解 MySQL 列表慢因：增加“窄投影 vs 宽投影”“仅数据库过滤 vs Go 端 JSON decode” benchmark | 新 benchmark、Explain 对照结果 | 能区分是数据库扫描慢、宽行物化慢，还是应用解码慢 |
| P2 | 梳理高频 JSON 键并做候选清单 | 键清单、出现频率、选择性假设 | 至少产出一批值得索引化的字段候选 |
| P3 | 为 MySQL 设计 generated column + 复合索引方案 | 设计文档、迁移草案、TDD 用例 | 对热点键可生成明确 DDL（数据定义语言）方案 |
| P4 | 验证 PostgreSQL 当前 `GIN` 是否已覆盖真实热点谓词，必要时补表达式索引 | Explain/Analyze 结果、索引方案 | PostgreSQL 保持领先且不过度设计 |
| P5 | 定义 SQLite 的职责边界 | 文档说明 | SQLite 仅作为本地/CI 基线，还是也要承担生产过滤，边界明确 |
| P6 | 将 before/after 对比纳入 PR 和 CI/CD | PR summary、Actions artifact、summary 规范 | 每次性能改造都可回溯结果、命令和执行计划 |

### P1：MySQL 慢因拆解的具体测试项

| 测试项 | 目的 | 预期能回答的问题 |
|---|---|---|
| `ListRecords` 窄投影 benchmark（只取 `id/table_id/created_at`） | 排除宽字段影响 | MySQL 是否主要慢在宽行读取 |
| `ListRecords` 全投影 benchmark（含 `data/version/timestamps`） | 对照现状 | 当前主路径是否主要受 JSON 大字段拖累 |
| `Query Executor` 仅过滤不解码 benchmark | 分离 DB 与 Go 端成本 | 慢点在数据库执行还是 `scanRows`/JSON decode |
| MySQL `EXPLAIN ANALYZE` 对比不同 `LIMIT` | 观察 top-N 和回表代价 | 慢是否与分页深度相关 |
| 同条件 PostgreSQL 对照 | 防止误把通用成本当成 MySQL 特例 | 哪些差距是数据库专有的 |

### P1 首轮诊断 benchmark 记录（2026-06-06）

已完成内容：

- `internal/services/perf_benchmark_test.go`
  - 新增 `no_filter_db_narrow_projection`
  - 新增 `no_filter_db_wide_projection`
  - 新增 `no_filter_go_response_shaping`
  - 新增 `structured_filter_db_narrow_projection`
  - 新增 `structured_filter_db_wide_projection`
- `pkg/query/perf_benchmark_test.go`
  - 新增 `records_json_filter_id_only`
  - 新增 `records_json_filter_full_data_projection`

这样做的目的不是追求更多 benchmark 名字，而是把以下三类成本拆开：

1. **数据库候选行获取成本**：只取窄字段，观察索引扫描 + 最小回表开销。
2. **宽行物化成本**：把 `data` 一起投影出来，观察宽字段是否明显拖慢数据库。
3. **Go 侧 JSON/响应整形成本**：脱离数据库过滤后，只看 `parseRecordPayload + filterReadableData + response` 组装成本。

本地 SQLite 首轮结果（Windows / PowerShell / 2026-06-06）：

| 场景 | 结果 | 观察 |
|---|---|---|
| `RecordServiceListRecords/no_filter` | `1425928 ns/op` | 端到端基线 |
| `RecordServiceListRecords/no_filter_db_narrow_projection` | `147897 ns/op` | 纯数据库窄投影很快 |
| `RecordServiceListRecords/no_filter_db_wide_projection` | `263182 ns/op` | 加上 `data` 后数据库读取开销上升，但仍明显低于端到端 |
| `RecordServiceListRecords/no_filter_go_response_shaping` | `281578 ns/op` | Go 侧 JSON 解码与响应整形有稳定成本，但不是全部成本来源 |
| `RecordServiceListRecords/structured_filter` | `8098706 ns/op` | 结构化过滤端到端仍明显更慢 |
| `RecordServiceListRecords/structured_filter_db_narrow_projection` | `1067977 ns/op` | 结构化过滤时，数据库过滤本身已显著放大 |
| `RecordServiceListRecords/structured_filter_db_wide_projection` | `1177650 ns/op` | 宽投影有影响，但不是结构化过滤慢的主因 |
| `ExecutorExecute/records_json_filter` | `12506741 ns/op` | 当前 Query DSL JSON 过滤端到端基线 |
| `ExecutorExecute/records_json_filter_id_only` | `12393429 ns/op` | 去掉 `data` 全列投影后，耗时几乎不变 |
| `ExecutorExecute/records_json_filter_full_data_projection` | `12514896 ns/op` | 投影 `data` 会增加分配，但对总耗时影响有限 |

首轮结论：

- 对 `ListRecords/no_filter` 而言，**数据库窄/宽投影 + Go 侧整形之和仍小于端到端耗时**，说明权限、COUNT、GORM 语句构建和结果拼装仍有固定税。
- 对 `structured_filter` 和 `records_json_filter` 而言，**数据库 JSON 过滤本身仍是主瓶颈**；至少在 SQLite 基线下，是否把整列 `data` 投影回应用侧，不会改变主耗时级别。
- 因此下一步把同一组拆解 benchmark 带到 **MySQL / PostgreSQL CI** 是必要的：如果 MySQL 上 `db_wide_projection` 明显高于 PostgreSQL，就更像“宽行物化/回表”问题；如果 `db_narrow_projection` 也显著慢，就更像 JSON 谓词执行与执行器成本问题。

建议的复测命令：

```powershell
go test ./internal/services -run ^$ -bench BenchmarkRecordServiceListRecords -benchmem -count 1
go test ./pkg/query -run ^$ -bench BenchmarkExecutorExecute -benchmem -count 1
```

### P1 跨数据库拆解结果（GitHub Actions `27057165213`）

运行地址：

- `https://github.com/jiangfire/cornerstone/actions/runs/27057165213`

#### `RecordService` 拆解结果

| 场景 | SQLite | MySQL 8.0 | PostgreSQL 16 | 观察 |
|---|---|---|---|---|
| `no_filter` | `2.39 ms` | `20.89 ms` | `4.69 ms` | MySQL 普通列表仍显著落后 |
| `no_filter_db_narrow_projection` | `0.27 ms` | `8.30 ms` | `1.45 ms` | **MySQL 即使只取窄字段也很慢**，说明问题不只是宽行 |
| `no_filter_db_wide_projection` | `0.46 ms` | `10.26 ms` | `1.99 ms` | 宽投影会继续放大 MySQL，但不是唯一问题 |
| `no_filter_go_response_shaping` | `0.33 ms` | `0.34 ms` | `0.34 ms` | Go 侧整形成本三者接近，不是跨库差异主因 |
| `structured_filter` | `15.76 ms` | `24.96 ms` | `4.08 ms` | PostgreSQL 对结构化过滤仍明显最优 |
| `structured_filter_db_narrow_projection` | `3.18 ms` | `10.83 ms` | `0.88 ms` | MySQL/SQLite 的 JSON 过滤 DB 成本明显高于 PostgreSQL |
| `structured_filter_db_wide_projection` | `3.36 ms` | `11.14 ms` | `1.11 ms` | MySQL 宽投影有增量，但核心仍是 DB 过滤成本 |

#### `Query Executor` 拆解结果

| 场景 | SQLite | MySQL 8.0 | PostgreSQL 16 | 观察 |
|---|---|---|---|---|
| `records_by_table` | `2.81 ms` | `6.33 ms` | `2.79 ms` | MySQL 普通 Query DSL 列表也偏慢 |
| `records_json_filter` | `22.15 ms` | `20.48 ms` | `3.22 ms` | PostgreSQL 对 JSON 条件优势巨大 |
| `records_json_filter_id_only` | `22.04 ms` | `20.25 ms` | `3.12 ms` | 去掉 `data` 全列投影后，MySQL/SQLite 总耗时几乎不变 |
| `records_json_filter_full_data_projection` | `22.33 ms` | `20.90 ms` | `3.99 ms` | `full_data_projection` 主要放大分配，不改主耗时级别 |

#### 最新 `EXPLAIN ANALYZE` 观察

| 数据库 | 观察 | 结论 |
|---|---|---|
| SQLite | `SEARCH records USING INDEX idx_records_table_deleted_created` | 主路径索引命中正常 |
| MySQL | `Index lookup on records using idx_records_deleted_at (deleted_at=NULL)`，之后 `Filter(table_id)`，再 `Sort(created_at DESC)` | **当前 MySQL 基础列表路径没有选中预期复合索引**，这是普通列表慢的重要原因 |
| PostgreSQL | `Index Scan using idx_records_table_deleted_created on records` + `top-N heapsort` | 计划稳定，主路径表现正常 |

#### 当前结论边界

1. **MySQL 当前慢不只是 JSON 问题**：最新基线已经显示 `no_filter` 的窄投影也明显慢，且 explain 选错了索引路径。
2. **但 MySQL 的 JSON 路径在当前模型下优化空间也确实有限**：`id_only` 与 `full_data_projection` 的耗时接近，说明瓶颈主要在数据库 JSON 谓词，而不是应用侧取回 `data`。
3. **所以“还能不能优化”的答案是有条件的**：
   - 若允许调整数据模型，可以继续做：generated column、`JSON_VALUE()` 表达式索引、必要时调整/收敛与 `deleted_at` 竞争的索引。
   - 若坚持保持“单个原始 `JSON` 列 + 通用 `JSON_EXTRACT(data, path) = value`”这一模型不变，则 **可优化空间有限**，很难把 MySQL 拉近 PostgreSQL。

### MySQL JSON 路径的官方证据

以下结论已由官方文档支持：

| 来源 | 关键点 | 对当前项目的含义 |
|---|---|---|
| MySQL Reference Manual, `Secondary Indexes and Generated Columns` | `JSON` 列不能直接索引；推荐通过 generated column 间接索引 | 当前 `JSON_EXTRACT(data, ...)` 若无派生列，很难获得稳定索引收益 |
| MySQL Reference Manual, `Functions That Search JSON Values` | `JSON_VALUE()` 可以直接用于表达式索引，简化 JSON 索引创建 | 如果继续支持 MySQL，应优先考虑 `JSON_VALUE(data, '$.status' RETURNING ...)` 一类索引方案 |
| PostgreSQL `JSON Types / jsonb Indexing` | `jsonb` 的 `@>` 可由 `GIN` 支持 | 当前 PostgreSQL 路径与项目实现天然更匹配 |

参考链接：

- MySQL: `https://dev.mysql.com/doc/refman/8.0/en/json-search-functions.html`
- MySQL: `https://dev.mysql.com/doc/refman/en/create-table-secondary-indexes.html`
- PostgreSQL: `https://www.postgresql.org/docs/current/datatype-json.html`

### P1 MySQL 原始 SQL 实验结果（GitHub Actions `27057515626`）

运行地址：

- `https://github.com/jiangfire/cornerstone/actions/runs/27057515626`

#### 实验目的

把以下三层拆开：

1. **当前普通列表 raw SQL** 到底有多慢。
2. **强制复合索引** 后 MySQL 普通列表理论上能快到什么程度。
3. **当前 `JSON_EXTRACT`** 与 **generated column** 在 MySQL 上的差距到底有多大。

#### MySQL 实验 benchmark

| 场景 | 结果 | 结论 |
|---|---|---|
| `mysql_no_filter_db_narrow_projection_raw_sql` | `8257899 ns/op` | 当前普通列表 raw SQL 仍明显偏慢 |
| `mysql_no_filter_db_narrow_projection_force_composite_index` | `496597 ns/op` | **强制复合索引后约快 `16.6x`**，说明主路径访问形态对 MySQL 极其敏感 |
| `mysql_structured_filter_raw_sql` | `11295368 ns/op` | 当前 `JSON_EXTRACT` 结构化过滤仍然偏慢 |
| `mysql_structured_filter_generated_columns` | `516111 ns/op` | **generated column 实验索引后约快 `21.9x`**，说明 JSON 热字段拆列对 MySQL 收益非常明确 |

#### MySQL `EXPLAIN ANALYZE` 实验

| 场景 | 计划摘要 | 观察 |
|---|---|---|
| plain list | `Index range scan on records using idx_records_table_deleted_created` | plain explain 已能看到复合索引，但总 benchmark 仍明显高于 forced 版本 |
| forced composite | `Covering index lookup on records using idx_records_table_deleted_created` | 强制后变成更明确的 covering lookup（覆盖索引查找），engine 时间进一步下降 |
| raw structured filter | `Index range scan ...` 后再做 `Filter(json_extract(...))` | 先按主路径取候选行，再逐行做 JSON 函数过滤 |
| generated columns | `Covering index lookup on records using idx_records_bench_status_category_created` | 结构化过滤直接变成索引查找，engine 时间降到 `~0.08 ms` 量级 |

补充说明：

- `EXPLAIN ANALYZE` 里的 `actual time` 只覆盖 MySQL engine 执行时间，不包含 Go 侧 round-trip、驱动扫描、GORM 组装等固定税，因此会明显低于 benchmark 的 `ns/op`。
- 但 **plain/raw 与 forced/generated 的相对差异方向是一致的**，所以这些实验仍然足以说明优化方向。

#### 这一轮可以得出的更强结论

1. **MySQL 还能优化，但不能只靠“保留原始 JSON 列 + 通用 JSON_EXTRACT”去优化。**
2. **如果允许改数据模型，MySQL 的正确路线已经很明确：**
   - 普通列表：确保主路径稳定落在 `table_id + deleted_at + created_at` 这一类覆盖访问路径上。
   - JSON 条件：把高频键拆成 generated column，或用 `JSON_VALUE()` 表达式索引替代当前通用 `JSON_EXTRACT`。
3. **如果不允许改数据模型与热点查询路径，只保留现状做微调，则 MySQL 的优化空间有限。**

### P3：MySQL/SQLite JSON 结构化优化候选

| 场景 | 建议索引方向 | 说明 |
|---|---|---|
| 单字段精确匹配，如 `status = "published"` | `(table_id, deleted_at, status_derived, created_at DESC)` | 同时兼顾主路径筛选和排序 |
| 多字段联合过滤，如 `status + category` | 按真实查询频率选择复合索引顺序 | 不建议无证据地为所有键做大而全索引 |
| 数值范围过滤，如 `score >= 80` | 数值型 generated column + BTREE | 不能继续让字符串化 JSON 比较承担主路径 |
| 模糊搜索 | 不建议继续挂在 JSON 通用路径 | 应拆到专用全文检索或搜索接口，而不是依赖 `JSON_EXTRACT` |

### 文档与 PR Summary 的整理建议

**建议整理，而且应作为固定交付物，而不是补充材料。**

推荐把每次性能改造的输出统一为以下三层：

| 层级 | 放置位置 | 必须包含的内容 |
|---|---|---|
| 长期基线文档 | `docs/Performance-Plan-2026-06.md` | 跨数据库 benchmark 表、Explain 结论、已知瓶颈、后续计划 |
| 单次改造摘要 | PR summary / PR 描述 | 变更目标、before/after 数据、命令、workflow run 链接、风险说明 |
| 原始产物 | GitHub Actions artifacts | `auth.txt`、`services.txt`、`query.txt`、`explain.txt` 原文，便于追溯 |

建议 PR summary 固定包含下面几项：

| 项目 | 内容 |
|---|---|
| 背景 | 为什么要做这次性能改造 |
| 范围 | 改了哪些模块，没改哪些模块 |
| Benchmark before/after | 至少列出受影响场景的前后数据 |
| Explain plan | 关键 SQL 是否命中目标索引 |
| DB 维度差异 | SQLite/MySQL/PostgreSQL 是否都验证过 |
| 风险 | 是否引入新索引、写放大、兼容性变化 |
| 未解决问题 | 如 Redis CI 失败这类与本次性能无关但仍待处理的问题 |

### 当前建议的决策

1. **生产优先后端**：若目标场景确实以 JSON 条件查询和列表读取为主，优先把 PostgreSQL 作为默认推荐部署后端。
2. **MySQL 优化方向**：不要继续只靠 `JSON_EXTRACT` 硬扛；进入下一轮时，应优先做 generated column 候选设计和对照 benchmark。
3. **SQLite 定位**：继续保留为本地与 CI 快速回归基线，不把它当作 JSON-heavy 生产能力的真实性能代表。
4. **交付规范**：后续每次性能改造都要同步更新本文档和 PR summary，避免 benchmark 结果只留在 Actions 页面里。
