<template>
  <div class="dashboard">
    <!-- 顶部导航栏 -->
    <el-header class="header">
      <div class="header-content">
        <div class="logo">
          <h3>🔧 Cornerstone</h3>
        </div>
        <div class="nav-actions">
          <el-dropdown @command="handleCommand">
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
      </div>
    </el-header>

    <el-container class="main-container">
      <!-- 侧边栏 -->
      <el-aside width="200px" class="sidebar">
        <el-menu
          :default-active="$route.path"
          router
          class="menu"
          background-color="#545c64"
          text-color="#fff"
          active-text-color="#409eff"
        >
          <el-menu-item index="/">
            <el-icon><HomeFilled /></el-icon>
            <span>工作台</span>
          </el-menu-item>

          <el-menu-item index="/organizations">
            <el-icon><OfficeBuilding /></el-icon>
            <span>组织管理</span>
          </el-menu-item>

          <el-menu-item index="/databases">
            <el-icon><Database /></el-icon>
            <span>数据库</span>
          </el-menu-item>

          <el-menu-item index="/plugins">
            <el-icon><Connection /></el-icon>
            <span>插件管理</span>
          </el-menu-item>

          <el-menu-item index="/settings">
            <el-icon><Setting /></el-icon>
            <span>系统设置</span>
          </el-menu-item>
        </el-menu>
      </el-aside>

      <!-- 主内容区 -->
      <el-main class="content">
        <div class="content-header">
          <h2>{{ pageTitle }}</h2>
          <p class="description">{{ pageDescription }}</p>
        </div>

        <div class="content-body">
          <!-- 统计卡片 -->
          <el-row :gutter="20" class="stats-row">
            <el-col :xs="24" :sm="12" :md="6">
              <el-card class="stat-card" shadow="hover">
                <div class="stat-content">
                  <div class="stat-icon" style="background: #409eff;">
                    <User />
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ stats.users }}</div>
                    <div class="stat-label">总用户数</div>
                  </div>
                </div>
              </el-card>
            </el-col>

            <el-col :xs="24" :sm="12" :md="6">
              <el-card class="stat-card" shadow="hover">
                <div class="stat-content">
                  <div class="stat-icon" style="background: #67c23a;">
                    <OfficeBuilding />
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ stats.organizations }}</div>
                    <div class="stat-label">组织数量</div>
                  </div>
                </div>
              </el-card>
            </el-col>

            <el-col :xs="24" :sm="12" :md="6">
              <el-card class="stat-card" shadow="hover">
                <div class="stat-content">
                  <div class="stat-icon" style="background: #e6a23c;">
                    <DataLine />
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ stats.databases }}</div>
                    <div class="stat-label">数据库数量</div>
                  </div>
                </div>
              </el-card>
            </el-col>

            <el-col :xs="24" :sm="12" :md="6">
              <el-card class="stat-card" shadow="hover">
                <div class="stat-content">
                  <div class="stat-icon" style="background: #f56c6c;">
                    <Connection />
                  </div>
                  <div class="stat-info">
                    <div class="stat-value">{{ stats.plugins }}</div>
                    <div class="stat-label">插件数量</div>
                  </div>
                </div>
              </el-card>
            </el-col>
          </el-row>

          <!-- 快捷操作 -->
          <div class="quick-actions">
            <h3>快捷操作</h3>
            <el-row :gutter="20">
              <el-col :xs="24" :sm="12" :md="8">
                <el-card class="action-card" @click="$router.push('/organizations')">
                  <div class="action-content">
                    <el-icon><Plus /></el-icon>
                    <span>创建组织</span>
                  </div>
                </el-card>
              </el-col>
              <el-col :xs="24" :sm="12" :md="8">
                <el-card class="action-card" @click="$router.push('/databases')">
                  <div class="action-content">
                    <el-icon><Plus /></el-icon>
                    <span>新建数据库</span>
                  </div>
                </el-card>
              </el-col>
              <el-col :xs="24" :sm="12" :md="8">
                <el-card class="action-card" @click="$router.push('/plugins')">
                  <div class="action-content">
                    <el-icon><Plus /></el-icon>
                    <span>安装插件</span>
                  </div>
                </el-card>
              </el-col>
            </el-row>
          </div>

          <!-- 最近活动 -->
          <div class="recent-activity">
            <h3>最近活动</h3>
            <el-empty v-if="activities.length === 0" description="暂无活动记录" />
            <el-timeline v-else>
              <el-timeline-item
                v-for="(activity, index) in activities"
                :key="index"
                :type="activity.type"
                :timestamp="activity.time"
              >
                {{ activity.content }}
              </el-timeline-item>
            </el-timeline>
          </div>
        </div>
      </el-main>
    </el-container>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import {
  UserFilled,
  ArrowDown,
  HomeFilled,
  OfficeBuilding,
  DataLine,
  Connection,
  Setting,
  User,
  Plus,
} from '@element-plus/icons-vue'

const router = useRouter()
const authStore = useAuthStore()

// 页面标题和描述
const pageTitle = computed(() => {
  const path = router.currentRoute.value.path
  const titles: Record<string, string> = {
    '/': '工作台',
    '/organizations': '组织管理',
    '/databases': '数据库管理',
    '/plugins': '插件管理',
    '/settings': '系统设置',
  }
  return titles[path] || '工作台'
})

const pageDescription = computed(() => {
  const path = router.currentRoute.value.path
  const descriptions: Record<string, string> = {
    '/': '欢迎使用 Cornerstone 硬件工程数据平台',
    '/organizations': '管理您的团队和组织',
    '/databases': '管理您的数据库和数据表',
    '/plugins': '扩展平台功能的插件系统',
    '/settings': '平台配置和个人设置',
  }
  return descriptions[path] || '欢迎使用 Cornerstone 硬件工程数据平台'
})

// 统计数据
const stats = ref({
  users: 0,
  organizations: 0,
  databases: 0,
  plugins: 0,
})

// 活动记录
const activities = ref<Array<{ content: string; time: string; type: any }>>([])

// 模拟统计数据
const loadStats = async () => {
  // TODO: 从后端获取真实数据
  stats.value = {
    users: 15,
    organizations: 3,
    databases: 8,
    plugins: 5,
  }
}

// 模拟活动数据
const loadActivities = async () => {
  // TODO: 从后端获取真实数据
  activities.value = [
    {
      content: '创建了新的数据库 "项目A-电路设计"',
      time: '2小时前',
      type: 'primary',
    },
    {
      content: '邀请 user2 加入组织 "研发团队"',
      time: '5小时前',
      type: 'success',
    },
    {
      content: '安装了 "数据导出" 插件',
      time: '1天前',
      type: 'warning',
    },
    {
      content: '更新了个人资料',
      time: '2天前',
      type: 'info',
    },
  ]
}

// 处理用户菜单命令
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

onMounted(() => {
  if (!authStore.isAuthenticated) {
    router.push('/login')
    return
  }

  loadStats()
  loadActivities()
})
</script>

<style scoped lang="scss">
.dashboard {
  height: 100vh;
  display: flex;
  flex-direction: column;
}

.header {
  background: #409eff;
  color: white;
  display: flex;
  align-items: center;
  padding: 0 20px;
  box-shadow: 0 2px 4px rgba(0, 0, 0, 0.1);
}

.header-content {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
}

.logo h3 {
  margin: 0;
  font-size: 20px;
  font-weight: 600;
}

.nav-actions {
  display: flex;
  align-items: center;
  gap: 16px;
}

.user-info {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: white;
}

.username {
  font-weight: 500;
}

.main-container {
  flex: 1;
  overflow: hidden;
}

.sidebar {
  background: #545c64;
  overflow-y: auto;
}

.menu {
  border: none;
  height: 100%;
}

.content {
  background: #f5f5f5;
  overflow-y: auto;
  padding: 24px;
}

.content-header {
  margin-bottom: 24px;

  h2 {
    margin: 0 0 8px;
    font-size: 24px;
    font-weight: 600;
  }

  .description {
    margin: 0;
    color: #909399;
    font-size: 14px;
  }
}

.stats-row {
  margin-bottom: 24px;
}

.stat-card {
  cursor: pointer;
  transition: transform 0.2s;

  &:hover {
    transform: translateY(-2px);
  }

  .stat-content {
    display: flex;
    align-items: center;
    gap: 16px;

    .stat-icon {
      width: 48px;
      height: 48px;
      border-radius: 8px;
      display: flex;
      align-items: center;
      justify-content: center;
      color: white;
      font-size: 20px;
    }

    .stat-info {
      flex: 1;

      .stat-value {
        font-size: 24px;
        font-weight: 600;
        color: #303133;
      }

      .stat-label {
        font-size: 12px;
        color: #909399;
        margin-top: 4px;
      }
    }
  }
}

.quick-actions {
  margin-bottom: 24px;

  h3 {
    margin: 0 0 16px;
    font-size: 18px;
    font-weight: 600;
  }

  .action-card {
    cursor: pointer;
    transition: all 0.2s;

    &:hover {
      transform: translateY(-2px);
      box-shadow: 0 4px 12px rgba(0, 0, 0, 0.15);
    }

    .action-content {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      padding: 20px;
      font-weight: 500;
      color: #409eff;
    }
  }
}

.recent-activity {
  h3 {
    margin: 0 0 16px;
    font-size: 18px;
    font-weight: 600;
  }
}
</style>