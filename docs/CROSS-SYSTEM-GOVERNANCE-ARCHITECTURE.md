# 跨系统数据治理总体方案

## 1. 目标

本方案用于协同：

- `cornerstone`：业务执行与流程中台
- `fuckcmdb`：元数据治理与数据标准中台
- `LLM Governor`：独立的智能治理服务

最终目标不是把两套系统合并，而是构建一个统一的数据治理操作平面：

- 元数据真相在 `fuckcmdb`
- 流程真相在 `cornerstone`
- AI 负责建议、解释、生成，不直接绕过人工修改治理主数据
- 系统之间通过 API 与事件协作，不共享数据库表

## 2. 已确认架构决策

### 2.1 身份与认证

- 不建设统一认证中心
- 两个系统保留各自登录体系
- 用户不是同一批人，不要求单点登录
- 跨系统调用采用服务间 `token` 校验

建议区分两类 token：

- 用户 token：仅在本系统内使用
- 集成 token：系统对系统调用时使用，例如 `cornerstone -> fuckcmdb`

### 2.2 集成方式

- 采用事件驱动
- 保留同步查询 API，用于详情拉取、确认写回、页面联查
- 不做数据库级耦合

### 2.3 AI 定位

- AI 是“建议器”和“编排器”，不是直接执行器
- 默认输出 `draft / proposed / approved / rejected / applied`
- 所有落治理主数据的动作，必须经过受控 API 和审计

## 3. 系统职责边界

## 3.1 `fuckcmdb`

负责：

- 数据源接入与扫描
- Schema、字段、术语、标签、血缘
- 分类分级
- DQ 规则与结果
- 告警规则与治理策略

不负责：

- 治理工单
- 审批流程
- 整改证据
- 责任人跟踪

## 3.2 `cornerstone`

负责：

- 组织、成员、数据库级/字段级权限
- 治理任务、审批、整改、证据、评论、活动日志
- 治理驾驶舱与执行工作台
- 通过插件或服务调用外部治理能力

不负责：

- 元数据主档
- 血缘主档
- DQ 主规则存储

## 3.3 `LLM Governor`

负责：

- 术语推荐与字段归一
- 敏感字段识别与分类分级建议
- DQ 规则建议
- 变更影响解释
- 整改任务建议与优先级编排
- 设计阶段标准校验建议

## 4. 最终业务闭环

### 4.1 新系统设计准入

1. 在 `cornerstone` 发起新系统、新表或字段设计申请
2. `cornerstone` 调用 `LLM Governor`
3. `LLM Governor` 查询 `fuckcmdb` 的术语、字段、标准、DQ 规则
4. 返回命名、类型、敏感级别、标准术语和 DDL 建议
5. 审批通过后，由 `fuckcmdb` 固化标准对象，`cornerstone` 记录审批链路

### 4.2 存量系统持续治理

1. `fuckcmdb` 扫描数据源、检测变更、执行 DQ
2. 发现异常后发布事件
3. `cornerstone` 消费事件并自动生成治理任务
4. `LLM Governor` 生成影响摘要、整改建议、优先级和责任建议
5. 人工处理完成后，结果回写 `fuckcmdb`

### 4.3 语义治理

1. `LLM Governor` 对字段做术语聚类和映射推荐
2. 推荐进入待审核状态
3. 审核通过后写入 `fuckcmdb`
4. `cornerstone` 保存审核意见、责任人和证据

## 5. 集成架构

```text
Users -> Cornerstone UI -> Cornerstone API
Users -> FuckCMDB UI -> FuckCMDB API

Cornerstone API <-> LLM Governor
FuckCMDB API <-> LLM Governor

FuckCMDB -> Event Bus -> Cornerstone
Cornerstone -> Event Bus -> FuckCMDB
LLM Governor -> Event Bus -> Cornerstone / FuckCMDB
```

建议引入 3 个技术组件：

- `event bus`：NATS、RabbitMQ 或 Kafka，优先选简单可运维方案
- `outbox`：两个系统都使用本地 outbox，避免事件丢失
- `integration token validator`：统一封装服务间 token 校验中间件

## 6. 跨系统认证方案

由于不做统一认证，建议采用“双登录 + 服务间 token”模型。

### 6.1 用户侧

- 用户登录 `cornerstone`，只获得 `cornerstone` 的用户 token
- 用户登录 `fuckcmdb`，只获得 `fuckcmdb` 的用户 token
- 两边用户体系互不打通

### 6.2 服务侧

系统间调用使用集成 token：

- `Authorization: Bearer <integration_token>`
- `X-Source-System: cornerstone | fuckcmdb | llm-governor`
- `X-Trace-ID: <uuid>`

校验规则建议：

- token 由目标系统本地配置的白名单校验
- 每个调用方单独配置 token
- 区分读权限、写权限、事件回写权限

## 7. `cornerstone` 需要新增的模块

- 治理任务中心
- 治理审批中心
- 整改证据中心
- 事件消费器
- 外部治理资源引用能力
- LLM 建议审核面板
- 治理看板与 SLA 面板

## 8. `fuckcmdb` 需要新增的模块

- 资源级权限补强
- 分类分级模型
- 推荐写入与确认接口
- 事件发布器
- 策略中心
- 面向 `cornerstone` 和 `LLM Governor` 的集成 API

## 9. `LLM Governor` 设计要求

- 独立部署，不嵌进任一现有服务
- 所有输出必须为结构化 JSON
- 不直接连业务源数据库
- 不直接修改 `fuckcmdb` 主表
- 所有执行型动作必须经过目标系统 API

核心能力：

- `semantic-mapper`
- `classifier`
- `dq-designer`
- `impact-explainer`
- `task-orchestrator`
- `policy-advisor`

## 10. 事件模型

建议的主题：

- `metadata.source.synced`
- `metadata.schema.changed`
- `metadata.term.bound`
- `metadata.classification.updated`
- `dq.rule.failed`
- `dq.alert.triggered`
- `governance.task.created`
- `governance.task.completed`
- `governance.review.approved`
- `governance.review.rejected`
- `ai.recommendation.generated`
- `ai.recommendation.applied`

统一字段：

- `event_id`
- `event_type`
- `occurred_at`
- `source_system`
- `resource_type`
- `resource_id`
- `actor_id`
- `trace_id`
- `payload`

## 11. 实施顺序

### 第 1 阶段：架构定版

- 统一领域模型
- 统一 ID 规则
- 定义 API 合同
- 定义事件 schema
- 定义集成 token 规则

### 第 2 阶段：改造 `fuckcmdb`

- 增加事件 outbox
- 增加推荐确认接口
- 增加分类分级与策略接口
- 增加集成 token 校验

### 第 3 阶段：改造 `cornerstone`

- 增加治理任务域模型
- 增加审批、证据、SLA 和评论
- 增加事件 consumer
- 增加 LLM 建议审核面板

### 第 4 阶段：建设 `LLM Governor`

- 接入 `fuckcmdb` 元数据
- 接入 `cornerstone` 流程上下文
- 输出结构化建议
- 建立人工审核与回写闭环

### 第 5 阶段：联调与上线

- 联调 API 和事件
- 跑通“异常 -> 建单 -> 整改 -> 回写”闭环
- 跑通“设计申请 -> AI 建议 -> 审批 -> 标准落地”闭环

## 12. 验收标准

满足以下条件才算完成：

- 两套系统无数据库耦合
- 任一 schema 变化可触发治理事件
- 任一 DQ 异常可自动生成治理任务
- AI 建议可审核、可拒绝、可回溯
- 元数据主档只在 `fuckcmdb`
- 治理流程主档只在 `cornerstone`
