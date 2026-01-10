<template>
  <div class="fields">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button link @click="goBack">
              <el-icon><ArrowLeft /></el-icon>
              返回表列表
            </el-button>
            <span class="table-name">{{ tableName }} - 字段管理</span>
          </div>
          <el-button type="primary" @click="handleCreate">添加字段</el-button>
        </div>
      </template>

      <el-empty v-if="fields.length === 0" description="暂无字段，请添加您的第一个字段">
        <el-button type="primary" @click="handleCreate">添加字段</el-button>
      </el-empty>

      <el-table v-else :data="fields" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" label="字段名称" min-width="180" />
        <el-table-column prop="type" label="字段类型" width="120">
          <template #default="{ row }">
            <el-tag :type="getFieldTypeColor(row.type)">{{ getFieldTypeLabel(row.type) }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="required" label="必填" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.required ? 'danger' : 'info'">
              {{ row.required ? '是' : '否' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="options" label="配置选项" min-width="200" show-overflow-tooltip />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="600px"
      @close="resetForm"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="120px"
        :loading="submitting"
      >
        <el-form-item label="字段名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入字段名称" />
        </el-form-item>
        <el-form-item label="字段类型" prop="type">
          <el-select v-model="form.type" placeholder="请选择字段类型" style="width: 100%">
            <el-option label="字符串" value="string" />
            <el-option label="数字" value="number" />
            <el-option label="布尔值" value="boolean" />
            <el-option label="日期" value="date" />
            <el-option label="日期时间" value="datetime" />
            <el-option label="文本" value="text" />
            <el-option label="下拉选择" value="select" />
            <el-option label="多选" value="multiselect" />
          </el-select>
        </el-form-item>
        <el-form-item label="必填" prop="required">
          <el-switch v-model="form.required" />
        </el-form-item>
        <el-form-item
          v-if="form.type === 'select' || form.type === 'multiselect'"
          label="选项配置"
          prop="options"
        >
          <el-input
            v-model="form.options"
            type="textarea"
            :rows="3"
            placeholder="请输入选项，用逗号分隔（例如：选项1,选项2,选项3）"
          />
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting">
            确定
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tableAPI, fieldAPI } from '@/services/api'

interface Field {
  id: string
  name: string
  type: string
  required: boolean
  options?: string
  created_at: string
}

const route = useRoute()
const router = useRouter()
const tableId = route.params.id as string
const tableName = ref('')

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const isEditMode = ref(false)
const fields = ref<Field[]>([])

const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  type: 'string',
  required: false,
  options: '',
  id: '',
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入字段名称', trigger: 'blur' },
    { min: 2, max: 100, message: '长度在 2-100 个字符之间', trigger: 'blur' },
  ],
  type: [
    { required: true, message: '请选择字段类型', trigger: 'change' },
  ],
  options: [
    {
      validator: (rule: any, value: string, callback: any) => {
        if ((form.value.type === 'select' || form.value.type === 'multiselect') && !value) {
          callback(new Error('下拉/多选字段必须配置选项'))
        } else {
          callback()
        }
      },
      trigger: 'blur',
    },
  ],
}

const dialogTitle = ref('添加字段')

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

const goBack = () => {
  router.push(`/databases`)
}

const getFieldTypeLabel = (type: string) => {
  const typeMap: Record<string, string> = {
    string: '字符串',
    number: '数字',
    boolean: '布尔值',
    date: '日期',
    datetime: '日期时间',
    text: '文本',
    select: '下拉选择',
    multiselect: '多选',
  }
  return typeMap[type] || type
}

const getFieldTypeColor = (type: string) => {
  const colorMap: Record<string, string> = {
    string: '',
    number: 'success',
    boolean: 'warning',
    date: 'info',
    datetime: 'info',
    text: '',
    select: 'success',
    multiselect: 'success',
  }
  return colorMap[type] || ''
}

const loadFields = async () => {
  loading.value = true
  try {
    const response = await tableAPI.getFields(tableId)
    if (response.success && response.data) {
      fields.value = response.data.fields || []
    }
  } catch (error) {
    ElMessage.error('加载字段列表失败')
  } finally {
    loading.value = false
  }
}

const loadTableInfo = async () => {
  try {
    const response = await tableAPI.get(tableId)
    if (response.success && response.data) {
      tableName.value = response.data.table.name
    }
  } catch (error) {
    console.error('Failed to load table info:', error)
  }
}

const handleCreate = () => {
  isEditMode.value = false
  dialogTitle.value = '添加字段'
  form.value = { name: '', type: 'string', required: false, options: '', id: '' }
  dialogVisible.value = true
}

const handleEdit = (row: Field) => {
  isEditMode.value = true
  dialogTitle.value = '编辑字段'
  form.value = {
    name: row.name,
    type: row.type,
    required: row.required,
    options: row.options || '',
    id: row.id,
  }
  dialogVisible.value = true
}

const handleDelete = async (row: Field) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除字段 "${row.name}" 吗？`,
      '警告',
      {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消',
      }
    )
    const response = await fieldAPI.delete(row.id)
    if (response.success) {
      ElMessage.success('删除成功')
      await loadFields()
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

const handleSubmit = async () => {
  if (!formRef.value) return

  try {
    const valid = await formRef.value.validate()
    if (!valid) return

    submitting.value = true

    if (isEditMode.value) {
      const response = await fieldAPI.update(form.value.id, {
        name: form.value.name,
        type: form.value.type,
        required: form.value.required,
        options: form.value.options,
      })
      if (response.success) {
        ElMessage.success('更新成功')
        dialogVisible.value = false
        await loadFields()
      }
    } else {
      const response = await fieldAPI.create({
        table_id: tableId,
        name: form.value.name,
        type: form.value.type,
        required: form.value.required,
        options: form.value.options,
      })
      if (response.success) {
        ElMessage.success('添加成功')
        dialogVisible.value = false
        await loadFields()
      }
    }
  } catch (error) {
    ElMessage.error(isEditMode.value ? '更新失败' : '添加失败')
  } finally {
    submitting.value = false
  }
}

const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

onMounted(() => {
  loadTableInfo()
  loadFields()
})
</script>

<style scoped lang="scss">
.fields {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;

    .header-left {
      display: flex;
      align-items: center;
      gap: 12px;

      .table-name {
        font-weight: 600;
        font-size: 16px;
      }
    }
  }
}
</style>
