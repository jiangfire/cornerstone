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
        <el-table-column prop="role" label="我的角色" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="getRoleTagType(row.role)">{{ row.role || 'viewer' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="is_public" label="公开" width="100" align="center">
          <template #default="{ row }">
            <el-tag :type="row.is_public ? 'success' : 'info'">
              {{ row.is_public ? '公开' : '私有' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="table_count" label="表数量" width="100" align="center" />
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="320" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleTables(row)">表结构</el-button>
            <el-button
              v-if="canManageMembers(row)"
              size="small"
              type="info"
              @click="handleManageUsers(row)"
            >
              分享
            </el-button>
            <el-button v-if="canEdit(row)" size="small" type="primary" @click="handleEdit(row)">
              编辑
            </el-button>
            <el-button v-if="canDelete(row)" size="small" type="danger" @click="handleDelete(row)">
              删除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="500px" @close="resetForm">
      <el-form ref="formRef" :model="form" :rules="rules" label-width="100px" :loading="submitting">
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
          <el-button type="primary" @click="handleSubmit" :loading="submitting">确定</el-button>
        </span>
      </template>
    </el-dialog>

    <el-dialog
      v-model="shareDialogVisible"
      title="数据库分享与成员管理"
      width="820px"
      @close="resetShareState"
    >
      <div v-if="selectedDatabase" class="share-panel">
        <el-alert
          :title="`当前数据库：${selectedDatabase.name}`"
          type="info"
          :closable="false"
          style="margin-bottom: 16px"
        />

        <el-form inline :model="shareForm" class="share-form">
          <el-form-item label="用户">
            <el-select
              v-model="shareForm.userId"
              placeholder="请选择用户"
              filterable
              style="width: 260px"
              :loading="sharingCandidatesLoading"
            >
              <el-option
                v-for="candidate in shareCandidates"
                :key="candidate.id"
                :label="`${candidate.username} (${candidate.email})`"
                :value="candidate.id"
              />
            </el-select>
          </el-form-item>
          <el-form-item label="角色">
            <el-select v-model="shareForm.role" style="width: 140px">
              <el-option label="Admin" value="admin" />
              <el-option label="Editor" value="editor" />
              <el-option label="Viewer" value="viewer" />
            </el-select>
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="sharing" @click="handleShareSubmit">
              添加成员
            </el-button>
          </el-form-item>
        </el-form>

        <el-table :data="sharedUsers" v-loading="sharedUsersLoading" border>
          <el-table-column prop="username" label="用户名" min-width="140" />
          <el-table-column prop="email" label="邮箱" min-width="220" />
          <el-table-column prop="role" label="角色" width="160">
            <template #default="{ row }">
              <el-tag v-if="row.role === 'owner'" type="success">owner</el-tag>
              <el-select
                v-else
                v-model="row.role"
                style="width: 120px"
                @change="(role: 'admin' | 'editor' | 'viewer') => handleSharedRoleChange(row, role)"
              >
                <el-option label="Admin" value="admin" />
                <el-option label="Editor" value="editor" />
                <el-option label="Viewer" value="viewer" />
              </el-select>
            </template>
          </el-table-column>
          <el-table-column prop="joined_at" label="加入时间" width="180">
            <template #default="{ row }">
              {{ formatDate(row.joined_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="100" fixed="right">
            <template #default="{ row }">
              <el-button
                type="danger"
                size="small"
                :disabled="row.role === 'owner'"
                @click="handleRemoveSharedUser(row)"
              >
                移除
              </el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { databaseAPI, userAPI } from '@/services/api'
import { formatDate } from '@/utils/format'

interface Database {
  id: string
  name: string
  description?: string
  type?: string
  table_count?: number
  created_at: string
  role?: string
  is_public?: boolean
}

interface SharedUser {
  user_id: string
  username: string
  email: string
  role: 'owner' | 'admin' | 'editor' | 'viewer'
  joined_at: string
}

interface ShareCandidate {
  id: string
  username: string
  email: string
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const isEditMode = ref(false)
const databases = ref<Database[]>([])
const shareDialogVisible = ref(false)
const selectedDatabase = ref<Database | null>(null)
const sharedUsers = ref<SharedUser[]>([])
const sharedUsersLoading = ref(false)
const shareCandidates = ref<ShareCandidate[]>([])
const sharingCandidatesLoading = ref(false)
const sharing = ref(false)

const router = useRouter()

const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  description: '',
  isPublic: false,
  id: '',
})

const shareForm = ref<{
  userId: string
  role: 'admin' | 'editor' | 'viewer'
}>({
  userId: '',
  role: 'viewer',
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入数据库名称', trigger: 'blur' },
    { min: 2, max: 50, message: '长度在 2-50 个字符之间', trigger: 'blur' },
  ],
  description: [{ max: 200, message: '描述不能超过200个字符', trigger: 'blur' }],
}

const dialogTitle = ref('创建数据库')

const getRoleTagType = (role?: string) => {
  const roleMap: Record<string, string> = {
    owner: 'success',
    admin: 'primary',
    editor: 'warning',
    viewer: 'info',
  }
  return roleMap[role || 'viewer'] || 'info'
}

const canEdit = (row: Database) => {
  return ['owner', 'admin'].includes(row.role || '')
}

const canDelete = (row: Database) => {
  return row.role === 'owner'
}

const canManageMembers = (row: Database) => {
  return ['owner', 'admin'].includes(row.role || '')
}

const loadDatabases = async () => {
  loading.value = true
  try {
    const response = await databaseAPI.list()
    if (response.success && response.data) {
      databases.value = response.data.databases || []
    }
  } catch {
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
    isPublic: Boolean(row.is_public),
    id: row.id,
  }
  dialogVisible.value = true
}

const handleTables = (row: Database) => {
  router.push(`/databases/${row.id}`)
}

const handleDelete = async (row: Database) => {
  try {
    await ElMessageBox.confirm(`确定要删除数据库 "${row.name}" 吗？相关数据将被清空。`, '警告', {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    const response = await databaseAPI.delete(row.id)
    if (response.success) {
      ElMessage.success('删除成功')
      await loadDatabases()
    }
  } catch (err) {
    if (err !== 'cancel') {
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
      const response = await databaseAPI.update(form.value.id, {
        name: form.value.name,
        description: form.value.description,
        is_public: form.value.isPublic,
      })
      if (response.success) {
        ElMessage.success('更新成功')
      }
    } else {
      const response = await databaseAPI.create({
        name: form.value.name,
        description: form.value.description,
        is_public: form.value.isPublic,
      })
      if (response.success) {
        ElMessage.success('创建成功')
      }
    }

    dialogVisible.value = false
    await loadDatabases()
  } catch {
    ElMessage.error(isEditMode.value ? '更新失败' : '创建失败')
  } finally {
    submitting.value = false
  }
}

const loadSharedUsers = async () => {
  if (!selectedDatabase.value) return

  sharedUsersLoading.value = true
  try {
    const response = await databaseAPI.listUsers(selectedDatabase.value.id)
    if (response.success && response.data) {
      sharedUsers.value = response.data.users || []
    }
  } catch {
    ElMessage.error('加载数据库成员失败')
  } finally {
    sharedUsersLoading.value = false
  }
}

const loadShareCandidates = async () => {
  if (!selectedDatabase.value) return

  sharingCandidatesLoading.value = true
  try {
    const response = await userAPI.list({ db_id: selectedDatabase.value.id })
    if (response.success && response.data) {
      shareCandidates.value = response.data.users || []
    }
  } catch {
    ElMessage.error('加载可分享用户失败')
  } finally {
    sharingCandidatesLoading.value = false
  }
}

const handleManageUsers = async (row: Database) => {
  selectedDatabase.value = row
  shareDialogVisible.value = true
  shareForm.value = {
    userId: '',
    role: 'viewer',
  }
  await Promise.all([loadSharedUsers(), loadShareCandidates()])
}

const handleShareSubmit = async () => {
  if (!selectedDatabase.value || !shareForm.value.userId) {
    ElMessage.warning('请选择要分享的用户')
    return
  }

  sharing.value = true
  try {
    const response = await databaseAPI.share(selectedDatabase.value.id, {
      user_id: shareForm.value.userId,
      role: shareForm.value.role,
    })
    if (response.success) {
      ElMessage.success('分享成功')
      shareForm.value.userId = ''
      await Promise.all([loadSharedUsers(), loadShareCandidates()])
    }
  } catch {
    ElMessage.error('分享失败')
  } finally {
    sharing.value = false
  }
}

const handleSharedRoleChange = async (
  row: SharedUser,
  role: 'admin' | 'editor' | 'viewer',
) => {
  if (!selectedDatabase.value || row.role === 'owner') return

  try {
    const response = await databaseAPI.updateUserRole(selectedDatabase.value.id, row.user_id, role)
    if (response.success) {
      ElMessage.success('角色更新成功')
      await loadSharedUsers()
    }
  } catch {
    ElMessage.error('角色更新失败')
    await loadSharedUsers()
  }
}

const handleRemoveSharedUser = async (row: SharedUser) => {
  if (!selectedDatabase.value || row.role === 'owner') return

  try {
    await ElMessageBox.confirm(`确定移除用户 "${row.username}" 的数据库访问权限吗？`, '提示', {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    const response = await databaseAPI.removeUser(selectedDatabase.value.id, row.user_id)
    if (response.success) {
      ElMessage.success('成员已移除')
      await Promise.all([loadSharedUsers(), loadShareCandidates()])
    }
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error('移除失败')
    }
  }
}

const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

const resetShareState = () => {
  selectedDatabase.value = null
  sharedUsers.value = []
  shareCandidates.value = []
  shareForm.value = {
    userId: '',
    role: 'viewer',
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

  .share-form {
    margin-bottom: 16px;
  }
}
</style>
