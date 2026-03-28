# 跨系统 LLM 数据治理实施清单

## 1. 文档目标

本清单用于指导以下三套能力按可落地顺序推进：

- `cornerstone`：治理流程与执行中台
- `fuckcmdb`：元数据、术语、血缘、DQ 与标准中台
- `LLM Governor`：智能建议与解释服务

目标不是一次性做完整平台，而是先打通最小闭环，再逐步增强可靠性和自动化能力。

---

## 2. 今天的结论

### 2.1 已有基础

`cornerstone` 已经具备以下能力：

- 治理任务、审核、证据、评论、外部资源引用模型
- `POST /api/integrations/events` 入站事件接口
- 集成 token 校验
- 入站事件幂等处理与自动建单
- 治理任务中心前端页面

关键文件：

- `backend/internal/services/integration_events.go`
- `backend/internal/services/governance.go`
- `backend/internal/middleware/integration.go`
- `frontend/src/views/GovernanceView.vue`

`fuckcmdb` 已经具备以下能力：

- 数据源接入与 schema 扫描
- 结构变更检测与 `schema_changes` 持久化
- 业务术语、标签、血缘、字段搜索
- DQ 规则、执行与结果持久化
- 告警规则与 webhook 发送能力

关键文件：

- `internal/service/source.go`
- `internal/service/dq.go`
- `internal/service/alert.go`
- `internal/api/router.go`

### 2.2 当前最大缺口

截至 2026-03-20，`fuckcmdb` 已经补上第一批标准事件发送能力，但仍然缺完整集成层：

- 缺少面向 `cornerstone` / `LLM Governor` 的集成鉴权中间件
- 缺少审核通过后的受控回写接口
- 缺少 AI 推荐结果的标准落库结构
- 缺少只读集成 API 与更完整的外部上下文查询

### 2.3 推荐策略

先走 HTTP 事件推送，后补消息总线。

原因：

- `fuckcmdb` 已经有 webhook 能力，可复用现有实现
- `cornerstone` 已经有入站事件接口
- 先打通闭环，比先引入 Kafka / NATS 更有价值

### 2.4 当前进展

当前已经完成：

- `fuckcmdb` 统一治理事件发送器
- `metadata.schema.changed` 出站事件
- `dq.rule.failed` 出站事件
- `DATAMAP_GOVERNANCE_*` 配置项

下一步重点已经从“补发送器”切换为“联调验证 + 补只读集成层”。

---

## 3. 总体实施路线

按五个阶段推进：

1. 打通最小治理闭环
2. 补齐 `fuckcmdb` 集成层
3. 接入 `LLM Governor` 只读建议
4. 建立审核通过后的受控回写
5. 增强为 outbox + event bus 的可靠架构

---

## 4. 阶段拆解

## 4.1 第一阶段：最小闭环

目标：跑通“发现问题 -> 自动建单 -> 人工处理”。

### `fuckcmdb` 要做

- 在结构同步完成后发送 `metadata.schema.changed` 。
  当前状态：已完成
- 在 DQ 执行失败后发送 `dq.rule.failed`
  当前状态：已完成
- 可选增加 `dq.alert.triggered`，用于更偏告警场景的任务触发
- 统一事件发送器，目标地址指向 `cornerstone /api/integrations/events`
  当前状态：已完成

建议改动位置：

- `internal/service/source.go`
- `internal/service/dq.go`
- 新增 `internal/integration/events/` 或 `internal/service/integration_event.go`
- `internal/config/config.go`

### `cornerstone` 要做

- 复用现有入站事件处理
- 校对事件 payload 字段，确保标题、摘要、优先级映射合理
- 在治理任务详情里展示更多外部资源上下文

建议改动位置：

- `backend/internal/services/integration_events.go`
- `frontend/src/views/GovernanceView.vue`

### 验收标准

- 在 `fuckcmdb` 执行一次 schema sync 后，`cornerstone` 自动生成结构变更任务
- 在 `fuckcmdb` 执行一次 DQ check 且失败后，`cornerstone` 自动生成 DQ 任务
- 相同 `event_id` 重放不会重复建单

### 联调配置

`fuckcmdb`：

```env
DATAMAP_GOVERNANCE_ENABLED=true
DATAMAP_GOVERNANCE_ENDPOINT=http://localhost:8081/api/integrations/events
DATAMAP_GOVERNANCE_INTEGRATION_TOKEN=your-integration-token
DATAMAP_GOVERNANCE_SOURCE_SYSTEM=fuckcmdb
DATAMAP_GOVERNANCE_TIMEOUT=5s
```

`cornerstone`：

```env
INTEGRATION_SHARED_TOKEN=your-integration-token
```

## 4.2 第二阶段：`fuckcmdb` 集成层

目标：把当前“业务 API”与“系统对系统 API”分开。

### `fuckcmdb` 要做

- 增加 integration token 中间件
- 新增只读集成 API
- 区分用户 token 与集成 token
- 为外部查询增加 `trace_id` 透传

建议新增接口：

- `GET /api/integration/v1/columns/:id`
- `GET /api/integration/v1/columns/:id/lineage`
- `GET /api/integration/v1/columns/:id/impact`
- `GET /api/integration/v1/terms`
- `GET /api/integration/v1/tags`
- `GET /api/integration/v1/dq/rules`
- `GET /api/integration/v1/dq/results`

建议改动位置：

- `internal/api/router.go`
- 新增 `internal/api/integration_handler.go`
- 新增 `internal/api/integration_middleware.go`
- `internal/config/config.go`

### `cornerstone` 要做

- 为任务详情增加“拉取外部详情”的只读能力
- 支持从外部资源引用跳转到元数据详情

### 验收标准

- `cornerstone` 能通过集成 token 安全查询 `fuckcmdb` 的字段、术语、血缘和 DQ 信息
- 用户 token 不可直接调用集成接口

## 4.3 第三阶段：接入 `LLM Governor`

目标：让 AI 先做建议器，不做执行器。

### `LLM Governor` 要做

- 聚合 `fuckcmdb` 元数据上下文
- 聚合 `cornerstone` 流程上下文
- 输出结构化 JSON 建议
- 为低置信度场景允许拒答

建议能力：

- `semantic-mapper`
- `classifier`
- `dq-designer`
- `impact-explainer`
- `task-orchestrator`

### 输入来源

- 来自 `fuckcmdb` 的字段、术语、DQ、血缘、分类规则
- 来自 `cornerstone` 的任务、责任人、历史评论、审核结论

### 输出建议类型

- `term_binding`
- `classification`
- `dq_rule`
- `impact_summary`
- `remediation_plan`

### `cornerstone` 要做

- 新增 AI 建议审核入口
- 将 AI 建议作为 `governance_review.proposal_payload` 的来源之一

### `fuckcmdb` 要做

- 先只提供只读上下文查询
- 不允许 AI 直接修改主表

### 验收标准

- 可针对指定字段生成术语推荐
- 可针对结构变更任务生成影响摘要和整改建议
- 所有 AI 建议都必须经过人工审核

## 4.4 第四阶段：审核通过后的受控回写

目标：跑通“建议 -> 审核 -> 落标准”的闭环。

### `fuckcmdb` 要做

- 新增 recommendation / review_apply 接口
- 为术语绑定、分类分级、DQ 规则确认提供受控写接口
- 每次写入保留审计信息和来源系统

建议新增接口：

- `POST /api/integration/v1/recommendations/term-bindings`
- `POST /api/integration/v1/recommendations/classifications`
- `POST /api/integration/v1/recommendations/dq-rules`
- `POST /api/integration/v1/recommendations/:id/approve`
- `POST /api/integration/v1/recommendations/:id/reject`

### `cornerstone` 要做

- 在审核通过时调用 `fuckcmdb` 回写接口
- 把回写结果记录到任务评论、证据或活动日志中
- 对回写失败场景保留重试入口

建议改动位置：

- 新增 `backend/internal/services/governance_apply.go`
- 新增 `backend/internal/handlers/governance_apply.go`

### 验收标准

- 审核通过后可把术语绑定安全回写到 `fuckcmdb`
- 审核拒绝后不会修改 `fuckcmdb`
- 回写结果可追踪到任务、审核和外部资源

## 4.5 第五阶段：可靠性增强

目标：从“可用”升级到“稳定可运维”。

### 要做的事

- 两边都引入 `event_outbox`
- 统一消费状态表或去重表
- 从 HTTP 直推升级到 event bus
- 增加失败重试、死信、告警、观测指标

### 推荐优先级

- 先 outbox
- 再总线
- 最后再做多订阅方编排

### 验收标准

- 系统重启、网络抖动后事件不丢
- 事件重复投递不会造成重复建单和重复落库

---

## 5. 三个系统的明确边界

## 5.1 `cornerstone`

负责：

- 治理任务
- 审核流转
- 责任人和组织协同
- 评论、证据、活动日志
- 审核通过后的受控回写编排

不负责：

- 元数据主档
- 术语主档
- 血缘主档
- DQ 规则主存储

## 5.2 `fuckcmdb`

负责：

- 数据源与 schema
- 字段、术语、标签、血缘
- 分类分级
- DQ 规则与结果
- 推荐结果落库与标准确认

不负责：

- 治理工单
- 人工整改过程管理
- 审批流与责任跟踪

## 5.3 `LLM Governor`

负责：

- 建议
- 解释
- 编排草案
- 置信度与风险评估

不负责：

- 直接写主数据
- 直接访问业务源库
- 绕过人工审核执行动作

---

## 6. 统一事件合同

第一阶段就应统一以下字段：

```json
{
  "event_id": "evt_xxx",
  "event_type": "metadata.schema.changed",
  "occurred_at": "2026-03-20T10:00:00Z",
  "source_system": "fuckcmdb",
  "resource_type": "column",
  "resource_id": "col_xxx",
  "actor_id": "system",
  "trace_id": "trc_xxx",
  "payload": {}
}
```

第一批事件类型：

- `metadata.schema.changed`
- `dq.rule.failed`
- `dq.alert.triggered`
- `ai.recommendation.generated`
- `governance.review.approved`
- `governance.review.rejected`

`payload` 最低建议字段：

- `title`
- `summary`
- `display_name`
- `priority`
- `change_type`
- `recommendation_type`
- `assignee_id`

---

## 7. 统一接口与权限约束

建议保留三类权限：

- `read`
- `write`
- `review_apply`

推荐授权关系：

- `fuckcmdb -> cornerstone`：`write`
- `cornerstone -> fuckcmdb`：`read` + `review_apply`
- `LLM Governor -> fuckcmdb`：`read`
- `LLM Governor -> cornerstone`：`read`

---

## 8. 建议的数据结构补强

## 8.1 `fuckcmdb`

建议新增：

- `recommendations`
- `recommendation_reviews`
- `classifications`
- `column_classifications`
- `governance_policies`
- `event_outbox`

## 8.2 `cornerstone`

当前表已足够支撑一期到三期。

建议后续补充：

- `governance_outbox`
- `governance_task_sla_logs`
- `governance_assistant_runs`

---

## 9. 近两周执行清单

## 9.1 第 1 周

- 在 `fuckcmdb` 抽出统一事件发送器
- 从 `source.go` 发出 `metadata.schema.changed`
- 从 `dq.go` 发出 `dq.rule.failed`
- 在本地环境配置 `cornerstone` 集成 token
- 联调 `POST /api/integrations/events`

交付结果：

- 本地可以自动生成两类治理任务

## 9.2 第 2 周

- 在 `fuckcmdb` 增加 integration token 中间件
- 增加集成只读 API
- `cornerstone` 任务详情联查外部资源
- 定义 `LLM Governor` 输入输出 schema

交付结果：

- 任务详情可以看到外部字段、术语、血缘、DQ 上下文

---

## 10. 验收标准

达到以下条件，才算这条路线真正成立：

- 两个系统没有数据库级耦合
- `fuckcmdb` 的 schema 变化可以自动进入 `cornerstone`
- `fuckcmdb` 的 DQ 异常可以自动进入 `cornerstone`
- AI 输出只能以结构化建议形式进入审核，不可直接改主数据
- 审核通过后的修改只能通过受控 API 回写
- 所有关键动作都有 `trace_id`、审计记录和幂等保护

---

## 11. 下一步建议

如果按当前投入产出比排序，建议下一步直接开始做下面两件事：

1. 在 `fuckcmdb` 实现标准事件发送器，并接入 `metadata.schema.changed`、`dq.rule.failed`
2. 为 `fuckcmdb` 增加集成鉴权与只读集成 API

这两步做完，再接 `LLM Governor`，整体风险最低，返工也最少。
