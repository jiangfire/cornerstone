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
        <el-table-column label="操作" width="180" fixed="right">
          <template #default="{ row }">
            <el-button size="small" @click="handleView(row)">查看</el-button>
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
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { organizationAPI } from '@/services/api'

interface Organization {
  id: string
  name: string
  description?: string
  role: string
  created_at: string
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