<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed } from 'vue'

const route = useRoute()

// 判断当前是否是认证相关页面
const isAuthPage = computed(() => {
  return route.path === '/login' || route.path === '/register'
})
</script>

<template>
  <div id="app">
    <!-- 认证页面使用简单布局 -->
    <div v-if="isAuthPage" class="auth-layout">
      <RouterView />
    </div>

    <!-- 主应用使用 Dashboard 布局 -->
    <div v-else class="main-layout">
      <RouterView />
    </div>
  </div>
</template>

<style>
/* 全局样式重置和基础样式 */
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif;
  background-color: #f5f5f5;
}

#app {
  width: 100%;
  height: 100vh;
}

.auth-layout {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
}

.main-layout {
  width: 100%;
  height: 100vh;
  overflow: hidden;
}

/* Element Plus 主题覆盖 */
.el-button--primary {
  background-color: #409eff;
  border-color: #409eff;
}

.el-button--primary:hover {
  background-color: #337ecc;
  border-color: #337ecc;
}

/* 滚动条样式 */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: #f1f1f1;
}

::-webkit-scrollbar-thumb {
  background: #c0c0c0;
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: #a0a0a0;
}

/* 表单样式 */
.el-form {
  background: white;
  padding: 24px;
  border-radius: 8px;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

/* 卡片样式 */
.el-card {
  border-radius: 8px;
  border: none;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.el-card__header {
  background: #f8f9fa;
  border-bottom: 1px solid #ebeef5;
  font-weight: 600;
  color: #303133;
}

/* 表格样式 */
.el-table {
  border-radius: 8px;
  overflow: hidden;
}

.el-table th {
  background: #f5f7fa;
  color: #606266;
  font-weight: 600;
}

/* 标签样式优化 */
.el-tag {
  font-weight: 500;
}

/* 空状态样式 */
.el-empty__description {
  color: #909399;
}

/* 对话框样式 */
.el-dialog {
  border-radius: 8px;
}

.el-dialog__header {
  background: #f8f9fa;
  border-bottom: 1px solid #ebeef5;
  border-radius: 8px 8px 0 0;
}

.el-dialog__body {
  padding: 20px 24px;
}

/* 按钮组间距 */
.el-button + .el-button {
  margin-left: 8px;
}

/* 表单项样式 */
.el-form-item__label {
  font-weight: 500;
  color: #606266;
}

/* 输入框样式 */
.el-input__wrapper {
  border-radius: 4px;
}

.el-input__wrapper:hover {
  box-shadow: 0 0 0 1px #c0c4cc inset;
}

.el-input__wrapper.is-focus {
  box-shadow: 0 0 0 1px #409eff inset;
}

/* 上传组件样式 */
.el-upload {
  border: 1px dashed #d9d9d9;
  border-radius: 6px;
  cursor: pointer;
  position: relative;
  overflow: hidden;
}

.el-upload:hover {
  border-color: #409eff;
}

/* 页码组件样式 */
.el-pagination {
  margin-top: 16px;
  text-align: right;
}

/* 标签页样式 */
.el-tabs__item {
  font-weight: 500;
}

.el-tabs__active-bar {
  background-color: #409eff;
}

/* 下拉菜单样式 */
.el-dropdown-menu__item {
  font-weight: 500;
}

.el-dropdown-menu__item:hover {
  background-color: #f5f7fa;
  color: #409eff;
}

/* 时间线样式 */
.el-timeline-item__content {
  font-weight: 500;
}

.el-timeline-item__timestamp {
  color: #909399;
  font-size: 12px;
}

/* 统计卡片样式 */
.stat-card .el-card__body {
  padding: 16px;
}

.action-card .el-card__body {
  padding: 20px;
  cursor: pointer;
}

/* 响应式工具类 */
@media (max-width: 768px) {
  .el-col-xs-24 {
    width: 100%;
  }

  .el-dialog {
    width: 90% !important;
    margin: 10vh auto !important;
  }

  .el-form {
    padding: 16px;
  }
}

/* 加载状态 */
.loading-overlay {
  position: fixed;
  top: 0;
  left: 0;
  width: 100%;
  height: 100%;
  background: rgba(255, 255, 255, 0.8);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

/* 页面过渡动画 */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* 工具类 */
.text-center {
  text-align: center;
}

.mt-2 {
  margin-top: 8px;
}

.mt-4 {
  margin-top: 16px;
}

.mb-2 {
  margin-bottom: 8px;
}

.mb-4 {
  margin-bottom: 16px;
}

.w-full {
  width: 100%;
}

.flex {
  display: flex;
}

.items-center {
  align-items: center;
}

.justify-between {
  justify-content: space-between;
}

.gap-2 {
  gap: 8px;
}
</style>
