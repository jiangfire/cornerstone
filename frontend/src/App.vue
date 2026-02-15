<script setup lang="ts">
import { RouterView, useRoute } from 'vue-router'
import { computed } from 'vue'
import AppLayout from '@/components/AppLayout.vue'

const route = useRoute()

const isAuthPage = computed(() => {
  return route.path === '/login' || route.path === '/register'
})
</script>

<template>
  <div id="app">
    <div v-if="isAuthPage" class="auth-layout">
      <RouterView />
    </div>
    <AppLayout v-else>
      <RouterView />
    </AppLayout>
  </div>
</template>

<style>
/* === Auth layout === */
.auth-layout {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  background: var(--fa-bg-auth);
  padding: 20px;
}

#app {
  width: 100%;
  min-height: 100vh;
}

/* === Element Plus: Glassmorphism Overrides === */

/* Cards */
.el-card {
  background: var(--fa-bg-surface);
  -webkit-backdrop-filter: var(--fa-blur);
  backdrop-filter: var(--fa-blur);
  border: var(--fa-border);
  border-radius: var(--fa-radius-lg);
  box-shadow: var(--fa-shadow-md);
}

.el-card__header {
  background: rgba(255, 255, 255, 0.3);
  border-bottom: var(--fa-border-subtle);
  font-weight: 600;
  color: var(--fa-text-primary);
}

/* Dialogs */
.el-dialog {
  background: var(--fa-bg-elevated);
  -webkit-backdrop-filter: var(--fa-blur-heavy);
  backdrop-filter: var(--fa-blur-heavy);
  border: var(--fa-border);
  border-radius: var(--fa-radius-lg);
  box-shadow: var(--fa-shadow-lg);
}

.el-dialog__header {
  background: rgba(255, 255, 255, 0.35);
  border-bottom: var(--fa-border-subtle);
  border-radius: var(--fa-radius-lg) var(--fa-radius-lg) 0 0;
}

.el-dialog__body {
  padding: 20px 24px;
}

/* Forms */
.el-form {
  background: transparent;
}

.el-form-item__label {
  font-weight: 500;
  color: var(--fa-text-secondary);
}

/* Inputs */
.el-input__wrapper {
  border-radius: var(--fa-radius-sm);
  background: rgba(255, 255, 255, 0.5);
  transition:
    box-shadow 0.2s,
    background 0.2s;
}

.el-input__wrapper:hover {
  box-shadow: 0 0 0 1px rgba(14, 165, 233, 0.3) inset;
}

.el-input__wrapper.is-focus {
  box-shadow: 0 0 0 1px var(--fa-accent-solid) inset;
  background: rgba(255, 255, 255, 0.7);
}

/* Primary buttons */
.el-button--primary {
  background: var(--fa-accent);
  border: none;
  color: #fff;
  font-weight: 500;
  transition: all 0.2s;
}

.el-button--primary:hover,
.el-button--primary:focus {
  background: var(--fa-accent-hover);
  border: none;
}

/* Tables */
.el-table {
  border-radius: var(--fa-radius-md);
  overflow: hidden;
  --el-table-bg-color: transparent;
  --el-table-tr-bg-color: transparent;
}

.el-table th {
  background: rgba(255, 255, 255, 0.4) !important;
  color: var(--fa-text-secondary);
  font-weight: 600;
}

.el-table td {
  border-bottom-color: rgba(255, 255, 255, 0.2);
}

.el-table--enable-row-hover .el-table__body tr:hover > td {
  background: rgba(255, 255, 255, 0.3) !important;
}

/* Tags */
.el-tag {
  font-weight: 500;
  border-radius: var(--fa-radius-sm);
}

/* Empty */
.el-empty__description {
  color: var(--fa-text-muted);
}

/* Pagination */
.el-pagination {
  margin-top: 16px;
  text-align: right;
}

/* Tabs */
.el-tabs__item {
  font-weight: 500;
}

.el-tabs__active-bar {
  background: var(--fa-accent-solid);
}

/* Dropdown */
.el-dropdown-menu {
  background: var(--fa-bg-elevated);
  -webkit-backdrop-filter: var(--fa-blur);
  backdrop-filter: var(--fa-blur);
  border: var(--fa-border);
  border-radius: var(--fa-radius-md);
}

.el-dropdown-menu__item {
  font-weight: 500;
}

.el-dropdown-menu__item:hover {
  background: rgba(14, 165, 233, 0.08);
  color: var(--fa-accent-solid);
}

/* Timeline */
.el-timeline-item__content {
  font-weight: 500;
}

.el-timeline-item__timestamp {
  color: var(--fa-text-muted);
  font-size: 12px;
}

/* Upload */
.el-upload {
  border: 1px dashed var(--fa-gray-300);
  border-radius: var(--fa-radius-sm);
  cursor: pointer;
  position: relative;
  overflow: hidden;
  transition: border-color 0.2s;
}

.el-upload:hover {
  border-color: var(--fa-accent-solid);
}

/* Scrollbar */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}

::-webkit-scrollbar-track {
  background: transparent;
}

::-webkit-scrollbar-thumb {
  background: rgba(14, 165, 233, 0.25);
  border-radius: 4px;
}

::-webkit-scrollbar-thumb:hover {
  background: rgba(14, 165, 233, 0.4);
}

/* Stat/action cards */
.stat-card .el-card__body {
  padding: 16px;
}

.action-card .el-card__body {
  padding: 20px;
  cursor: pointer;
}

/* Page transitions */
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.3s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}

/* Responsive */
@media (max-width: 768px) {
  .el-dialog {
    width: 90% !important;
    margin: 10vh auto !important;
  }
}

/* Utility classes */
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
