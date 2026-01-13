# Cornerstone 开发指南

**版本**: v1.7 | **最后更新**: 2026-01-11

---

## 项目概述

**Cornerstone** 是一个低代码数据管理平台，提供：
- 多租户数据库管理（个人 + 组织）
- 动态字段支持（JSONB存储）
- 三层权限模型（数据库/表/字段级权限）
- 插件扩展系统（Go/Python子进程）

### 技术栈

| 层级 | 技术 | 版本 |
|------|------|------|
| 后端 | Go + Gin + GORM | 1.21+ |
| 数据库 | PostgreSQL | 15+ |
| 前端 | Vue 3 + TypeScript | 3.4+ |
| UI库 | Element Plus | 2.5+ |
| 状态管理 | Pinia | 2.1+ |
| 构建工具 | Vite | 5.0+ |

---

## 快速开始

### 环境要求
- Go 1.21+
- PostgreSQL 15+ (推荐) 或 SQLite 3.35+
- Node.js 18+ (pnpm)

### 1. 克隆项目
```bash
git clone <repository-url>
cd cornerstone
```

### 2. 后端设置
```bash
cd backend

# 安装依赖
go mod download

# 配置环境变量
cp .env.example .env
# 编辑 .env 文件，设置数据库连接和 JWT 密钥

# 运行数据库迁移
go run ./cmd/server/main.go migrate

# 启动服务
go run ./cmd/server/main.go
```

后端服务将运行在 `http://localhost:8080`

### 3. 前端设置
```bash
cd frontend

# 安装依赖
pnpm install

# 配置环境变量
cp .env.example .env.local
# 编辑 .env.local 文件，设置 API 地址

# 启动开发服务器
pnpm dev
```

前端服务将运行在 `http://localhost:5173`

### 4. 验证安装
- 访问健康检查: `http://localhost:8080/health`
- 访问前端应用: `http://localhost:5173`
- 注册新用户并登录

---

## 项目结构

### 后端结构
```
backend/
├── cmd/server/              # 应用入口
│   └── main.go
├── internal/
│   ├── config/              # 配置管理 (12-Factor)
│   ├── handlers/            # API 处理器
│   ├── middleware/          # 中间件 (认证/日志/CORS)
│   ├── models/              # 数据模型 (13张表)
│   ├── services/            # 业务逻辑层
│   └── types/               # 类型定义
├── pkg/
│   ├── db/                  # 数据库连接管理
│   ├── log/                 # 日志系统
│   └── utils/               # 工具函数 (加密/JWT)
├── go.mod
├── go.sum
└── .env.example
```

### 前端结构
```
frontend/
├── src/
│   ├── router/              # 路由配置
│   ├── stores/              # Pinia 状态管理
│   ├── services/            # API 服务层
│   ├── views/               # 页面视图
│   ├── components/          # 通用组件
│   ├── assets/              # 静态资源
│   ├── App.vue              # 根组件
│   └── main.ts              # 入口文件
├── public/
├── package.json
├── vite.config.ts
├── tsconfig.json
└── .env.example
```

---

## 开发指南

### 后端开发

#### 添加新的 API 端点

1. **在 `internal/handlers/` 创建处理器**

```go
package handlers

import (
    "github.com/gin-gonic/gin"
    "cornerstone/pkg/utils"
)

type MyHandler struct {
    // 依赖注入
}

func NewMyHandler() *MyHandler {
    return &MyHandler{}
}

func (h *MyHandler) Create(c *gin.Context) {
    // 业务逻辑
    utils.SuccessResponse(c, "创建成功", data)
}
```

2. **在 `internal/services/` 创建服务层**

```go
package services

type MyService struct {
    db *gorm.DB
}

func (s *MyService) Create(data interface{}) error {
    // 数据库操作
    return nil
}
```

3. **在 `cmd/server/main.go` 注册路由**

```go
myHandler := handlers.NewMyHandler()
apiRoutes.POST("/my-resource", myHandler.Create)
```

#### 添加新的数据模型

在 `internal/models/models.go` 添加模型定义：

```go
type MyModel struct {
    ID        string    `gorm:"primaryKey" json:"id"`
    Name      string    `gorm:"size:255;not null" json:"name"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
```

运行迁移：
```bash
go run ./cmd/server/main.go migrate
```

### 前端开发

#### 添加新页面

1. **创建页面组件**

```typescript
// src/views/MyView.vue
<template>
  <div class="my-view">
    <h1>我的页面</h1>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

onMounted(() => {
  // 初始化逻辑
})
</script>
```

2. **添加路由配置**

```typescript
// src/router/index.ts
{
  path: '/my-page',
  name: 'MyPage',
  component: () => import('@/views/MyView.vue'),
  meta: { requiresAuth: true, title: '我的页面' }
}
```

#### 添加 API 方法

在 `src/services/api.ts` 添加：

```typescript
export const myAPI = {
  getList: (params: any) => request.get('/my-resource', { params }),
  create: (data: any) => request.post('/my-resource', data),
  update: (id: string, data: any) => request.put(`/my-resource/${id}`, data),
  delete: (id: string) => request.delete(`/my-resource/${id}`)
}
```

---

## 状态管理

### Pinia Store 示例

```typescript
// src/stores/myStore.ts
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useMyStore = defineStore('my', () => {
  const data = ref([])

  const fetchData = async () => {
    const response = await myAPI.getList()
    data.value = response.data
  }

  return { data, fetchData }
})
```

### 在组件中使用

```typescript
import { useMyStore } from '@/stores/myStore'

const myStore = useMyStore()
await myStore.fetchData()
```

---

## 测试指南

### 后端测试

```bash
cd backend

# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/services/...

# 查看测试覆盖率
go test -cover ./...
```

### 前端测试

```bash
cd frontend

# 运行单元测试
pnpm test

# 运行 E2E 测试
pnpm test:e2e
```

---

## 部署指南

### Docker 部署 (推荐)

创建 `docker-compose.yml`：

```yaml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: cornerstone
      POSTGRES_USER: user
      POSTGRES_PASSWORD: password
    volumes:
      - postgres_data:/var/lib/postgresql/data
    ports:
      - "5432:5432"

  backend:
    build: ./backend
    environment:
      DATABASE_URL: postgres://user:password@postgres:5432/cornerstone?sslmode=disable
      JWT_SECRET: ${JWT_SECRET}
      SERVER_MODE: release
    ports:
      - "8080:8080"
    depends_on:
      - postgres

volumes:
  postgres_data:
```

启动：
```bash
docker-compose up -d
```

### 生产环境检查清单

- [ ] 设置强 JWT 密钥
- [ ] 配置 PostgreSQL 连接池
- [ ] 启用 HTTPS/TLS
- [ ] 设置 `SERVER_MODE=release`
- [ ] 配置适当的日志
- [ ] 设置数据库备份
- [ ] 配置 CORS 来源限制

---

## 环境变量

### 后端环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `DATABASE_URL` | PostgreSQL 连接字符串 | - |
| `JWT_SECRET` | JWT 签名密钥 | - |
| `PORT` | 服务端口 | 8080 |
| `SERVER_MODE` | 运行模式 (debug/release) | debug |
| `LOG_LEVEL` | 日志级别 | info |

### 前端环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `VITE_API_BASE_URL` | API 基础地址 | http://localhost:8080/api |
| `VITE_APP_TITLE` | 应用标题 | Cornerstone |

---

## 常见问题

### Q: 如何重置数据库？
```bash
cd backend
go run ./cmd/server/main.go migrate-reset
go run ./cmd/server/main.go migrate
```

### Q: 如何查看日志？
- 后端日志: `backend/logs/app.log`
- 前端日志: 浏览器控制台

### Q: 如何调试 API？
使用 Postman 或 curl：
```bash
curl -X GET http://localhost:8080/health
```

### Q: 如何添加新的数据库表？
1. 在 `internal/models/models.go` 定义模型
2. 在 `internal/db/migrate.go` 添加迁移逻辑
3. 运行 `go run ./cmd/server/main.go migrate`

---

## 相关文档

- [API 文档](./API.md) - 完整 API 接口文档
- [权限系统](./PERMISSION-SYSTEM.md) - 三层权限模型详解
- [测试报告](./E2E-TEST-REPORT.md) - E2E 测试报告
- [项目状态](./PROJECT-STATUS.md) - 项目进度状态

---

## 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

---

## 许可证

本项目采用 GNU AGPL v3 许可证。
