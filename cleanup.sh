#!/bin/bash
# Cornerstone 项目清理脚本
# 用于删除临时文件、日志和编译产物

echo "开始清理 Cornerstone 项目..."

# 停止所有相关进程
echo "停止后端服务器..."
pkill -f "cornerstone" 2>/dev/null
pkill -f "go run" 2>/dev/null

# 等待进程完全停止
sleep 2

# 删除日志文件
echo "清理日志文件..."
rm -rf backend/logs/
rm -f backend/*.log

# 删除二进制文件
echo "清理二进制文件..."
rm -f backend/cornerstoned
rm -f backend/*.exe
rm -rf backend/bin/

# 删除测试数据库文件
echo "清理测试数据库..."
find backend/internal/services -name "test_*.db" -delete 2>/dev/null
find backend -name "*.db" -delete 2>/dev/null

# 删除临时文件
echo "清理临时文件..."
find . -name "nul" -delete 2>/dev/null
find . -name "*.tmp" -delete 2>/dev/null

echo "清理完成！"
echo ""
echo "项目现在很干净了。"
