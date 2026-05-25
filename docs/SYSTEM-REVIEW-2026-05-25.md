# Cornerstone 系统 Review 报告

## 评审日期：2026-05-25

---

## 一、严重缺陷（影响功能可用性）

### 1. AI 推荐功能必然失败
**位置：** `backend/internal/services/governance.go:787-794`

`GenerateAIRecommendation` 在创建 Review 时未设置 `ReviewerID`：
```go
reviewReq := CreateGovernanceReviewRequest{
    TaskID:          req.TaskID,
    ReviewType:      mapRecommendationTypeToReviewType(req.RecommendationType),
    ProposalSource:  "llm-governor",
    ProposalPayload: string(proposalJSON),
    // BUG: 缺少 ReviewerID 赋值
}
```

而 `CreateReview` 要求 `ReviewerID` 必须存在且对应用户存在（`binding:"required"` + `ensureUserExists`），这会导致 **AI 推荐功能 100% 失败**。

**修复建议：** 在请求中增加 `reviewer_id` 字段，或在 AI 推荐场景下将当前用户设为默认审核人。

---

### 2. 前端治理任务列表频繁触发请求
**位置：** `frontend/src/views/GovernanceView.vue:1104-1112`

```ts
watch(
  () => filters.value,
  () => {
    currentPage.value = 1
    loadTasks()
  },
  { deep: true },
)
```

对 `filters.value` 进行 deep watch，包括输入框的每个字符变化都会触发请求，造成：
- 大量无效请求
- 后端压力增加
- 用户体验差（列表闪烁）

**修复建议：** 对文本输入使用防抖（debounce），或使用 `watchEffect` + 明确的监听字段。

---

### 3. 审核通过后任务状态仍为 "open" 而非 "done"
**位置：** `backend/internal/services/governance.go:691-698`

```go
taskStatus := "blocked"
if targetStatus == "approved" {
    taskStatus = "open"  // 应该是 "done"？
    if s.shouldEnqueueApply(review) {
        review.ApplyStatus = "pending"
    }
}
```

审核通过后任务状态回到 "open"，需要人工再次标记完成。如果审核通过且无需回写（或回写成功），任务应该自动变为 "done"。

---

## 二、功能缺失

### 治理域
| 缺失功能 | 影响 | 建议优先级 |
|---------|------|-----------|
| 治理任务删除 API | 无法删除误创建的任务 | P1 |
| 治理证据删除/更新 API | 只能添加，不能修改/删除 | P2 |
| 治理评论删除/更新 API | 只能添加，不能修改/删除 | P2 |
| 治理外部链接独立管理 API | 只能在创建任务时添加 | P3 |
| Outbox 事件查询/管理 API | 无法查看回写任务状态 | P2 |

### 系统管理
| 缺失功能 | 影响 | 建议优先级 |
|---------|------|-----------|
| 活动日志查询 API | 有记录但无查询接口，Dashboard 的活动统计依赖 `stats/activities` | P2 |
| 集成事件列表/查询 API | 无法查看入站事件历史 | P3 |
| 用户角色管理（管理员修改） | 无 API 将普通用户设为管理员 | P2 |
| 数据库 Owner 转让 | 只有 share/remove/update role，不能转让所有权 | P3 |
| 组织 Owner 转让 | 同上 | P3 |

### MCP 接口
当前仅实现 4 个 Tools：
- `query_data`
- `create_database`
- `list_databases`
- `get_table_schema`

**缺失：** 记录 CRUD、表/字段管理、插件执行等工具，限制了 AI Agent 的能力边界。

---

## 三、安全缺陷

### 1. XSS 防护不足
**位置：** `backend/internal/services/governance.go:188-192`

```go
func sanitizeText(input string) string {
    input = strings.TrimSpace(input)
    input = strings.ReplaceAll(input, "\x00", "")
    return input
}
```

仅去除 NULL 字符，对 `<script>` 等 XSS payload 无防护。虽然前端 Element Plus 默认转义，但 API 层面应做输入净化。

### 2. 缺少速率限制
无 Rate Limiting 中间件，存在暴力破解和 DDoS 风险。

### 3. Token 黑名单非持久化
`utils.IsTokenBlacklisted` 使用内存 map，重启后所有黑名单失效，已登出用户 token 可继续使用。

### 4. 插件执行无沙箱隔离
插件直接在当前进程执行，恶意插件可导致系统崩溃或数据泄露。

### 5. Outbound Token 明文存储
`OUTBOUND_INTEGRATION_TOKENS` 以明文形式存储在环境变量中，建议使用加密或密钥管理服务。

### 6. 头像上传大小未校验
Handler 中未根据 `AppSettings.MaxFileSize` 校验上传文件大小。

---

## 四、性能问题

### 1. 治理任务详情多次独立查询
**位置：** `backend/internal/services/governance.go:415-451`

`GetTask` 使用 4 次独立查询获取 links/evidences/comments/reviews，可优化为 JOIN 或并行查询。

### 2. Outbox 串行处理
**位置：** `backend/internal/services/governance_apply.go:561-585`

`ProcessPendingOutbox` 逐个串行处理，遇到慢请求会阻塞后续任务，建议使用 worker pool 并行处理。

### 3. MCP Replay Buffer 内存风险
**位置：** `backend/internal/handlers/mcp.go:28-33`

`mcpHub` 是全局单例，按用户维度维护 replay buffer。用户量增大时内存占用线性增长，建议设置 buffer 过期策略。

### 4. 物化视图刷新缺少并发控制
**位置：** `backend/internal/db/migrate.go:536-553`

`REFRESH MATERIALIZED VIEW CONCURRENTLY` 需要唯一索引支持，若创建失败会导致刷新失败，但当前无监控告警。

---

## 五、代码质量 & 潜在 Bug

### 1. `isSystemAdmin` 忽略数据库错误
**位置：** `backend/internal/services/governance.go:265-269`

```go
func (s *GovernanceService) isSystemAdmin(userID string) bool {
    var count int64
    s.db.Model(&models.User{}).Where("id = ? AND is_system_admin = ?", userID, true).Count(&count)
    return count > 0
}
```

`Count()` 错误被忽略，数据库故障时可能错误地判定为非管理员。

### 2. `isUniqueConstraintError` 字符串匹配不可靠
**位置：** `backend/internal/services/integration_events.go:180-190`

使用错误信息字符串匹配判断唯一约束，不同数据库驱动（pgx vs sqlite）的错误信息格式不同，可能误判。

### 3. 默认管理员 ID 硬编码可能冲突
**位置：** `backend/internal/db/migrate.go:252`

```go
admin := models.User{
    ID: "usr_admin",  // 硬编码 ID
```

如果用户表已存在该 ID（虽然概率低），会导致创建失败。

### 4. `PluginExecution` 缺少软删除
与其他模型不一致，无 `DeletedAt` 字段。

### 5. `UpdateGovernanceTaskRequest` 字段全为 required
**位置：** `backend/internal/services/governance.go:110-117`

```go
type UpdateGovernanceTaskRequest struct {
    Title       string `json:"title" binding:"required"`
    Description string `json:"description"`
    Status      string `json:"status" binding:"required"`
    Priority    string `json:"priority" binding:"required"`
    // ...
}
```

无法只更新单个字段（如仅修改 assignee），必须提供完整对象。

### 6. 前端缺少错误边界处理
多个 API 调用使用 `console.error` 但无统一错误上报，生产环境难以排查问题。

### 7. 集成事件接收缺少幂等性保障（竞态条件）
**位置：** `backend/internal/services/integration_events.go:192-222`

虽然先查询再插入，但高并发下仍存在竞态窗口，建议使用数据库唯一约束作为最终保障（当前有唯一索引，但代码也做了预检查）。

---

## 六、架构 & 设计建议

### 1. 配置来源不统一
`governance_apply.go` 中大量使用 `os.Getenv` 直接读取环境变量，而 `config.go` 已经解析了这些配置。建议统一使用 `config` 包，避免运行时重复解析。

### 2. 缺少事件总线（Event Bus）
当前 MCP 通知通过直接调用 `publishXxxChanged` 函数，耦合度高。建议引入内部事件总线，handler 只发事件，通知逻辑独立订阅。

### 3. 前端状态管理可优化
`GovernanceView.vue` 接近 1300 行，逻辑复杂。建议：
- 拆分为子组件（TaskList、TaskDetail、ReviewPanel 等）
- 使用 Pinia store 管理治理状态

### 4. 测试覆盖
E2E 测试报告（2026-01-11）显示 26/26 通过，但缺少：
- 治理域单元测试（回写重试、熔断等）
- 集成事件去重测试
- MCP SSE 断线重连测试

---

## 七、总结

| 类别 | 数量 | 最严重问题 |
|------|------|-----------|
| 严重缺陷 | 3 | AI 推荐功能不可用、前端频繁请求、审核状态逻辑 |
| 功能缺失 | 9 | 任务删除、日志查询、MCP 工具不全 |
| 安全缺陷 | 6 | XSS 防护不足、无 Rate Limit、Token 黑名单非持久 |
| 性能问题 | 4 | Outbox 串行、详情多次查询、Buffer 内存风险 |
| 代码质量问题 | 7 | 错误忽略、硬编码、required 过严 |

**建议优先处理：**
1. 修复 AI 推荐 ReviewerID 缺失（1行代码即可修复）
2. 前端治理列表增加防抖
3. 添加 Rate Limiting 中间件
4. 补充治理任务删除 API
5. 统一配置读取来源（移除 `os.Getenv` 直接调用）
