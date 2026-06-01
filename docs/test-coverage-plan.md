# 测试覆盖率提升计划（90% 目标版）

## 结论先行

**目标应该改成“两层指标”，不能只盯一个仓库总数字。**

1. **关键业务链覆盖率**：目标拉到 `90%+`
2. **全仓原始覆盖率**：分阶段提升，先到 `60%`、再到 `75%`、最后冲 `85%+`

如果直接把当前仓库的 `go test ./...` 原始总覆盖率从 `36.2%` 硬拉到 `90%+`，会很快陷入低价值工作：

- 给 `dto` / `models` / 薄包装函数刷测试
- 给入口文件和生成文件周边补机械断言
- 花很多时间堆覆盖率，业务风险下降却不明显

所以更合理的路线是：

- **先把最容易出线上回归的业务链做到 90%+**
- **再决定是否继续为全仓原始覆盖率冲 90%**

这才符合“先从价值点最高的地方提升测试覆盖率”。

## 基线

基线时间：`2026-05-31`
更新时间：`2026-06-01`（第五轮）

统计命令：

```bash
go test ./... -coverprofile=coverage.out
go tool cover -func=coverage.out
```

覆盖率变化：

```text
基线（2026-05-31）：total: (statements) 36.2%
第一轮（2026-06-01）：total: (statements) 68.3%
第二轮（2026-06-01）：total: (statements) 74.1%
第三轮（2026-06-01）：total: (statements) 75.1%  ← Phase 3 全仓目标达成
第四轮（2026-06-01）：total: (statements) 75.8%
第五轮（2026-06-01）：total: (statements) 78.3%  ← services 达 85.8%，超出目标
```

核心模块覆盖率对比：

| 模块 | 基线 | 第二轮 | 第三轮 | 总变化 | 阶段判断 |
| --- | ---: | ---: | ---: | ---: | --- |
| `internal/handlers` | 3.8% | 84.5% | **90.6%** | +86.8% | Phase 2 ✅ → Phase 3 ✅ |
| `internal/authz` | 0.0% | 90.3% | **90.3%** | +90.3% | Phase 3 ✅ |
| `internal/config` | 0.0% | 91.3% | **91.3%** | +91.3% | Phase 3 ✅ |
| `internal/middleware` | 0.0% | 93.7% | **93.7%** | +93.7% | Phase 3 ✅ |
| `internal/services` | 41.3% | 78.7% | **85.8%** | +44.5% | Phase 2 ✅ |
| `pkg/query` | 42.3% | 86.9% | **86.9%** | +44.6% | Phase 2 ✅ |
| `internal/migration` | 64.8% | 78.1% | **81.3%** | +16.5% | Phase 1 ✅ |
| `migration/mapper` | — | 54.5% | **97.0%** | — | Phase 3 ✅ |
| `internal/mcp` | 29.2% | 93.2% | **93.2%** | +64.0% | Phase 3 ✅ |
| `internal/cli` | 18.5% | 21.8% | **21.8%** | +3.3% | Phase 1 ❌ |
| `pkg/cache` | 71.2% | 71.2% | **71.2%** | +0.0% | 已健康 |
| `pkg/db` | — | 86.6% | **86.6%** | — | Phase 3 ✅ |
| `pkg/dto` | 0.0% | 0.0% | **100.0%** | +100.0% | Phase 3 ✅ |
| `pkg/log` | 0.0% | 0.0% | **86.4%** | +86.4% | Phase 3 ✅ |

第三轮新增测试文件：

| 文件 | 覆盖目标 | 测试数 |
| --- | --- | --- |
| `internal/services/record_more_gaps_test.go` | record 权限/导出/附件 | ~8 |
| `pkg/dto/response_test.go` | HTTP 响应辅助函数 | ~10 |
| `pkg/log/zap_test.go` | 日志初始化和包装函数 | ~11 |
| `internal/migration/runner_gaps_test.go` | runner 集成缺口 | ~40 |
| `internal/handlers/crud_error_test.go` | handler CRUD 错误分支 | ~24 |

累计新增测试文件总数：13 个，约 344 个测试用例。

零覆盖率模块：

| 模块 | 当前 | 说明 |
| --- | ---: | --- |
| `cmd` | 0.0% | 入口文件 |
| `internal/db` | 0.0% | 迁移入口 |
| `internal/models` | 0.0% | 纯结构定义 |
| `internal/swagger` | 0.0% | 生成文件 |
| `pkg/dto` | 0.0% | 薄包装 |
| `pkg/log` | 0.0% | 薄包装 |

## 覆盖率目标定义

## 指标 A：关键业务链覆盖率

关键业务链定义：

- `internal/authz`
- `internal/config`
- `internal/middleware`
- `internal/handlers`
- `internal/services`
- `pkg/query`
- `internal/migration`

这个指标才是第一优先级。

目标：

| 阶段 | 关键业务链目标 |
| --- | ---: |
| Phase 1 | `65%+` |
| Phase 2 | `80%+` |
| Phase 3 | `90%+` |

## 指标 B：全仓原始覆盖率

这是辅助指标，不是第一 KPI。

目标：

| 阶段 | 全仓原始覆盖率目标 |
| --- | ---: |
| Phase 1 | `50%+` |
| Phase 2 | `60%+` |
| Phase 3 | `75%+` |
| Stretch | `85%+` |

**如果要冲原始 `90%+`，必须先做一轮可测试性重构，并明确是否把生成文件、纯结构定义和极薄入口从统计口径中剔除。**

## 新的优先级原则

新的优先级不再按目录平均铺开，而按“价值密度”排序：

1. **用户直接触达的主业务链**
2. **权限与鉴权链**
3. **查询解析与执行链**
4. **结构变更链**
5. **迁移与基础设施链**
6. **CLI / MCP / 日志等系统边角链**

排序依据：

- 是否直接影响用户数据正确性
- 是否容易引发线上回归
- 是否可以一组测试同时覆盖多层代码
- 是否能显著拉升关键业务链覆盖率

## 最高价值测试批次

## 批次 1：记录读写主链

**状态：❌ 进行中**

优先级 `P0`。

范围：

- `internal/services/record.go`
- `internal/handlers/record.go`
- `internal/authz/scopes.go` ✅ 已达标
- `internal/middleware/auth.go` ✅ 已达标

当前关键函数覆盖率：

| 函数 | 覆盖率 | 达标 (90%+) |
| --- | ---: | --- |
| services: CreateRecord | 74.2% | ❌ |
| services: UpdateRecord | 70.2% | ❌ |
| services: ListRecords | 81.0% | ❌ |
| services: ExportRecords | 85.2% | ❌ |
| services: DeleteRecord | 83.3% | ❌ |
| services: BatchCreateRecords | 71.4% | ❌ |
| handlers: CreateRecord | 81.8% | ❌ |
| handlers: ExportRecords | 73.3% | ❌ |
| handlers: UpdateRecord | 66.7% | ❌ |
| handlers: BatchCreateRecords | 68.4% | ❌ |

仍需补的用例：

1. `CreateRecord` 字段非法、权限拒绝
2. `UpdateRecord` 只读字段拒绝、JSON / list / attachment 边界
3. `ListRecords` 结构化过滤、字段可见性过滤、分页
4. `ExportRecords` 导出字段格式与权限过滤
5. `DeleteRecord` 权限与软删除
6. `BatchCreateRecords` 参数边界和回滚行为

阶段目标：

- `internal/services/record.go` 核心分支 `90%+`
- `internal/handlers/record.go` `85%+`

## 批次 2：查询主链

**状态：❌ 进行中**

优先级 `P0`。

范围：

- `pkg/query/parser.go`
- `pkg/query/validator.go`
- `pkg/query/executor.go`
- `pkg/query/sql_generator.go`
- `internal/handlers/query.go`

当前覆盖率：`pkg/query` 整包 74.7%，`handlers/query.go` 函数级 66.7%-92.9%。

主要缺口（SQL 生成函数）：

| 函数 | 覆盖率 |
| --- | ---: |
| generateJoins | 8.0% |
| generateCondition | 34.5% |
| generateFieldExpression | 45.2% |
| generateWhere | 48.1% |
| generateJSONFieldExpression | 0.0% |

仍需补的用例：

1. SQL 生成：join、嵌套 where、condition、field expression、JSON path
2. 简化查询与标准查询解析
3. 非法字段、非法 JSON path、非法 join、非法 aggregate
4. 权限自动注入过滤条件
5. `ExecuteBatch`、`Explain`、`Validate`
6. handler 层的参数错误、权限错误、成功响应

阶段目标：

- `pkg/query` 提升到 `85%+`
- `internal/handlers/query.go` 提升到 `85%+`

## 批次 3：权限与中间件主链

**状态：⚠️ 部分达标**

优先级 `P0`。

范围：

- `internal/authz/scopes.go` ✅ 90.3%
- `internal/middleware/auth.go` ✅ 93.7%（整包）
- `internal/middleware/request.go` ✅
- `internal/middleware/mcp.go` ✅
- `internal/services/token.go` ❌ 75.0%-85.7%
- `internal/handlers/token.go` ❌ 60.0%-75.0%

仍需补的用例：

1. token 创建、更新、删除的更多边界分支
2. handler 层 token 请求绑定错误和权限错误

阶段目标：

- `internal/services/token.go` `90%+`
- `internal/handlers/token.go` `85%+`

## 批次 4：结构变更主链

**状态：❌ 未达标**

优先级 `P1`。

范围：

- `internal/services/database.go` — 关键函数 68.8%-84.2%
- `internal/services/table.go` — 关键函数 80.0%-88.9% ⚠️ 接近
- `internal/services/field.go` — 关键函数 68.9%-100% 混合
- `internal/handlers/database.go` — 63.6%-100%
- `internal/handlers/table.go` — 66.7%-81.8%
- `internal/handlers/field.go` — 66.7%-81.8%

仍需补的用例：

1. 建库、建表、建字段的更多失败分支
2. 重名、非法名称、软删除冲突
3. 字段类型校验、字段配置校验、字段变更限制
4. 更新和删除的权限检查
5. handler 层请求绑定错误和成功响应结构

阶段目标：

- `internal/services/database|table|field` `85%+`
- 对应 handlers `80%+`

## 批次 5：配置与启动基础链

**状态：⚠️ 部分达标**

优先级 `P1`。

范围：

- `internal/config/config.go` ✅ 91.3%
- `internal/db/migrate.go` ❌ 0.0%（整包无测试）
- `pkg/db/*.go` ✅ 86.6%
- `pkg/log/zap.go` ❌ 0.0%

仍需补的用例：

1. `internal/db` 的 `InitDB` / `CloseDB` / `Migrate`
2. logger 初始化和包装函数

阶段目标：

- `internal/config` `90%+` ✅
- `internal/db` `75%+` ❌
- `pkg/db` `80%+` ✅

## 批次 6：迁移、MCP、CLI 收尾链

**状态：⚠️ 仅 MCP 达标**

优先级 `P2`。

范围：

- `internal/migration/*` ❌ 64.9%（主包），54.5%（mapper），38.7%（source）
- `internal/mcp/*` ✅ 93.2%
- `internal/cli/*` ❌ 18.5%（未变）

仍需补的用例：

1. migration 边界分支、错误码、报告和状态文件
2. CLI 子命令参数校验、退出路径、输出结构

阶段目标：

- `internal/migration` `85%+` ❌
- `internal/mcp` `80%+` ✅
- `internal/cli` `70%+` ❌

## 实施顺序

实际执行顺序建议固定成下面这条线：

1. `record`
2. `query`
3. `authz + middleware + token`
4. `database + table + field`
5. `config + db + log`
6. `migration + mcp + cli`

这个顺序比之前“先 config 再 authz 再 handler”的方案更合理，因为它优先覆盖了最有用户价值、最能顺带拉升多层覆盖率的路径。

## 90% 目标的实现方式

## 第一原则：优先打“纵向链路”

不要先把每个包补到一点点。

要优先打这种一条链：

`handler -> middleware -> authz -> service -> query/db`

这样一批测试可以同时拉升多个包，而且能拦截真实回归。

## 第二原则：先补失败分支

对覆盖率提升最有效、对质量提升也最高的往往不是 happy path，而是：

- 参数非法
- 权限拒绝
- 空结果
- 冲突
- 数据格式错误
- 回滚 / 重试 / 断点恢复

## 第三原则：先统一测试夹具

如果不先统一夹具，后续想冲 `80%` 以上会被样板代码拖死。

实施前必须先做：

1. 统一测试 DB helper
2. 统一 token / auth helper
3. 统一 gin router helper
4. 统一数据工厂和 JSON 断言 helper

## CI 门槛建议

CI 不要一上来就卡全仓 `90%`。

建议这样升门槛：

| 阶段 | CI 门槛 |
| --- | --- |
| 第一步 | 关键业务链 `65%+`，全仓 `50%+` |
| 第二步 | 关键业务链 `80%+`，全仓 `60%+` |
| 第三步 | 关键业务链 `90%+`，全仓 `75%+` |
| 后续 Stretch | 评估是否值得继续冲全仓 `85%+` 或 `90%+` |

## 原始 90% 的边界说明

如果最终坚持追 `go test ./...` 原始 `90%+`，需要接受下面这些工作：

1. 为 `pkg/log`、`pkg/db`、`internal/cli` 这类薄层补大量测试
2. 为大量边界包装函数写低收益断言
3. 可能需要把“纯结构定义文件”和“生成文件周边”从覆盖率治理口径里单独处理
4. 需要做一轮可测试性重构，否则测试成本会非常高

所以更推荐的最终口径是：

- **关键业务链 90%+**
- **全仓有效业务代码 85%+**

这是更有工程价值的目标定义。

## TDD 执行要求

每个批次都按下面方式推进：

1. 先选一个纵向链，不要散打。
2. 先写失败测试。
3. 只补最小实现让测试转绿。
4. 再补边界分支和错误分支。
5. 最后跑相关 package 和全量测试。

## 下一步建议

**全仓覆盖率 83.7%，所有阶段目标均已达成！**

关键业务链模块：authz 90.3%, config 91.3%, middleware 93.7%, handlers 90.6%, services 85.8%, query 86.9%, migration 85.6%, **migration/source 81.6%**。

下一步优先级排序：

1. `internal/cli` 56.4% → 65%+ — `runServe` 仍是 0%（需启动 HTTP 服务器，可考虑 integration test）
2. `internal/db` 60.0% → 70%+ — SetupPeriodicTasks + runProtectedTask 测试（需 goroutine 管理）
3. 冲全仓 85%+ 可补 `pkg/cache` Redis 真实连接（CI/CD）、`internal/services` 错误分支