<template>
  <div class="organizations">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <span>组织管理</span>
          <el-button type="primary" @click="handleCreate">新建组织</el-button>
        </div>
      </template>

      <el-empty v-if="organizations.length === 0" description="暂无组织，请创建您的第一个组织">
        <el-button type="primary" @click="handleCreate">创建组织</el-button>
      </el-empty>

      <el-table v-else :data="organizations" style="width: 100%" v-loading="loading">
        <el-table-column prop="name" label="组织名称" min-width="180" />
        <el-table-column prop="role" label="角色" width="120">
          <template #default="{ row }">
            <el-tag :type="getRoleType(row.role)">{{ row.role }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="240" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleView(row)">查看</el-button>
            <el-button size="small" type="primary" @click="handleEdit(row)">编辑</el-button>
            <el-button size="small" type="info" @click="handleManageMembers(row)">成员管理</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 成员管理面板 -->
    <el-card v-if="selectedOrg" class="box-card member-card" style="margin-top: 16px;">
      <template #header>
        <div class="card-header">
          <span>成员管理 - {{ selectedOrg.name }}</span>
          <div>
            <el-button link @click="closeMemberPanel">关闭</el-button>
            <el-button type="primary" @click="handleAddMember" :disabled="!canManageMembers">
              添加成员
            </el-button>
          </div>
        </div>
      </template>

      <el-table :data="members" v-loading="membersLoading" border>
        <el-table-column prop="username" label="用户名" min-width="120" />
        <el-table-column prop="email" label="邮箱" min-width="180" />
        <el-table-column prop="role" label="角色" width="130">
          <template #default="{ row }">
            <el-select
              v-model="row.role"
              size="small"
              :disabled="!canManageMembers || row.role === 'owner'"
              @change="(val) => handleRoleChange(row, val)"
              style="width: 100%"
            >
              <el-option label="Owner" value="owner" />
              <el-option label="Admin" value="admin" />
              <el-option label="Member" value="member" />
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
              @click="handleRemoveMember(row)"
              :disabled="!canManageMembers || row.role === 'owner'"
            >
              移除
            </el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-card>

    <!-- 创建/编辑组织对话框 -->
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
        <el-form-item label="组织名称" prop="name">
          <el-input v-model="form.name" placeholder="请输入组织名称" />
        </el-form-item>
        <el-form-item label="描述" prop="description">
          <el-input
            v-model="form.description"
            type="textarea"
            :rows="3"
            placeholder="请输入组织描述（可选）"
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

    <!-- 添加成员对话框 -->
    <el-dialog
      v-model="addMemberDialogVisible"
      title="添加成员"
      width="500px"
      @close="resetAddMemberForm"
    >
      <el-form
        ref="addMemberFormRef"
        :model="addMemberForm"
        :rules="addMemberRules"
        label-width="100px"
        :loading="submitting"
      >
        <el-form-item label="用户" prop="user_id">
          <el-select
            v-model="addMemberForm.user_id"
            filterable
            placeholder="搜索用户"
            :loading="userLoading"
            style="width: 100%"
            @focus="loadAvailableUsers"
          >
            <el-option
              v-for="user in availableUsers"
              :key="user.id"
              :label="`${user.username} (${user.email})`"
              :value="user.id"
            />
          </el-select>
        </el-form-item>
        <el-form-item label="角色" prop="role">
          <el-select v-model="addMemberForm.role" placeholder="选择角色" style="width: 100%">
            <el-option label="Owner" value="owner" />
            <el-option label="Admin" value="admin" />
            <el-option label="Member" value="member" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="dialog-footer">
          <el-button @click="addMemberDialogVisible = false">取消</el-button>
          <el-button type="primary" @click="submitAddMember" :loading="submitting">
            确定
          </el-button>
        </span>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { organizationAPI, userAPI } from '@/services/api'

interface Organization {
  id: string
  name: string
  description?: string
  role: string
  created_at: string
}

interface Member {
  id: string
  organization_id: string
  user_id: string
  username: string
  email: string
  role: string
  joined_at: string
}

interface User {
  id: string
  username: string
  email: string
}

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const isEditMode = ref(false)
const organizations = ref<Organization[]>([])

const formRef = ref<FormInstance>()
const form = ref({
  name: '',
  description: '',
  id: '',
})

const rules: FormRules = {
  name: [
    { required: true, message: '请输入组织名称', trigger: 'blur' },
    { min: 2, max: 50, message: '长度在 2-50 个字符之间', trigger: 'blur' },
  ],
  description: [
    { max: 200, message: '描述不能超过200个字符', trigger: 'blur' },
  ],
}

const dialogTitle = ref('创建组织')

// 成员管理相关状态
const selectedOrg = ref<Organization | null>(null)
const members = ref<Member[]>([])
const membersLoading = ref(false)
const addMemberDialogVisible = ref(false)
const addMemberFormRef = ref<FormInstance>()
const addMemberForm = ref({ user_id: '', role: 'member' })
const userLoading = ref(false)
const availableUsers = ref<User[]>([])

// 当前用户信息（从localStorage获取）
const currentUser = ref<{ id: string; role: string }>({ id: '', role: '' })

// 权限计算
const canManageMembers = computed(() => {
  if (!selectedOrg.value) return false
  return currentUser.value.role === 'owner' || currentUser.value.role === 'admin'
})

// 添加成员表单验证规则
const addMemberRules: FormRules = {
  user_id: [{ required: true, message: '请选择用户', trigger: 'change' }],
  role: [{ required: true, message: '请选择角色', trigger: 'change' }],
}

// 获取角色标签类型
const getRoleType = (role: string) => {
  const roleMap: Record<string, string> = {
    owner: 'success',
    admin: 'primary',
    member: 'info',
  }
  return roleMap[role] || 'info'
}

// 格式化日期
const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

// 加载组织列表
const loadOrganizations = async () => {
  loading.value = true
  try {
    const response = await organizationAPI.list()
    if (response.success && response.data) {
      organizations.value = response.data.organizations || []
    }
  } catch (error) {
    ElMessage.error('加载组织列表失败')
  } finally {
    loading.value = false
  }
}

// 处理创建
const handleCreate = () => {
  isEditMode.value = false
  dialogTitle.value = '创建组织'
  form.value = { name: '', description: '', id: '' }
  dialogVisible.value = true
}

// 处理编辑
const handleEdit = (row: Organization) => {
  isEditMode.value = true
  dialogTitle.value = '编辑组织'
  form.value = {
    name: row.name,
    description: row.description || '',
    id: row.id,
  }
  dialogVisible.value = true
}

// 处理查看
const handleView = (row: Organization) => {
  ElMessageBox.alert(
    `
    <strong>组织名称:</strong> ${row.name}<br/>
    <strong>角色:</strong> ${row.role}<br/>
    <strong>创建时间:</strong> ${formatDate(row.created_at)}<br/>
    <strong>描述:</strong> ${row.description || '无'}
    `,
    '组织详情',
    {
      dangerouslyUseHTMLString: true,
      confirmButtonText: '确定',
    }
  )
}

// 处理删除
const handleDelete = async (row: Organization) => {
  try {
    await ElMessageBox.confirm(
      `确定要删除组织 "${row.name}" 吗？此操作不可恢复。`,
      '警告',
      {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消',
      }
    )

    const response = await organizationAPI.delete(row.id)
    if (response.success) {
      ElMessage.success('删除成功')
      await loadOrganizations()
      // 如果删除的是当前选中的组织，关闭成员面板
      if (selectedOrg.value?.id === row.id) {
        closeMemberPanel()
      }
    }
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 提交表单
const handleSubmit = async () => {
  if (!formRef.value) return

  try {
    const valid = await formRef.value.validate()
    if (!valid) return

    submitting.value = true

    let response
    if (isEditMode.value) {
      response = await organizationAPI.update(form.value.id, {
        name: form.value.name,
        description: form.value.description,
      })
    } else {
      response = await organizationAPI.create({
        name: form.value.name,
        description: form.value.description,
      })
    }

    if (response.success) {
      ElMessage.success(isEditMode.value ? '更新成功' : '创建成功')
      dialogVisible.value = false
      await loadOrganizations()
    }
  } catch (error) {
    ElMessage.error(isEditMode.value ? '更新失败' : '创建失败')
  } finally {
    submitting.value = false
  }
}

// 重置表单
const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
}

// 处理成员管理
const handleManageMembers = async (row: Organization) => {
  // 获取当前用户在该组织的角色
  currentUser.value = { id: 'current-user-id', role: row.role }
  selectedOrg.value = row
  await loadMembers()
}

// 加载成员列表
const loadMembers = async () => {
  if (!selectedOrg.value) return
  membersLoading.value = true
  try {
    const response = await organizationAPI.getMembers(selectedOrg.value.id)
    if (response.success) {
      members.value = response.data.members || []
    }
  } catch (error) {
    ElMessage.error('加载成员列表失败')
  } finally {
    membersLoading.value = false
  }
}

// 关闭成员面板
const closeMemberPanel = () => {
  selectedOrg.value = null
  members.value = []
}

// 加载可用用户列表
const loadAvailableUsers = async () => {
  if (!selectedOrg.value) return
  userLoading.value = true
  try {
    const response = await userAPI.list({ org_id: selectedOrg.value.id })
    if (response.success) {
      availableUsers.value = response.data.users || []
    }
  } catch (error) {
    ElMessage.error('加载用户列表失败')
    availableUsers.value = []
  } finally {
    userLoading.value = false
  }
}

// 处理添加成员
const handleAddMember = async () => {
  await loadAvailableUsers()
  addMemberForm.value = { user_id: '', role: 'member' }
  addMemberDialogVisible.value = true
}

// 重置添加成员表单
const resetAddMemberForm = () => {
  if (addMemberFormRef.value) {
    addMemberFormRef.value.resetFields()
  }
  availableUsers.value = []
}

// 提交添加成员
const submitAddMember = async () => {
  if (!addMemberFormRef.value || !selectedOrg.value) return

  try {
    const valid = await addMemberFormRef.value.validate()
    if (!valid) return

    submitting.value = true
    const response = await organizationAPI.addMember(
      selectedOrg.value.id,
      addMemberForm.value
    )

    if (response.success) {
      ElMessage.success('添加成员成功')
      addMemberDialogVisible.value = false
      await loadMembers()
    }
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '添加成员失败')
  } finally {
    submitting.value = false
  }
}

// 处理角色变更
const handleRoleChange = async (member: Member, newRole: string) => {
  if (!selectedOrg.value) return

  try {
    await ElMessageBox.confirm(
      `确定要将 ${member.username} 的角色改为 ${newRole} 吗？`,
      '确认',
      { type: 'warning' }
    )

    await organizationAPI.updateMemberRole(
      selectedOrg.value.id,
      member.id,
      newRole
    )

    ElMessage.success('角色更新成功')
    await loadMembers()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('角色更新失败')
    }
    // 恢复原值
    await loadMembers()
  }
}

// 处理移除成员
const handleRemoveMember = async (member: Member) => {
  if (!selectedOrg.value) return

  try {
    await ElMessageBox.confirm(
      `确定要移除成员 ${member.username} 吗？`,
      '警告',
      { type: 'warning' }
    )

    await organizationAPI.removeMember(selectedOrg.value.id, member.id)
    ElMessage.success('移除成员成功')
    await loadMembers()
  } catch (error) {
    if (error !== 'cancel') {
      ElMessage.error('移除成员失败')
    }
  }
}

onMounted(() => {
  loadOrganizations()
})
</script>

<style scoped lang="scss">
.organizations {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .dialog-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
  }
}
</style>