<template>
  <div class="plugins">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>插件管理</span>
          <el-button type="primary" @click="showCreateDialog">创建插件</el-button>
        </div>
      </template>

      <el-table :data="plugins" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" label="插件名称" min-width="150" />
        <el-table-column prop="language" label="语言" width="100">
          <template #default="{ row }">
            <el-tag :type="getLanguageType(row.language)">{{ row.language }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="description" label="描述" min-width="200" />
        <el-table-column prop="timeout" label="超时(秒)" width="100" />
        <el-table-column label="操作" width="320" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button size="small" type="info" @click="handleViewBindings(row)">查看绑定</el-button>
            <el-button size="small" type="primary" @click="handleBind(row)">绑定</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑插件对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="isEdit ? '编辑插件' : '创建插件'"
      width="500px"
    >
      <el-form :model="form" label-width="100px">
        <el-form-item label="插件名称">
          <el-input v-model="form.name" placeholder="请输入插件名称" />
        </el-form-item>
        <el-form-item label="描述">
          <el-input v-model="form.description" type="textarea" rows="3" />
        </el-form-item>
        <el-form-item label="语言" v-if="!isEdit">
          <el-select v-model="form.language" placeholder="请选择语言">
            <el-option label="Go" value="go" />
            <el-option label="Python" value="python" />
            <el-option label="Bash" value="bash" />
          </el-select>
        </el-form-item>
        <el-form-item label="入口文件" v-if="!isEdit">
          <el-input v-model="form.entry_file" placeholder="main.go / main.py / main.sh" />
        </el-form-item>
        <el-form-item label="超时(秒)">
          <el-input-number v-model="form.timeout" :min="1" :max="300" />
        </el-form-item>
        <el-form-item label="配置参数" v-if="!isEdit">
          <el-button size="small" @click="showConfigEditor">配置参数</el-button>
          <span v-if="form.config && form.config.length > 0" style="margin-left: 10px">
            已配置 {{ parseConfig(form.config).length }} 个参数
          </span>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleSubmit" :loading="submitting">确定</el-button>
      </template>
    </el-dialog>

    <!-- 绑定插件对话框 -->
    <el-dialog v-model="bindDialogVisible" title="绑定插件到表" width="500px">
      <el-form :model="bindForm" label-width="100px">
        <el-form-item label="选择表">
          <el-select v-model="bindForm.table_id" placeholder="请选择表">
            <el-option
              v-for="table in tables"
              :key="table.id"
              :label="table.name"
              :value="table.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="触发器">
          <el-select v-model="bindForm.trigger" placeholder="请选择触发器">
            <el-option label="创建记录时" value="create" />
            <el-option label="更新记录时" value="update" />
            <el-option label="删除记录时" value="delete" />
            <el-option label="手动触发" value="manual" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="bindDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="handleConfirmBind" :loading="binding">确定</el-button>
      </template>
    </el-dialog>

    <!-- 查看绑定对话框 -->
    <el-dialog v-model="bindingsDialogVisible" title="插件绑定管理" width="700px">
      <el-table :data="bindingsList" style="width: 100%" v-loading="loadingBindings">
        <el-table-column prop="database_name" label="数据库" />
        <el-table-column prop="table_name" label="表名" />
        <el-table-column prop="trigger" label="触发器" width="120">
          <template #default="{ row }">
            <el-tag :type="getTriggerType(row.trigger)">{{ getTriggerLabel(row.trigger) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="绑定时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-button size="small" type="danger" @click="handleUnbind(row)">解绑</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="bindingsList.length === 0" description="暂无绑定" />
    </el-dialog>

    <!-- 配置编辑器对话框 -->
    <el-dialog v-model="configDialogVisible" title="配置插件参数" width="700px">
      <el-button @click="addConfigItem" style="margin-bottom: 15px">+ 添加参数</el-button>

      <el-table :data="configItems" style="width: 100%">
        <el-table-column prop="name" label="参数名" width="150">
          <template #default="{ row }">
            <el-input v-model="row.name" placeholder="参数名" size="small" />
          </template>
        </el-table-column>
        <el-table-column prop="type" label="类型" width="120">
          <template #default="{ row }">
            <el-select v-model="row.type" size="small">
              <el-option label="字符串" value="string" />
              <el-option label="数字" value="number" />
              <el-option label="布尔" value="boolean" />
              <el-option label="下拉选择" value="select" />
            </el-select>
          </template>
        </el-table-column>
        <el-table-column prop="default" label="默认值" width="120">
          <template #default="{ row }">
            <el-input v-model="row.default" placeholder="默认值" size="small" />
          </template>
        </el-table-column>
        <el-table-column prop="required" label="必填" width="80">
          <template #default="{ row }">
            <el-switch v-model="row.required" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="{ $index }">
            <el-button size="small" type="danger" @click="removeConfigItem($index)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>

      <template #footer>
        <el-button @click="configDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="configDialogVisible = false">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { pluginAPI, tableAPI } from '@/services/api'

interface Plugin {
  id: string
  name: string
  description: string
  language: string
  entry_file: string
  timeout: number
  config?: string
  config_values?: string
  created_by: string
  created_at: string
}

interface Table {
  id: string
  name: string
}

const loading = ref(false)
const submitting = ref(false)
const binding = ref(false)
const plugins = ref<Plugin[]>([])
const tables = ref<Table[]>([])

const dialogVisible = ref(false)
const bindDialogVisible = ref(false)
const bindingsDialogVisible = ref(false)
const configDialogVisible = ref(false)
const isEdit = ref(false)
const currentPlugin = ref<Plugin | null>(null)
const bindingsList = ref<any[]>([])
const loadingBindings = ref(false)
const configItems = ref<any[]>([])

const form = ref({
  name: '',
  description: '',
  language: 'go',
  entry_file: '',
  timeout: 30,
  config: '',
  config_values: ''
})

const bindForm = ref({
  table_id: '',
  trigger: 'manual'
})

// 加载插件列表
const loadPlugins = async () => {
  loading.value = true
  try {
    const res: any = await pluginAPI.list()
    plugins.value = res.data || []
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '加载插件列表失败')
  } finally {
    loading.value = false
  }
}

// 显示创建对话框
const showCreateDialog = () => {
  isEdit.value = false
  form.value = {
    name: '',
    description: '',
    language: 'go',
    entry_file: '',
    timeout: 30,
    config: '',
    config_values: ''
  }
  configItems.value = []
  dialogVisible.value = true
}

// 编辑插件
const handleEdit = (plugin: Plugin) => {
  isEdit.value = true
  currentPlugin.value = plugin
  form.value = {
    name: plugin.name,
    description: plugin.description,
    language: plugin.language,
    entry_file: plugin.entry_file,
    timeout: plugin.timeout,
    config: plugin.config || '',
    config_values: plugin.config_values || ''
  }
  if (plugin.config) {
    configItems.value = parseConfig(plugin.config)
  } else {
    configItems.value = []
  }
  dialogVisible.value = true
}

// 提交表单
const handleSubmit = async () => {
  submitting.value = true
  try {
    // Save config as JSON string
    form.value.config = configItems.value.length > 0 ? JSON.stringify(configItems.value) : ''

    if (isEdit.value && currentPlugin.value) {
      await pluginAPI.update(currentPlugin.value.id, {
        name: form.value.name,
        description: form.value.description,
        timeout: form.value.timeout,
        config: form.value.config,
        config_values: form.value.config_values
      })
      ElMessage.success('更新成功')
    } else {
      await pluginAPI.create({
        name: form.value.name,
        description: form.value.description,
        language: form.value.language,
        entry_file: form.value.entry_file,
        timeout: form.value.timeout,
        config: form.value.config,
        config_values: form.value.config_values
      })
      ElMessage.success('创建成功')
    }
    dialogVisible.value = false
    loadPlugins()
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '操作失败')
  } finally {
    submitting.value = false
  }
}

// 删除插件
const handleDelete = async (plugin: Plugin) => {
  try {
    await ElMessageBox.confirm('确定要删除该插件吗？', '提示', {
      type: 'warning'
    })
    await pluginAPI.delete(plugin.id)
    ElMessage.success('删除成功')
    loadPlugins()
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '删除失败')
    }
  }
}

// 显示绑定对话框
const handleBind = async (plugin: Plugin) => {
  currentPlugin.value = plugin
  bindForm.value = {
    table_id: '',
    trigger: 'manual'
  }
  bindDialogVisible.value = true
}

// 确认绑定
const handleConfirmBind = async () => {
  if (!currentPlugin.value) return
  binding.value = true
  try {
    await pluginAPI.bind(currentPlugin.value.id, bindForm.value)
    ElMessage.success('绑定成功')
    bindDialogVisible.value = false
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '绑定失败')
  } finally {
    binding.value = false
  }
}

// 获取语言标签类型
const getLanguageType = (language: string) => {
  const types: Record<string, string> = {
    go: 'success',
    python: 'warning',
    bash: 'info'
  }
  return types[language] || ''
}

// 显示绑定列表
const handleViewBindings = async (plugin: Plugin) => {
  currentPlugin.value = plugin
  bindingsDialogVisible.value = true
  await loadBindings(plugin.id)
}

// 加载绑定列表
const loadBindings = async (pluginId: string) => {
  loadingBindings.value = true
  try {
    const res: any = await pluginAPI.getBindings(pluginId)
    bindingsList.value = res.data || []
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '加载绑定列表失败')
  } finally {
    loadingBindings.value = false
  }
}

// 解绑插件
const handleUnbind = async (binding: any) => {
  if (!currentPlugin.value) return
  try {
    await ElMessageBox.confirm(`确定要解绑表"${binding.table_name}"吗？`, '提示', {
      type: 'warning'
    })
    await pluginAPI.unbind(currentPlugin.value.id, { table_id: binding.table_id })
    ElMessage.success('解绑成功')
    await loadBindings(currentPlugin.value.id)
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error(error.response?.data?.message || '解绑失败')
    }
  }
}

// 获取触发器标签
const getTriggerLabel = (trigger: string) => {
  const labels: Record<string, string> = {
    create: '创建时',
    update: '更新时',
    delete: '删除时',
    manual: '手动触发',
  }
  return labels[trigger] || trigger
}

// 获取触发器类型
const getTriggerType = (trigger: string) => {
  const types: Record<string, string> = {
    create: 'success',
    update: 'warning',
    delete: 'danger',
    manual: 'info',
  }
  return types[trigger] || ''
}

// 格式化日期
const formatDate = (dateStr: string) => {
  return new Date(dateStr).toLocaleString('zh-CN')
}

// 解析配置 JSON
const parseConfig = (configStr: string) => {
  try {
    return JSON.parse(configStr || '[]')
  } catch {
    return []
  }
}

// 显示配置编辑器
const showConfigEditor = () => {
  configDialogVisible.value = true
}

// 添加配置项
const addConfigItem = () => {
  configItems.value.push({
    name: '',
    type: 'string',
    default: '',
    required: false
  })
}

// 删除配置项
const removeConfigItem = (index: number) => {
  configItems.value.splice(index, 1)
}

// 组件挂载时加载数据
onMounted(() => {
  loadPlugins()
})
</script>

<style scoped>
.card-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
</style>
