# 治理联调测试清单

## 1. 测试目标

验证以下链路是否可用：

- `fuckcmdb -> cornerstone` 结构变更自动建单
- `fuckcmdb -> cornerstone` DQ 失败自动建单
- `cornerstone` 入站事件幂等

---

## 2. 前置条件

## 2.1 `cornerstone`

需要配置：

```env
INTEGRATION_SHARED_TOKEN=your-integration-token
```

确认：

- `POST /api/integrations/events` 已启动
- 可以正常登录并进入 `/governance`

## 2.2 `fuckcmdb`

需要配置：

```env
DATAMAP_GOVERNANCE_ENABLED=true
DATAMAP_GOVERNANCE_ENDPOINT=http://localhost:8081/api/integrations/events
DATAMAP_GOVERNANCE_INTEGRATION_TOKEN=your-integration-token
DATAMAP_GOVERNANCE_SOURCE_SYSTEM=fuckcmdb
DATAMAP_GOVERNANCE_TIMEOUT=5s
```

确认：

- 至少存在一个可同步的数据源
- 至少存在一条可执行且会失败的 DQ 规则

---

## 3. 测试步骤

## 3.1 结构变更自动建单

1. 启动 `cornerstone`
2. 启动 `fuckcmdb`
3. 在源库中制造一个 schema 变化
4. 在 `fuckcmdb` 触发该数据源的同步
5. 打开 `cornerstone /governance`

期望结果：

- 出现一条新任务
- 任务类型为 `schema_change`
- 来源系统为 `fuckcmdb`
- 任务详情中可看到外部资源引用

## 3.2 DQ 失败自动建单

1. 在 `fuckcmdb` 选择一条会失败的 DQ 规则
2. 执行 DQ check
3. 打开 `cornerstone /governance`

期望结果：

- 出现一条新任务
- 任务类型为 `dq_issue`
- 来源系统为 `fuckcmdb`
- 任务详情可看到 DQ 相关摘要

## 3.3 幂等验证

1. 找到 `fuckcmdb` 发出的某条事件对应的 `event_id`
2. 用相同 `event_id` 重放一次请求到 `cornerstone`

期望结果：

- `cornerstone` 不新增第二条任务
- 原任务保持不变

---

## 4. 排查重点

如果没有自动建单，优先检查：

- `cornerstone` 的 `INTEGRATION_SHARED_TOKEN` 是否与 `fuckcmdb` 的 `DATAMAP_GOVERNANCE_INTEGRATION_TOKEN` 一致
- `DATAMAP_GOVERNANCE_ENDPOINT` 是否正确
- `X-Source-System` 是否为 `fuckcmdb`
- `cornerstone` 是否已启动治理域相关路由
- `fuckcmdb` 是否真的检测到了 schema 变化或产生了失败的 DQ 结果

---

## 5. 当前已实现范围

已实现：

- `metadata.schema.changed`
- `dq.rule.failed`

已实现，待双端联调验证：

- `dq.alert.triggered`
- `ai.recommendation.generated`
- 审核通过后回写 `fuckcmdb`
