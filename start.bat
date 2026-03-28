@echo off
chcp 65001 >nul
echo.
echo ===========================================
echo    Cornerstone 数据管理平台
echo ===========================================
echo.

:: 检查程序文件
if not exist cornerstone.exe (
    echo [错误] 未找到 cornerstone.exe
    echo 请确保程序文件完整。
    pause
    exit /b 1
)

:: 启动程序
echo [信息] 正在启动服务...
echo [信息] 首次启动可能需要几秒...
echo.

start /B cornerstone.exe >nul 2>&1

:: 等待服务启动
timeout /t 2 /nobreak >nul

:: 检查端口是否可用
echo [信息] 正在检查服务状态...

:: 尝试访问本地端口
powershell -Command "try { $response = Invoke-WebRequest -Uri 'http://localhost:8080/health' -UseBasicParsing -TimeoutSec 5; if ($response.StatusCode -eq 200) { exit 0 } } catch { exit 1 }"

if %errorlevel% == 0 (
    echo [成功] 服务已启动！
    echo.
    echo 正在打开浏览器...
    start http://localhost:8080
    echo.
    echo ===========================================
    echo 访问地址: http://localhost:8080
    echo ===========================================
    echo.
    echo 按任意键关闭服务...
    pause >nul
    taskkill /F /IM cornerstone.exe >nul 2>&1
) else (
    echo [警告] 服务启动较慢，请稍等...
    timeout /t 3 /nobreak >nul
    start http://localhost:8080
    echo.
    echo 按任意键关闭服务...
    pause >nul
    taskkill /F /IM cornerstone.exe >nul 2>&1
)
