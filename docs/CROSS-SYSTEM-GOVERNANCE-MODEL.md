# 跨系统数据治理模型与实现设计

## 1. 统一领域模型

为避免后续接口反复返工，先定义统一对象，并明确主归属系统。

| 对象 | 说明 | 主归属 |
|------|------|--------|
| `DataSource` | 数据源 | `fuckcmdb` |
| `DataAsset` | 表、视图、集合等资产 | `fuckcmdb` |
| `FieldAsset` | 字段资产 | `fuckcmdb` |
| `BusinessTerm` | 业务术语 | `fuckcmdb` |
| `Tag` | 标签 | `fuckcmdb` |
| `Classification` | 分类分级 | `fuckcmdb` |
| `DQRule` | 数据质量规则 | `fuckcmdb` |
| `DQResult` | 数据质量结果 | `fuckcmdb` |
| `LineageEdge` | 血缘边 | `fuckcmdb` |
| `GovernanceIssue` | 治理问题 | `cornerstone` 引用生成 |
| `GovernanceTask` | 治理任务 | `cornerstone` |
| `GovernanceReview` | 审批与审核 | `cornerstone` |
| `Evidence` | 整改证据 | `cornerstone` |
| `Owner` | 责任人、责任组织 | `cornerstone` |
| `Recommendation` | AI 推荐 | `fuckcmdb` 或 `cornerstone` 受控落库 |

## 2. ID 与引用规则

不共享表，但要共享可追踪的外部引用。

建议规则：

- 每个系统保持自己的主键
- 跨系统引用统一使用 `external_ref`
- `cornerstone` 保存 `source_system + resource_type + resource_id`

示例：

```json
{
  "source_system": "fuckcmdb",
  "resource_type": "column",
  "resource_id": "col_01JXYZ...",
  "display_name": "panel_id"
}
```

## 3. 服务间 token 模型

本方案不引入统一认证中心，只定义系统集成 token。

### 3.1 token 分类

- `user_token`
  - 仅在系统内部使用
  - 不允许直接跨系统透传并作为授权依据
- `integration_token`
  - 用于系统到系统调用
  - 按调用方单独分配

### 3.2 校验建议

- `cornerstone`、`fuckcmdb`、`LLM Governor` 各自维护集成 token 白名单
- 使用固定 token 即可，不必引入复杂 OAuth
- 请求头统一：

```http
Authorization: Bearer <integration_token>
X-Source-System: cornerstone
X-Trace-ID: 8a4d8f9d-a6a4-4a26-aad6-9d8f5b7c0001
```

### 3.3 权限粒度

建议配置三个级别：

- `read`
- `write`
- `review_apply`

例如：

- `LLM Governor -> fuckcmdb` 默认只有 `read`
- `cornerstone -> fuckcmdb` 可拥有 `review_apply`
- `fuckcmdb -> cornerstone` 可拥有 `write`，用于自动建单

## 4. 关键 API 边界

## 4.1 `fuckcmdb` 对 `cornerstone` / `LLM Governor`

建议保留并强化以下 API：

- `GET /api/v1/sources`
- `GET /api/v1/columns/search`
- `GET /api/v1/columns/:id`
- `GET /api/v1/columns/:id/lineage`
- `GET /api/v1/columns/:id/impact`
- `GET /api/v1/terms`
- `GET /api/v1/tags`
- `GET /api/v1/dq/rules`
- `GET /api/v1/dq/results`

新增建议：

- `POST /api/v1/recommendations/term-bindings`
- `POST /api/v1/recommendations/classifications`
- `POST /api/v1/recommendations/dq-rules`
- `POST /api/v1/recommendations/:id/approve`
- `POST /api/v1/recommendations/:id/reject`

## 4.2 `cornerstone` 对 `fuckcmdb` / `LLM Governor`

建议新增治理域 API：

- `POST /api/governance/tasks`
- `GET /api/governance/tasks`
- `GET /api/governance/tasks/:id`
- `PUT /api/governance/tasks/:id`
- `POST /api/governance/tasks/:id/evidences`
- `POST /api/governance/reviews`
- `POST /api/governance/reviews/:id/approve`
- `POST /api/governance/reviews/:id/reject`
- `POST /api/governance/assistant/analyze`
- `POST /api/governance/assistant/generate-plan`

## 5. 事件 schema

统一 JSON 结构：

```json
{
  "event_id": "evt_01JXYZ",
  "event_type": "dq.alert.triggered",
  "occurred_at": "2026-03-19T22:00:00Z",
  "source_system": "fuckcmdb",
  "resource_type": "dq_result",
  "resource_id": "dqr_01JXYZ",
  "actor_id": "system",
  "trace_id": "trc_01JXYZ",
  "payload": {}
}
```

事件要求：

- 幂等消费
- 至少一次投递
- 业务表 + outbox 同事务提交
- consumer 记录消费游标

## 6. `cornerstone` 建议新增表

### 6.1 `governance_tasks`

用于治理任务主表。

关键字段：

- `id`
- `title`
- `description`
- `task_type`
- `status`
- `priority`
- `source_system`
- `resource_type`
- `resource_id`
- `assignee_id`
- `review_id`
- `due_at`
- `created_by`
- `created_at`

### 6.2 `governance_reviews`

用于术语确认、分类确认、规则确认、设计审批。

关键字段：

- `id`
- `review_type`
- `status`
- `proposal_source`
- `proposal_payload`
- `decision_payload`
- `reviewer_id`
- `reviewed_at`

### 6.3 `governance_evidences`

用于整改证据、截图、SQL、附件引用。

关键字段：

- `id`
- `task_id`
- `evidence_type`
- `content`
- `file_id`
- `created_by`
- `created_at`

### 6.4 `governance_external_links`

用于跨系统资源引用。

关键字段：

- `id`
- `task_id`
- `source_system`
- `resource_type`
- `resource_id`
- `display_name`

### 6.5 `governance_comments`

用于任务内讨论和 AI 建议批注。

## 7. `fuckcmdb` 建议新增表

### 7.1 `classifications`

保存分类分级字典，例如：

- `pii`
- `financial_sensitive`
- `master_data`
- `core_metric`

### 7.2 `column_classifications`

字段与分类分级的绑定关系。

### 7.3 `recommendations`

统一保存 AI 推荐。

关键字段：

- `id`
- `recommendation_type`
- `target_resource_type`
- `target_resource_id`
- `payload`
- `confidence`
- `status`
- `generated_by`
- `approved_by`
- `approved_at`

### 7.4 `recommendation_reviews`

保存人工审核结论与理由。

### 7.5 `governance_policies`

保存命名规范、类型规范、敏感策略、DQ 模板等标准。

### 7.6 `event_outbox`

用于可靠发布事件。

## 8. LLM 输出结构

建议固定成如下格式：

```json
{
  "recommendation_type": "term_binding",
  "target_resource_type": "column",
  "target_resource_id": "col_01JXYZ",
  "confidence": 0.92,
  "reasoning_summary": "字段名与样本值高度匹配既有术语",
  "suggested_action": {
    "term_id": "term_01JXYZ"
  },
  "risk_level": "medium",
  "requires_human_review": true,
  "evidence_refs": [
    {
      "type": "sample_value",
      "value": "PANEL-2026-0001"
    }
  ]
}
```

强制要求：

- 只返回结构化结果
- 允许置信度低时拒答
- 默认 `requires_human_review = true`

## 9. 开发任务拆分建议

### 9.1 `cornerstone`

- 后端新增治理模型与路由
- 前端新增治理任务页、审批页、AI 建议审核页
- 增加事件 consumer
- 增加外部资源引用组件

### 9.2 `fuckcmdb`

- 增加 recommendation API
- 增加 classification API
- 增加 outbox publisher
- 增加集成 token 中间件

### 9.3 `LLM Governor`

- 建立上下文聚合层
- 建立 prompt 模板与输出校验
- 建立推荐生成与回写流程
- 建立失败重试与审计日志

## 10. 联调优先级

按顺序联调：

1. `fuckcmdb` 查询 API
2. `cornerstone` 任务 API
3. 事件 outbox -> consumer
4. `LLM Governor` 只读分析
5. recommendation 写入
6. 审核通过后的受控回写
