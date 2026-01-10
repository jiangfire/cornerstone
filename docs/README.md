# Cornerstone - 硬件工程数据管理平台

**版本**: v1.3
**状态**: ✅ Sprint 1 完成 (P0 后端服务 100% 测试通过)
**最后更新**: 2026-01-09

---

## 🎯 项目概述

Cornerstone 是一个面向硬件工程师的私有化部署数据管理平台，提供类 Excel 的操作体验，同时具备数据库的强大能力。解决团队协作中数据分散、版本混乱的问题，并通过插件系统支持快速定制。

### 核心特性

- **📊 类 Excel 操作**：Web 界面轻松创建表、定义字段、录入数据
- **🗄️ 数据库能力**：PostgreSQL 存储，支持关联查询和分析
- **🔐 多租户架构**：个人数据库 + 组织数据库，支持团队协作
- **🔌 插件系统**：Go/Python 子进程隔离执行，5 秒超时控制
- **⚡ 性能优化**：GIN 索引、物化视图、JSONB 动态字段
- **👥 权限继承**：组织角色自动继承数据库权限

---

## 📚 文档导航

### 核心文档（5 个）

| 文档 | 说明 | 适用角色 |
|------|------|----------|
| **[REQUIREMENTS.md](./REQUIREMENTS.md)** | 产品需求、用户角色、MVP 范围、成功标准 | 产品经理、业务分析师 |
| **[ARCHITECTURE.md](./ARCHITECTURE.md)** | 技术架构、6 个 PlantUML 图表、设计决策 | 架构师、技术负责人 |
| **[DATABASE.md](./DATABASE.md)** | 13 张核心表设计、ER 图、SQL 脚本、性能优化 | 后端开发、DBA |
| **[API.md](./API.md)** | 100+ 接口规范、Go/Vue 代码示例 | 前后端开发 |
| **[DEVELOPMENT.md](./DEVELOPMENT.md)** | 开发计划、实施手册、Sprint 状态、执行步骤 | 开发团队 |

### 快速开始

```bash
# 1. 阅读需求
docs/REQUIREMENTS.md

# 2. 理解架构
docs/ARCHITECTURE.md

# 3. 数据库设计
docs/DATABASE.md

# 4. API 设计
docs/API.md

# 5. 开发执行
docs/DEVELOPMENT.md
```

---

## 🏗️ 技术栈

### 后端
- **语言**: Go 1.25+
- **框架**: Gin (Web) + GORM (ORM)
- **数据库**: PostgreSQL 15
- **存储**: MinIO / 本地文件系统

### 前端
- **框架**: Vue 3 + TypeScript
- **UI 库**: Element Plus
- **构建**: Vite

### 关键技术点
- ✅ UUID 主键（`usr_`, `db_`, `tbl_`, `rec_` 前缀）
- ✅ JSONB 动态字段存储
- ✅ GIN 索引加速 JSONB 查询
- ✅ 物化视图权限缓存（5 分钟刷新）
- ✅ 乐观锁（version）+ 编辑锁
- ✅ JWT + PostgreSQL 黑名单（SHA256）
- ✅ 插件子进程隔离 + 5 秒超时

---

## 📊 数据库设计概览

### 13 张核心表

```
用户与组织层：
├─ users                    (用户表)
├─ organizations            (组织表)
├─ organization_members     (组织成员表)

数据库与权限层：
├─ databases                (数据库表 - 支持双重模式)
├─ database_access          (数据库权限表)
├─ token_blacklist          (Token黑名单)

数据结构层：
├─ tables                   (表定义)
├─ fields                   (字段定义)
├─ records                  (业务数据 - JSONB)
├─ files                    (文件元数据)

插件系统层：
├─ plugins                  (插件配置)
├─ plugin_logs              (插件日志)

并发控制层：
└─ edit_locks               (编辑锁)
```

**详细设计**: 见 [DATABASE.md](./DATABASE.md)

---

## 🔐 权限模型

### 数据库模式

**个人数据库**：
- 用户创建 → 自动成为 owner
- 手动共享给其他用户（editor/viewer）

**组织数据库**：
- 组织创建 → 组织所有者自动拥有 owner 权限
- 组织管理员自动拥有 editor 权限
- 组织成员需手动授权
- 系统管理员拥有所有权限

### 权限矩阵

| 角色 | 表管理 | 字段管理 | 数据操作 | 插件管理 | 数据导出 |
|------|--------|----------|----------|----------|----------|
| **所有者** | ✅ | ✅ | ✅ | 查看日志 | 全量/分页 |
| **编辑者** | ✅ | ✅ | ✅ | ❌ | 分页导出 |
| **查看者** | ❌ | ❌ | 👁️ | ❌ | 分页导出 |
| **管理员** | 系统级 | 系统级 | 系统级 | 上传/启用/禁用/日志 | 系统级 |

---

## 🚀 开发状态

### ✅ Sprint 1 完成 (2026-01-09)

**已完成**:
- ✅ 用户认证（注册/登录/JWT）
- ✅ 组织管理（创建/成员/权限）
- ✅ 数据库/表/字段管理
- ✅ 基础 CRUD 操作
- ✅ 文件上传/下载
- ✅ P0 后端服务 100% 测试通过 (33/33)

**测试覆盖率**:
- 用户模块: 100% (12/12)
- 组织模块: 100% (8/8)
- 数据库模块: 100% (13/13)

### ⏳ 进行中 (Sprint 2)

- 权限系统（继承逻辑）
- 插件系统（子进程执行）
- 编辑锁（乐观锁）
- 数据导出
- API 文档完善

### 📅 计划中 (Sprint 3)

- 性能优化（索引/物化视图）
- 监控系统（Prometheus + Grafana）
- Docker 部署
- 测试覆盖

**详细进度**: 见 [DEVELOPMENT.md](./DEVELOPMENT.md)

---

## 💡 设计原则

### 核心原则
1. **调研优先** - 检索现有代码模式，识别复用机会
2. **三问原则** - 真问题？可复用？影响范围？
3. **红线原则** - 无重复、无破坏、无妥协、无盲从

### 架构特点
- **多租户支持**：个人 + 组织数据库隔离
- **权限继承**：数据库级权限自动继承
- **插件隔离**：子进程执行，异常不影响主进程
- **动态字段**：JSONB 存储，无需修改表结构
- **性能优化**：GIN 索引 + 物化视图 + 复合索引
- **并发控制**：乐观锁 + 编辑锁

---

## 🔗 快速链接

### 开发资源
- [API 接口列表](./API.md#接口模块概览)
- [数据库表结构](./DATABASE.md#核心表设计)
- [PlantUML 图表](./ARCHITECTURE.md#图表清单)
- [开发执行手册](./DEVELOPMENT.md#每日开发计划)

### 关键代码示例
- [JWT 认证](./API.md#1-用户认证模块)
- [组织权限继承](./ARCHITECTURE.md#权限继承流程)
- [插件执行](./ARCHITECTURE.md#插件执行流程)
- [JSONB 查询优化](./DATABASE.md#性能优化)

---

## 🎯 成功标准

1. **功能标准**：MVP 所有功能点开发完成并通过测试
2. **用户标准**：
   - 邀请 5-10 名硬件工程师种子用户
   - 创建至少 1 张与工作相关的表
   - 持续使用 1 个月，累积 300+ 条记录
   - 成功通过 API/导出提供给数据研究工程师
3. **插件标准**：IT 团队成功开发并部署至少 1 个演示插件

---

## 📞 下一步行动

1. **审核文档** - 确认设计符合需求
2. **环境搭建** - Docker + PostgreSQL
3. **项目初始化** - Go 后端 + Vue 前端
4. **开始 Sprint 2** - 按路线图开发

---

## 📦 交付物清单

```
c:\Users\yimo\Codes\cornerstone\docs\
├── README.md              # 本文档 - 项目总览
├── REQUIREMENTS.md        # 需求文档 (原 PRD.md + 规格)
├── ARCHITECTURE.md        # 技术架构 (6 个图表)
├── DATABASE.md            # 数据库设计 (完整)
├── API.md                 # API 接口 (100+ 接口)
└── DEVELOPMENT.md         # 开发手册 (计划 + 状态)
```

**已删除的冗余文档**:
- ❌ GUIDE.md (合并到 README.md + DEVELOPMENT.md)
- ❌ PLAN.md (合并到 DEVELOPMENT.md)
- ❌ IMPLEMENTATION-PLAN.md (合并到 DEVELOPMENT.md)
- ❌ PROJECT-STATUS.md (合并到 DEVELOPMENT.md)
- ❌ PRD.md (合并到 REQUIREMENTS.md)

---

**文档版本**: v1.0
**维护者**: 开发团队
**反馈**: 请通过 GitHub Issues 提交建议