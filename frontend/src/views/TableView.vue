<template>
  <div class="tables">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button link @click="goBack">
              <el-icon><ArrowLeft /></el-icon>
              返回数据库列表
            </el-button>
            <span class="database-name">{{ databaseName }}</span>
          </div>
          <el-button v-if="canCreate" type="primary" @click="handleCreate">新建表</el-button>
        </div>
      </template>

      <el-empty v-if="tables.length === 0" description="暂无表，请创建您的第一个表">
        <el-button v-if="canCreate" type="primary" @click="handleCreate">创建表</el-button>
      </el-empty>

      <el-table v-else :data="tables" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" label="表名称" min-width="200" />
        <el-table-column prop="description" label="描述" min-width="250" show-overflow-tooltip />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleFields(row)">字段管理</el-button>
            <el-button size="small" @click="handleRecords(row)">数据记录</el-button>
            <el-button v-if="canDelete" size="small" type="danger" @click="handleDelete(row)">删除</el-button>
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
        label-width="100px"
        :loading="submitting"
      >
        <el-form-item label="表名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入表名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="4"
            placeholder="请输入表描述（可选）"
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
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tableAPI, databaseAPI } from '@/services/api'

interface Table {
  id: string
  name: string
  description?: string
  created_at: string
}

const route = useRoute()
const router = useRouter()
const databaseId = route.params.id as string
const databaseName = ref('')
const userRole = ref('')

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const tables = ref<Table[]>([])

const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  description: '',
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入表名称', trigger: 'blur' },
    { min: 2, max: 100, message: '长度在 2-100 个字符之间', trigger: 'blur' },
  ],
  description: [
    { max: 500, message: '描述不能超过500个字符', trigger: 'blur' },
  ],
}

const dialogTitle = ref('创建表')

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

// 权限判断
const canCreate = computed(() => ['owner', 'admin', 'editor'].includes(userRole.value))
const canDelete = computed(() => ['owner', 'admin'].includes(userRole.value))

const goBack = () => {
  router.push('/databases')
}

const loadTables = async () => {
  loading.value = true
  try {
    const response = await databaseAPI.getTables(databaseId)
    if (response.success && response.data) {
      tables.value = response.data.tables || []
    }
  } catch (error) {
    ElMessage.error('加载表列表失败')
  } finally {
    loading.value = false
  }
}

const loadDatabaseInfo = async () => {
  try {
    const response = await databaseAPI.getDetail(databaseId)
    if (response.success && response.data) {
      databaseName.value = response.data.name || ''
      userRole.value = response.data.role || 'viewer'
    }
  } catch (error) {
    console.error('Failed to load database info:', error)
  }
}

const handleCreate = () => {
  dialogTitle.value = '创建表'
  form.value = { name: '', description: '' }
  dialogVisible.value = true
}

const handleFields = (row: Table) => {
  router.push(`/tables/${row.id}/fields`)
}

const handleRecords = (row: Table) => {
  router.push(`/tables/${row.id}/records`)
}

const handleDelete = async (row: Table) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除表 "${row.name}" 吗？相关数据将被清空。`,
      '警告',
      {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消',
      }
    )
    const response = await tableAPI.delete(row.id)
    if (response.success) {
      ElMessage.success('删除成功')
      await loadTables()
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

    const response = await tableAPI.create({
      database_id: databaseId,
      name: form.value.name,
      description: form.value.description,
    })

    if (response.success) {
      ElMessage.success('创建成功')
      dialogVisible.value = false
      await loadTables()
    }
  } catch (error) {
    ElMessage.error('创建失败')
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
  loadDatabaseInfo()
  loadTables()
})
</script>

<style scoped lang="scss">
.tables {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;

    .header-left {
      display: flex;
      align-items: center;
      gap: 12px;

      .database-name {
        font-weight: 600;
        font-size: 16px;
      }
    }
  }
}
</style>
