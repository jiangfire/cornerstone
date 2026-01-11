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
        <el-table-column label="操作" width="250" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleEdit(row)">编辑</el-button>
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
const isEdit = ref(false)
const currentPlugin = ref<Plugin | null>(null)

const form = ref({
  name: '',
  description: '',
  language: 'go',
  entry_file: '',
  timeout: 30
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
    timeout: 30
  }
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
    timeout: plugin.timeout
  }
  dialogVisible.value = true
}

// 提交表单
const handleSubmit = async () => {
  submitting.value = true
  try {
    if (isEdit.value && currentPlugin.value) {
      await pluginAPI.update(currentPlugin.value.id, {
        name: form.value.name,
        description: form.value.description,
        timeout: form.value.timeout
      })
      ElMessage.success('更新成功')
    } else {
      await pluginAPI.create(form.value)
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
