# Cornerstone - 双击运行说明

## 快速开始

### 方法 1: 双击运行（推荐）

1. **双击 `start.bat`** 启动程序
2. 等待几秒，程序会自动打开浏览器访问 http://localhost:8080
3. 如果浏览器没有自动打开，请手动访问 http://localhost:8080

### 方法 2: 命令行运行

```bash
cornerstone.exe
```

## 首次使用

默认会自动创建：
- SQLite 数据库文件 `cornerstone.db`
- 日志目录 `logs/`
- 默认管理员账户

访问 http://localhost:8080 后即可开始使用。

## 配置说明

如需修改配置，创建 `.env` 文件：

```env
# 数据库类型: sqlite 或 postgres
DB_TYPE=sqlite
DATABASE_URL=./cornerstone.db

# 服务器端口
PORT=8080

# JWT 密钥（建议修改）
JWT_SECRET=your-secret-key
```

## 常见问题

### 端口被占用

如果 8080 端口被占用，修改 `.env` 文件：
```env
PORT=8081
```

### 无法访问

检查是否被防火墙拦截，或尝试使用 `localhost` 代替 `127.0.0.1`。

### 数据存储位置

- SQLite 数据库：`cornerstone.db`（程序同级目录）
- 日志文件：`logs/app.log`
