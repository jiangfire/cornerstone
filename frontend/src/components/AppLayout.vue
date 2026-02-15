<template>
  <div class="app-layout">
    <div v-if="mobileMenuOpen" class="sidebar-overlay" @click="mobileMenuOpen = false"></div>
    <aside
      class="app-sidebar"
      :class="{ collapsed: sidebarCollapsed, 'mobile-open': mobileMenuOpen }"
    >
      <div class="sidebar-logo" @click="$router.push('/')">
        <span class="logo-icon">C</span>
        <span v-show="!sidebarCollapsed" class="logo-text">Cornerstone</span>
      </div>
      <el-menu
        :default-active="$route.path"
        router
        class="sidebar-menu"
        :collapse="sidebarCollapsed"
      >
        <el-menu-item index="/">
          <el-icon><HomeFilled /></el-icon>
          <template #title>工作台</template>
        </el-menu-item>
        <el-menu-item index="/organizations">
          <el-icon><OfficeBuilding /></el-icon>
          <template #title>组织管理</template>
        </el-menu-item>
        <el-menu-item index="/databases">
          <el-icon><Coin /></el-icon>
          <template #title>数据库</template>
        </el-menu-item>
        <el-menu-item index="/plugins">
          <el-icon><Connection /></el-icon>
          <template #title>插件管理</template>
        </el-menu-item>
        <el-menu-item index="/settings">
          <el-icon><Setting /></el-icon>
          <template #title>系统设置</template>
        </el-menu-item>
      </el-menu>
      <div class="sidebar-collapse-btn" @click="sidebarCollapsed = !sidebarCollapsed">
        <el-icon>
          <Fold v-if="!sidebarCollapsed" />
          <Expand v-else />
        </el-icon>
      </div>
    </aside>

    <div class="app-main">
      <header class="app-header">
        <div class="header-left">
          <button class="mobile-menu-btn" @click="mobileMenuOpen = !mobileMenuOpen">
            <el-icon :size="20"><Expand /></el-icon>
          </button>
          <h2 class="header-title">{{ title }}</h2>
        </div>
        <div class="header-actions">
          <el-dropdown @command="handleCommand" trigger="click">
            <span class="user-info">
              <el-avatar :size="32" :icon="UserFilled" />
              <span class="username">{{ authStore.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item command="profile">个人资料</el-dropdown-item>
                <el-dropdown-item command="settings">设置</el-dropdown-item>
                <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </div>
      </header>

      <main class="app-content">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import { usePageMeta } from '@/composables/usePageMeta'
import {
  UserFilled,
  ArrowDown,
  HomeFilled,
  OfficeBuilding,
  Coin,
  Connection,
  Setting,
  Fold,
  Expand,
} from '@element-plus/icons-vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()
const { title } = usePageMeta()

const sidebarCollapsed = ref(false)
const mobileMenuOpen = ref(false)

// Close mobile menu on navigation
watch(
  () => route.path,
  () => {
    mobileMenuOpen.value = false
  },
)

const handleCommand = (command: string) => {
  switch (command) {
    case 'logout':
      authStore.logout().then(() => {
        router.push('/login')
      })
      break
    case 'profile':
      router.push('/profile')
      break
    case 'settings':
      router.push('/settings')
      break
  }
}
</script>

<style scoped lang="scss">
.app-layout {
  display: flex;
  height: 100vh;
  overflow: hidden;
}

/* Sidebar */
.app-sidebar {
  display: flex;
  flex-direction: column;
  width: 220px;
  background: var(--fa-bg-sidebar);
  -webkit-backdrop-filter: var(--fa-blur-heavy);
  backdrop-filter: var(--fa-blur-heavy);
  border-right: var(--fa-border);
  transition: width 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  z-index: 20;

  &.collapsed {
    width: 64px;
  }
}

.sidebar-logo {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 20px 16px;
  cursor: pointer;
  user-select: none;
  border-bottom: var(--fa-border-subtle);
  min-height: 64px;
}

.logo-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  border-radius: var(--fa-radius-sm);
  background: var(--fa-accent);
  color: #fff;
  font-weight: 700;
  font-size: 16px;
  flex-shrink: 0;
}

.logo-text {
  font-size: 17px;
  font-weight: 600;
  color: var(--fa-text-primary);
  white-space: nowrap;
}

.sidebar-menu {
  flex: 1;
  border: none;
  background: transparent !important;
  overflow-y: auto;
  overflow-x: hidden;

  :deep(.el-menu-item) {
    color: var(--fa-text-secondary);
    border-radius: var(--fa-radius-sm);
    margin: 4px 8px;
    height: 44px;
    line-height: 44px;
    transition: all 0.2s;

    &:hover {
      background: rgba(255, 255, 255, 0.4);
      color: var(--fa-text-primary);
    }

    &.is-active {
      background: rgba(14, 165, 233, 0.12);
      color: var(--fa-accent-solid);
      font-weight: 600;
    }
  }

  :deep(.el-menu--collapse) {
    .el-menu-item {
      margin: 4px 6px;
      padding: 0 !important;
      justify-content: center;
    }
  }
}

.sidebar-collapse-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 12px;
  cursor: pointer;
  color: var(--fa-text-muted);
  border-top: var(--fa-border-subtle);
  transition: color 0.2s;

  &:hover {
    color: var(--fa-text-primary);
  }
}

/* Main */
.app-main {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-width: 0;
}

/* Header */
.app-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 24px;
  height: 64px;
  background: var(--fa-bg-header);
  -webkit-backdrop-filter: var(--fa-blur);
  backdrop-filter: var(--fa-blur);
  border-bottom: var(--fa-border-subtle);
  flex-shrink: 0;
}

.header-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mobile-menu-btn {
  display: none;
  background: none;
  border: none;
  cursor: pointer;
  color: var(--fa-text-secondary);
  padding: 4px;
  border-radius: var(--fa-radius-sm);

  &:hover {
    background: rgba(255, 255, 255, 0.4);
  }
}

.header-title {
  font-size: 18px;
  font-weight: 600;
  color: var(--fa-text-primary);
  margin: 0;
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: var(--fa-text-secondary);
  padding: 4px 8px;
  border-radius: var(--fa-radius-sm);
  transition: background 0.2s;

  &:hover {
    background: rgba(255, 255, 255, 0.4);
  }
}

.username {
  font-weight: 500;
  font-size: 14px;
}

/* Content */
.app-content {
  flex: 1;
  overflow-y: auto;
  padding: 24px;
}

/* Overlay */
.sidebar-overlay {
  display: none;
}

/* Responsive */
@media (max-width: 768px) {
  .sidebar-overlay {
    display: block;
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.3);
    z-index: 99;
  }

  .app-sidebar {
    position: fixed;
    left: 0;
    top: 0;
    height: 100vh;
    width: 220px;
    z-index: 100;
    box-shadow: var(--fa-shadow-lg);
    transform: translateX(-100%);
    transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);

    &.mobile-open {
      transform: translateX(0);
    }

    &.collapsed {
      width: 220px;
    }
  }

  .sidebar-collapse-btn {
    display: none;
  }

  .mobile-menu-btn {
    display: flex;
  }

  .app-content {
    padding: 16px;
  }
}
</style>
