<template>
  <div class="plugins">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>插件管理</span>
          <el-button type="primary" @click="handleInstall">安装插件</el-button>
        </div>
      </template>

      <el-tabs v-model="activeTab" @tab-change="handleTabChange">
        <el-tab-pane label="已安装" name="installed">
          <el-empty v-if="installedPlugins.length === 0" description="暂无已安装的插件">
            <el-button type="primary" @click="handleInstall">安装插件</el-button>
          </el-empty>

          <el-table v-else :data="installedPlugins" style="width: 100%" v-loading="loading">
            <el-table-column prop="name" label="插件名称" min-width="150" />
            <el-table-column prop="language" label="语言" width="100">
              <template #default="{ row }">
                <el-tag :type="getLanguageType(row.language)">{{ row.language }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="version" label="版本" width="100" />
            <el-table-column prop="status" label="状态" width="100">
              <template #default="{ row }">
                <el-tag :type="getStatusType(row.status)">{{ row.status }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column label="操作" width="200" fixed="right">
              <template #default="{ row }">
                <el-button size="small" @click="handleConfig(row)">配置</el-button>
                <el-button size="small" type="warning" @click="handleDisable(row)">
                  {{ row.status === 'enabled' ? '禁用' : '启用' }}
                </el-button>
                <el-button size="small" type="danger" @click="handleUninstall(row)">卸载</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-tab-pane>

        <el-tab-pane label="插件市场" name="market">
          <el-row :gutter="20" v-loading="marketLoading">
            <el-col :xs="24" :sm="12" :md="8" v-for="plugin in marketPlugins" :key="plugin.id">
              <el-card class="plugin-card" shadow="hover">
                <div class="plugin-header">
                  <h4>{{ plugin.name }}</h4>
                  <el-tag size="small" :type="getLanguageType(plugin.language)">{{ plugin.language }}</el-tag>
                </div>
                <p class="plugin-desc">{{ plugin.description }}</p>
                <div class="plugin-meta">
                  <span>版本: {{ plugin.version }}</span>
                  <span>下载: {{ plugin.downloads }}</span>
                </div>
                <el-button
                  type="primary"
                  size="small"
                  class="install-btn"
                  @click="handleInstallPlugin(plugin)"
                >
                  安装
                </el-button>
              </el-card>
            </el-col>
          </el-row>
        </el-tab-pane>
      </el-tabs>
    </el-card>

    <!-- 安装对话框 -->
    <el-dialog
      v-model="installDialogVisible"
      title="安装插件"
      width="500px"
    >
      <el-form :model="installForm" label-width="100px">
        <el-form-item label="插件来源">
          <el-radio-group v-model="installForm.source">
            <el-radio label="market">插件市场</el-radio>
            <el-radio label="url">URL安装</el-radio>
            <el-radio label="upload">上传文件</el-radio>
          </el-radio-group>
        </el-form-item>

        <el-form-item v-if="installForm.source === 'url'" label="插件URL">
          <el-input v-model="installForm.url" placeholder="输入插件Git仓库URL" />
        </el-form-item>

        <el-form-item v-if="installForm.source === 'upload'" label="上传文件">
          <el-upload
            class="upload-demo"
            action="#"
            :auto-upload="false"
            :on-change="handleFileChange"
            :file-list="installForm.fileList"
          >
            <el-button type="primary">选择文件</el-button>
            <template #tip>
              <div class="el-upload__tip">支持 .zip, .tar.gz 格式</div>
            </template>
          </el-upload>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="installDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleConfirmInstall" :loading="installing">
            确认安装
          </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 配置对话框 -->
    <el-dialog
      v-model="configDialogVisible"
      title="插件配置"
      width="600px"
    >
      <el-form :model="configForm" label-width="120px">
        <el-form-item label="插件名称">
          <el-input v-model="configForm.name" disabled />
        </el-form-item>
        <el-form-item label="超时时间(秒)">
          <el-input-number v-model="configForm.timeout" :min="1" :max="300" />
        </el-form-item>
        <el-form-item label="触发器">
          <el-select v-model="configForm.trigger" placeholder="选择触发器">
            <el-option label="数据导入后" value="after_import" />
            <el-option label="数据导出前" value="before_export" />
            <el-option label="手动触发" value="manual" />
          </el-select>
        </el-form-item>
        <el-form-item label="配置参数">
          <el-input
            v-model="configForm.params"
            type="textarea"
            :rows="4"
            placeholder='{"key": "value"}'
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="configDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSaveConfig" :loading="savingConfig">
            保存配置
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import type { UploadFile } from 'element-plus'

interface Plugin {
  id: string
  name: string
  language: string
  version: string
  status?: string
  description?: string
  downloads?: number
}

const activeTab = ref('installed')
const loading = ref(false)
const marketLoading = ref(false)
const installing = ref(false)
const savingConfig = ref(false)

const installedPlugins = ref<Plugin[]>([])
const marketPlugins = ref<Plugin[]>([])

const installDialogVisible = ref(false)
const configDialogVisible = ref(false)

const installForm = ref({
  source: 'market',
  url: '',
  fileList: [] as UploadFile[],
})

const configForm = ref({
  name: '',
  timeout: 30,
  trigger: 'manual',
  params: '{}',
})

// 获取语言标签类型
const getLanguageType = (language: string) => {
  const typeMap: Record<string, string> = {
    Python: 'success',
    Go: 'primary',
    JavaScript: 'warning',
    TypeScript: 'info',
  }
  return typeMap[language] || 'info'
}

// 获取状态标签类型
const getStatusType = (status: string) => {
  const typeMap: Record<string, string> = {
    enabled: 'success',
    disabled: 'warning',
    error: 'danger',
  }
  return typeMap[status] || 'info'
}

// 加载已安装插件
const loadInstalledPlugins = async () => {
  loading.value = true
  try {
    // 模拟数据
    installedPlugins.value = [
      {
        id: '1',
        name: '数据导出插件',
        language: 'Python',
        version: '1.2.0',
        status: 'enabled',
      },
      {
        id: '2',
        name: 'Excel导入工具',
        language: 'Go',
        version: '2.0.1',
        status: 'enabled',
      },
      {
        id: '3',
        name: 'JSON验证器',
        language: 'JavaScript',
        version: '0.9.5',
        status: 'disabled',
      },
    ]
  } catch (error) {
    ElMessage.error('加载插件列表失败')
  } finally {
    loading.value = false
  }
}

// 加载插件市场
const loadMarketPlugins = async () => {
  marketLoading.value = true
  try {
    // 模拟数据
    marketPlugins.value = [
      {
        id: 'm1',
        name: 'CSV解析器',
        language: 'Python',
        version: '1.0.0',
        description: '强大的CSV文件解析和处理工具',
        downloads: 1234,
      },
      {
        id: 'm2',
        name: '数据同步器',
        language: 'Go',
        version: '1.5.2',
        description: '跨数据库数据同步工具',
        downloads: 856,
      },
      {
        id: 'm3',
        name: '图表生成器',
        language: 'TypeScript',
        version: '0.8.0',
        description: '生成数据可视化图表',
        downloads: 2341,
      },
      {
        id: 'm4',
        name: 'API测试工具',
        language: 'JavaScript',
        version: '2.1.0',
        description: '自动化API测试和监控',
        downloads: 567,
      },
    ]
  } catch (error) {
    ElMessage.error('加载插件市场失败')
  } finally {
    marketLoading.value = false
  }
}

const handleTabChange = (tab: string) => {
  if (tab === 'market') {
    loadMarketPlugins()
  }
}

const handleInstall = () => {
  installForm.value = { source: 'market', url: '', fileList: [] }
  installDialogVisible.value = true
}

const handleInstallPlugin = (plugin: Plugin) => {
  ElMessageBox.confirm(
    `确定要安装插件 "${plugin.name}" 吗？`,
    '安装插件',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'info',
    }
  ).then(() => {
    ElMessage.success(`插件 "${plugin.name}" 安装成功`)
    loadInstalledPlugins()
  }).catch(() => {})
}

const handleFileChange = (file: UploadFile) => {
  installForm.value.fileList = [file]
}

const handleConfirmInstall = () => {
  installing.value = true
  setTimeout(() => {
    installing.value = false
    installDialogVisible.value = false
    ElMessage.success('插件安装成功')
    loadInstalledPlugins()
  }, 1500)
}

const handleConfig = (row: Plugin) => {
  configForm.value = {
    name: row.name,
    timeout: 30,
    trigger: 'manual',
    params: '{}',
  }
  configDialogVisible.value = true
}

const handleSaveConfig = () => {
  savingConfig.value = true
  setTimeout(() => {
    savingConfig.value = false
    configDialogVisible.value = false
    ElMessage.success('配置保存成功')
  }, 1000)
}

const handleDisable = (row: Plugin) => {
  const action = row.status === 'enabled' ? '禁用' : '启用'
  ElMessageBox.confirm(
    `确定要${action}插件 "${row.name}" 吗？`,
    '操作确认',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(() => {
    row.status = row.status === 'enabled' ? 'disabled' : 'enabled'
    ElMessage.success(`${action}成功`)
  }).catch(() => {})
}

const handleUninstall = (row: Plugin) => {
  ElMessageBox.confirm(
    `确定要卸载插件 "${row.name}" 吗？`,
    '警告',
    {
      confirmButtonText: '确定',
      cancelButtonText: '取消',
      type: 'warning',
    }
  ).then(() => {
    installedPlugins.value = installedPlugins.value.filter(p => p.id !== row.id)
    ElMessage.success('卸载成功')
  }).catch(() => {})
}

onMounted(() => {
  loadInstalledPlugins()
})
</script>

<style scoped lang="scss">
.plugins {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .plugin-card {
    margin-bottom: 16px;
    position: relative;

    .plugin-header {
      display: flex;
      justify-content: space-between;
      align-items: center;
      margin-bottom: 8px;

      h4 {
        margin: 0;
        font-size: 16px;
        font-weight: 600;
      }
    }

    .plugin-desc {
      color: #606266;
      font-size: 13px;
      margin: 8px 0;
      line-height: 1.4;
      height: 38px;
      overflow: hidden;
    }

    .plugin-meta {
      display: flex;
      justify-content: space-between;
      font-size: 12px;
      color: #909399;
      margin-bottom: 12px;
    }

    .install-btn {
      width: 100%;
    }
  }

  .upload-demo {
    width: 100%;
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }
}
</style>