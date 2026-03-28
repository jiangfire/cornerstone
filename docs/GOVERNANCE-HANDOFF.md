# 治理集成交接记录

## 1. 本次已确认的架构决策

- 不做统一认证中心
- `cornerstone` 与 `fuckcmdb` 保持各自登录体系
- 系统间调用使用 `integration token`
- 采用事件驱动，不做数据库级耦合
- AI 只做建议和编排，不直接修改治理主数据
- 元数据真相在 `fuckcmdb`
- 流程真相在 `cornerstone`

相关设计文档：

- `docs/CROSS-SYSTEM-GOVERNANCE-ARCHITECTURE.md`
- `docs/CROSS-SYSTEM-GOVERNANCE-MODEL.md`

## 2. `cornerstone` 已完成内容

### 2.1 后端治理域

已新增模型：

- `governance_tasks`
- `governance_reviews`
- `governance_evidences`
- `governance_external_links`
- `governance_comments`
- `integration_inbound_events`

已新增治理域接口：

- `POST /api/governance/tasks`
- `GET /api/governance/tasks`
- `GET /api/governance/tasks/:id`
- `PUT /api/governance/tasks/:id`
- `POST /api/governance/tasks/:id/evidences`
- `POST /api/governance/tasks/:id/comments`
- `POST /api/governance/reviews`
- `GET /api/governance/reviews/:id`
- `POST /api/governance/reviews/:id/approve`
- `POST /api/governance/reviews/:id/reject`

### 2.2 入站事件骨架

已新增接口：

- `POST /api/integrations/events`

行为：

- 校验 `Authorization: Bearer <integration_token>`
- 校验 `X-Source-System`
- 按 `event_id` 幂等处理
- 保存入站事件
- 对以下事件自动创建治理任务：
  - `dq.alert.triggered`
  - `dq.rule.failed`
  - `metadata.schema.changed`
  - `ai.recommendation.generated`

### 2.3 前端治理页面

已新增页面：

- `/governance`

已支持：

- 治理任务列表与筛选
- 新建治理任务
- 查看任务详情
- 更新任务状态/优先级/负责人
- 添加证据
- 添加评论
- 发起审核
- 审核通过/驳回

## 3. 当前集成配置

后端入站事件认证读取以下环境变量：

- `INTEGRATION_SHARED_TOKEN`
  - 所有调用方共用一个 token
- `INTEGRATION_TOKENS`
  - 多调用方 token 白名单
  - 格式示例：`fuckcmdb=token_a,llm-governor=token_b`

建议请求头：

```http
Authorization: Bearer your-token
X-Source-System: fuckcmdb
X-Trace-ID: trace-001
```

## 4. 已验证项

- `cd backend && go test ./...`
- `cd frontend && pnpm type-check`
- `cd ../fuckcmdb && go test ./...`

验证结果：通过。

## 5. 下次继续时优先做什么

### 第一优先级

联调并验证 `fuckcmdb -> cornerstone` 自动建单，当前已接入：

- `metadata.schema.changed`
- `dq.rule.failed`

明日联调时需要：

- 在 `cornerstone` 配置 `INTEGRATION_SHARED_TOKEN` 或 `INTEGRATION_TOKENS`
- 在 `fuckcmdb` 配置 `DATAMAP_GOVERNANCE_*`
- 触发一次 schema sync
- 触发一次 DQ check

目标是让 `cornerstone` 自动生成两类治理任务。

### 第二优先级

在 `cornerstone` 增加“审核通过后回写 `fuckcmdb`”能力：

- 术语绑定回写
- 分类分级回写
- DQ 规则确认回写

### 第三优先级

前端体验优化：

- 审核提案 JSON 模板
- 外部资源跳转
- 任务统计卡片
- 负责人展示优化

## 6. 关键文件位置

后端：

- `backend/internal/models/models.go`
- `backend/internal/db/migrate.go`
- `backend/internal/services/governance.go`
- `backend/internal/services/integration_events.go`
- `backend/internal/handlers/governance.go`
- `backend/internal/handlers/integration_events.go`
- `backend/internal/middleware/integration.go`
- `backend/cmd/server/main.go`

前端：

- `frontend/src/views/GovernanceView.vue`
- `frontend/src/services/api.ts`
- `frontend/src/types/api.ts`
- `frontend/src/router/index.ts`
- `frontend/src/components/AppLayout.vue`

## 7. 当前边界说明

当前已经完成：

- `cornerstone` 入站事件接收与自动建单
- `fuckcmdb` 出站 HTTP 治理事件发送
- 已接入 `metadata.schema.changed`
- 已接入 `dq.rule.failed`

当前还没有完成：

- `cornerstone -> fuckcmdb` 审核结果回写
- `dq.alert.triggered`
- `ai.recommendation.generated`
- AI 建议结果在页面上的专门展示区
- 事件总线实现

当前状态已从“等待对端接入”变为“HTTP 双端已就位，等待联调验证”。

## 8. `fuckcmdb` 联调配置

建议在 `fuckcmdb` 配置以下环境变量：

```env
DATAMAP_GOVERNANCE_ENABLED=true
DATAMAP_GOVERNANCE_ENDPOINT=http://localhost:8081/api/integrations/events
DATAMAP_GOVERNANCE_INTEGRATION_TOKEN=your-integration-token
DATAMAP_GOVERNANCE_SOURCE_SYSTEM=fuckcmdb
DATAMAP_GOVERNANCE_TIMEOUT=5s
```

事件发送位置：

- `internal/service/source.go`
- `internal/service/dq.go`
- `internal/service/governance_event.go`
