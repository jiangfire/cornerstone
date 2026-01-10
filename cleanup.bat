@echo off
REM Cornerstone 项目清理脚本 (Windows)
REM 用于删除临时文件、日志和编译产物

echo 开始清理 Cornerstone 项目...

REM 停止所有相关进程
echo 停止后端服务器...
taskkill /F /IM cornerstoned.exe 2>nul
taskkill /F /IM server.exe 2>nul
taskkill /F /IM go.exe 2>nul

REM 等待进程完全停止
timeout /t 2 /nobreak >nul

REM 删除日志文件
echo 清理日志文件...
if exist backend\logs rmdir /S /Q backend\logs
del /Q backend\*.log 2>nul

REM 删除二进制文件
echo 清理二进制文件...
del /Q backend\cornerstoned 2>nul
del /Q backend\*.exe 2>nul
if exist backend\bin rmdir /S /Q backend\bin

REM 删除测试数据库文件
echo 清理测试数据库...
del /Q backend\internal\services\test_*.db 2>nul
del /Q backend\*.db 2>nul

REM 删除临时文件
echo 清理临时文件...
del /Q nul 2>nul
del /Q *.tmp 2>nul

echo 清理完成！
echo.
echo 项目现在很干净了。
pause
