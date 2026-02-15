<template>
  <div class="dashboard">
    <div class="content-header">
      <p class="description">欢迎使用 Cornerstone 数据平台</p>
    </div>

    <!-- 统计卡片 -->
    <el-row :gutter="20" class="stats-row">
      <el-col :xs="24" :sm="12" :md="6">
        <el-card class="stat-card" shadow="hover">
          <div class="stat-content">
            <div class="stat-icon stat-icon--sky">
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
            <div class="stat-icon stat-icon--teal">
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
            <div class="stat-icon stat-icon--amber">
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
            <div class="stat-icon stat-icon--rose">
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
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { statsAPI } from '@/services/api'
import { ElMessage } from 'element-plus'
import { formatTimeAgo } from '@/utils/format'
import { OfficeBuilding, DataLine, Connection, User, Plus } from '@element-plus/icons-vue'

const stats = ref({
  users: 0,
  organizations: 0,
  databases: 0,
  plugins: 0,
})

const activities = ref<
  Array<{
    content: string
    time: string
    type: 'primary' | 'success' | 'warning' | 'danger' | 'info'
  }>
>([])

const loadStats = async () => {
  try {
    const res = await statsAPI.getSummary()
    stats.value = res.data || {
      users: 0,
      organizations: 0,
      databases: 0,
      plugins: 0,
    }
  } catch (error) {
    console.error('Failed to load stats:', error)
    ElMessage.error('加载统计数据失败')
  }
}

const loadActivities = async () => {
  try {
    const res = await statsAPI.getActivities(10)
    activities.value = (res.data || []).map(
      (item: { type?: string; content: string; time: string }) => ({
        content: item.content,
        time: formatTimeAgo(item.time),
        type: (item.type || 'primary') as 'primary' | 'success' | 'warning' | 'danger' | 'info',
      }),
    )
  } catch (error) {
    console.error('Failed to load activities:', error)
    activities.value = []
  }
}

onMounted(() => {
  loadStats()
  loadActivities()
})
</script>

<style scoped lang="scss">
.dashboard {
  max-width: 1200px;
}

.content-header {
  margin-bottom: 24px;

  .description {
    margin: 0;
    color: var(--fa-text-muted);
    font-size: 14px;
  }
}

.stats-row {
  margin-bottom: 24px;
}

.stat-card {
  cursor: pointer;
  transition:
    transform 0.25s,
    box-shadow 0.25s;

  &:hover {
    transform: translateY(-3px);
    box-shadow: var(--fa-shadow-lg), var(--fa-shadow-glow);
  }

  .stat-content {
    display: flex;
    align-items: center;
    gap: 16px;

    .stat-icon {
      width: 48px;
      height: 48px;
      border-radius: var(--fa-radius-md);
      display: flex;
      align-items: center;
      justify-content: center;
      color: white;
      font-size: 20px;
      flex-shrink: 0;
    }

    .stat-icon--sky {
      background: linear-gradient(135deg, var(--fa-sky-400), var(--fa-sky-500));
    }

    .stat-icon--teal {
      background: linear-gradient(135deg, var(--fa-teal-400), var(--fa-teal-500));
    }

    .stat-icon--amber {
      background: linear-gradient(135deg, #fbbf24, #f59e0b);
    }

    .stat-icon--rose {
      background: linear-gradient(135deg, #fb7185, #f43f5e);
    }

    .stat-info {
      flex: 1;

      .stat-value {
        font-size: 24px;
        font-weight: 600;
        color: var(--fa-text-primary);
      }

      .stat-label {
        font-size: 12px;
        color: var(--fa-text-muted);
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
    color: var(--fa-text-primary);
  }

  .action-card {
    cursor: pointer;
    transition:
      transform 0.25s,
      box-shadow 0.25s;

    &:hover {
      transform: translateY(-3px);
      box-shadow: var(--fa-shadow-lg), var(--fa-shadow-glow);
    }

    .action-content {
      display: flex;
      align-items: center;
      justify-content: center;
      gap: 8px;
      padding: 20px;
      font-weight: 500;
      color: var(--fa-accent-solid);
    }
  }
}

.recent-activity {
  h3 {
    margin: 0 0 16px;
    font-size: 18px;
    font-weight: 600;
    color: var(--fa-text-primary);
  }
}
</style>
