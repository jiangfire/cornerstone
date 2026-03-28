# Cornerstone - 双击运行说明

**当前平台**: Windows

## 快速开始

### 方法 1: 双击 `start.bat`

1. 双击 `start.bat` 启动程序
2. 等待几秒，程序会自动打开浏览器访问 http://localhost:8080
3. 如果浏览器没有自动打开，请手动访问 http://localhost:8080

特点：

- 会在前台显示启动状态
- 按任意键退出脚本时，会主动结束 `cornerstone.exe`
- 适合本地演示、临时验证和手工停止服务

### 方法 2: 双击 `Cornerstone.vbs`

1. 双击 `Cornerstone.vbs`
2. 脚本会无窗口后台启动 `cornerstone.exe`
3. 浏览器会自动打开 http://localhost:8080

特点：

- 无命令行窗口
- 关闭提示框不会结束服务
- 如需停止服务，需要在任务管理器中结束 `cornerstone.exe`

### 方法 3: 命令行运行

```bash
cornerstone.exe
```

如果你需要前台日志输出或要手工传入环境变量，优先用这种方式。

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

### 前置文件

- `cornerstone.exe`
- 可选：`.env`

如果缺少 `cornerstone.exe`，`start.bat` 和 `Cornerstone.vbs` 都无法启动。

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

### 健康检查

启动脚本会尝试访问：

```text
http://localhost:8080/health
```

如果浏览器已打开但页面暂时无法访问，通常是服务还在启动中，稍等几秒再刷新即可。
