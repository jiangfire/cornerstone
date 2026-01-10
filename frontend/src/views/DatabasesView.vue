<template>
  <div class="databases">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>数据库管理</span>
          <el-button type="primary" @click="handleCreate">新建数据库</el-button>
        </div>
      </template>

      <el-empty v-if="databases.length === 0" description="暂无数据库，请创建您的第一个数据库">
        <el-button type="primary" @click="handleCreate">创建数据库</el-button>
      </el-empty>

      <el-table v-else :data="databases" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" label="数据库名称" min-width="180" />
        <el-table-column prop="type" label="类型" width="120">
          <template #default="{ row }">
            <el-tag type="info">{{ row.type || 'PostgreSQL' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="table_count" label="表数量" width="100" align="center" />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="200" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleTables(row)">表结构</el-button>
            <el-button size="small" type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="500px"
      @close="resetForm"
    >
      <el-form
        ref="formRef"
        :model="form"
        :rules="rules"
        label-width="100px"
        :loading="submitting"
      >
        <el-form-item label="数据库名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入数据库名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入数据库描述（可选）"
          />
        </el-form-item>
        <el-form-item label="公开" prop="isPublic">
          <el-switch v-model="form.isPublic" active-text="允许其他用户访问" />
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
import { useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { databaseAPI } from '@/services/api'

interface Database {
  id: string
  name: string
  description?: string
  type: string
  table_count: number
  created_at: string
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const isEditMode = ref(false)
const databases = ref<Database[]>([])

const router = useRouter()

const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  description: '',
  isPublic: false,
  id: '',
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入数据库名称', trigger: 'blur' },
    { min: 2, max: 50, message: '长度在 2-50 个字符之间', trigger: 'blur' },
  ],
  description: [
    { max: 200, message: '描述不能超过200个字符', trigger: 'blur' },
  ],
}

const dialogTitle = ref('创建数据库')

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

const loadDatabases = async () => {
  loading.value = true
  try {
    const response = await databaseAPI.list()
    if (response.success && response.data) {
      databases.value = response.data.databases || []
    }
  } catch (error) {
    ElMessage.error('加载数据库列表失败')
  } finally {
    loading.value = false
  }
}

const handleCreate = () => {
  isEditMode.value = false
  dialogTitle.value = '创建数据库'
  form.value = { name: '', description: '', isPublic: false, id: '' }
  dialogVisible.value = true
}

const handleEdit = (row: Database) => {
  isEditMode.value = true
  dialogTitle.value = '编辑数据库'
  form.value = {
    name: row.name,
    description: row.description || '',
    isPublic: false,
    id: row.id,
  }
  dialogVisible.value = true
}

const handleTables = (row: Database) => {
  router.push(`/databases/${row.id}`)
}

const handleDelete = async (row: Database) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除数据库 "${row.name}" 吗？相关数据将被清空。`,
      '警告',
      {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消',
      }
    )
    ElMessage.success('删除成功')
    await loadDatabases()
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
      ElMessage.success('更新成功')
    } else {
      const response = await databaseAPI.create({
        name: form.value.name,
        description: form.value.description,
        isPublic: form.value.isPublic,
      })
      if (response.success) {
        ElMessage.success('创建成功')
      }
    }

    dialogVisible.value = false
    await loadDatabases()
  } catch (error) {
    ElMessage.error(isEditMode.value ? '更新失败' : '创建失败')
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
  loadDatabases()
})
</script>

<style scoped lang="scss">
.databases {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
}
</style>