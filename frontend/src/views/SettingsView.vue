<template>
  <div class="settings">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>系统设置</span>
        </div>
      </template>

      <el-tabs v-model="activeTab">
        <el-tab-pane label="系统配置" name="system">
          <el-form :model="systemForm" label-width="150px" style="max-width: 600px">
            <el-form-item label="系统名称">
              <el-input v-model="systemForm.name" placeholder="Cornerstone" />
            </el-form-item>
            <el-form-item label="系统描述">
              <el-input
                v-model="systemForm.description"
                type="textarea"
                :rows="3"
                placeholder="数据管理平台"
              />
            </el-form-item>
            <el-form-item label="允许用户注册">
              <el-switch v-model="systemForm.allowRegistration" />
            </el-form-item>
            <el-form-item label="最大文件大小(MB)">
              <el-input-number v-model="systemForm.maxFileSize" :min="1" :max="1024" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveSystemConfig">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="数据库配置" name="database">
          <el-form :model="dbForm" label-width="150px" style="max-width: 600px">
            <el-form-item label="数据库类型">
              <el-select v-model="dbForm.type" placeholder="请选择">
                <el-option label="PostgreSQL" value="postgresql" />
                <el-option label="MySQL" value="mysql" />
                <el-option label="SQLite" value="sqlite" />
              </el-select>
            </el-form-item>
            <el-form-item label="连接池大小">
              <el-input-number v-model="dbForm.poolSize" :min="1" :max="100" />
            </el-form-item>
            <el-form-item label="超时时间(秒)">
              <el-input-number v-model="dbForm.timeout" :min="1" :max="300" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="saveDbConfig">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <el-tab-pane label="插件配置" name="plugins">
          <el-form :model="pluginForm" label-width="150px" style="max-width: 600px">
            <el-form-item label="插件超时(秒)">
              <el-input-number v-model="pluginForm.timeout" :min="1" :max="600" />
            </el-form-item>
            <el-form-item label="工作目录">
              <el-input v-model="pluginForm.workDir" placeholder="/var/plugins" />
            </el-form-item>
            <el-form-item label="自动更新">
              <el-switch v-model="pluginForm.autoUpdate" />
            </el-form-item>
            <el-form-item>
              <el-button type="primary" @click="savePluginConfig">保存配置</el-button>
            </el-form-item>
          </el-form>
        </el-tab-pane>
      </el-tabs>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { ElMessage } from 'element-plus'

const activeTab = ref('system')

// 系统配置
const systemForm = ref({
  name: 'Cornerstone',
  description: '数据管理平台',
  allowRegistration: true,
  maxFileSize: 50,
})

// 数据库配置
const dbForm = ref({
  type: 'postgresql',
  poolSize: 10,
  timeout: 30,
})

// 插件配置
const pluginForm = ref({
  timeout: 300,
  workDir: '/var/plugins',
  autoUpdate: false,
})

const saveSystemConfig = () => {
  ElMessage.success('系统配置已保存')
}

const saveDbConfig = () => {
  ElMessage.success('数据库配置已保存')
}

const savePluginConfig = () => {
  ElMessage.success('插件配置已保存')
}
</script>

<style scoped lang="scss">
.settings {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
}
</style>
