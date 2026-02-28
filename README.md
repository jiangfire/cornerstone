# Cornerstone

> 把零散表格，升级成可交付、可协作、可扩展的数据产品。

[![Go Version](https://img.shields.io/badge/Go-1.25+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Vue Version](https://img.shields.io/badge/Vue-3.5+-4FC08D?style=flat&logo=vue.js&logoColor=white)](https://vuejs.org/)
[![PostgreSQL](https://img.shields.io/badge/PostgreSQL-15+-4169E1?style=flat&logo=postgresql)](https://www.postgresql.org/)
[![License](https://img.shields.io/badge/License-AGPL--3.0-blue.svg)](LICENSE)

Cornerstone 是一个面向团队的数据管理平台。  
你可以把它理解为：**“数据库 + 权限系统 + 业务工作台 + 插件扩展”** 的一体化底座。

---

## 一句话价值

用更少人力，在更短时间内，把内部数据流程做成可持续迭代的业务系统。

---

## 适合谁用

- 创业团队：希望快速上线内部系统，不想重复造后台
- 中小企业业务团队：表格过多、权限混乱、难协作
- 技术团队：需要一个可扩展、可管控的低代码数据底座
- 交付团队/外包团队：想标准化交付数据管理类项目

---

## 典型场景

### 1) 销售与客户管理

- 统一管理线索、客户、跟进记录、合同附件
- 按角色控制字段可见范围（例如金额、毛利）

### 2) 运营与项目协作

- 用表/字段快速建模任务、排期、里程碑
- 通过活动日志追踪“谁在什么时候改了什么”

### 3) 财务与行政流程

- 费用、采购、审批记录结构化管理
- 文件上传下载与记录绑定，归档更清晰

### 4) 内部工具产品化

- 从 Excel 原型直接过渡到可持续使用的系统
- 用插件机制接入自动化规则和外部能力

---

## 为什么是 Cornerstone

- 权限够细：数据库级、表级、字段级三级权限
- 结构够灵活：动态字段，不需要频繁改表结构
- 扩展够强：支持 Go/Python/Bash 插件
- 协作够稳：活动日志、权限边界、角色分工清晰

---

## 10 分钟体验（推荐 Docker）

### 1) 准备环境

- Docker Desktop（或 Docker Engine + Compose）

### 2) 一键启动

```bash
git clone https://github.com/jiangfire/cornerstone.git
cd cornerstone
docker compose up -d --build
```

### 3) 访问服务

- 前端：`http://localhost:3000`
- 后端 API：`http://localhost:8080/api`
- 健康检查：`http://localhost:8080/health`

### 4) 停止服务

```bash
docker compose down
```

---

## 本地开发启动（前后端分离）

### 环境要求

- Go `1.25+`
- PostgreSQL `15+`
- Node.js `20+`
- pnpm

### 后端

```bash
cd backend
cp .env.example .env
go mod download
go run ./cmd/server/main.go
```

后端默认地址：`http://localhost:8080`

### 前端

```bash
cd frontend
cp .env.example .env
pnpm install
pnpm dev
```

前端默认地址：`http://localhost:5173`

---

## 核心能力

- 用户认证：注册、登录、JWT、登出黑名单
- 组织协作：成员管理、角色管理
- 数据管理：数据库、表、字段、记录完整 CRUD
- 权限控制：owner/admin/editor/viewer + 字段级权限
- 批量处理：批量记录创建
- 文件系统：上传、下载、删除、关联记录
- 插件系统：创建、绑定、触发配置
- 统计分析：概览统计、近期活动

---

## API 与兼容性

- API 基础路径：`/api`
- 兼容路径：`/api/v1`
- API 文档：`docs/API.md`

---

## 上线前清单（生产建议）

- 将 `SERVER_MODE` 设为 `release`
- 使用强随机 `JWT_SECRET`
- 为 PostgreSQL 和日志目录启用持久化与备份
- 通过反向代理统一 HTTPS 与域名
- 配置基础监控（健康检查、错误日志、容量）

---

## 项目结构

```text
cornerstone/
  backend/    # Go + Gin + GORM
  frontend/   # Vue3 + TypeScript + Element Plus
  docs/       # API 与补充文档
```

---

## 现在就试

如果你在评估一套可长期维护的数据管理平台，可以直接跑起来：

```bash
docker compose up -d --build
```

你可以先从一个真实业务场景开始（例如“客户管理”或“项目协作”），1 天内验证可用性，1 周内完成首版上线。

---

## License

AGPL-3.0
