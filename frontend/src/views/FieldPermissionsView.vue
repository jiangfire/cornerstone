<template>
  <div class="field-permissions">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button link @click="goBack">
              <el-icon><ArrowLeft /></el-icon>
              返回字段列表
            </el-button>
            <span class="table-name">{{ tableName }} - 字段权限配置</span>
          </div>
          <div class="header-actions">
            <el-button @click="resetToDefault">重置为默认</el-button>
            <el-button type="primary" @click="savePermissions" :loading="saving">
              保存配置
            </el-button>
          </div>
        </div>
      </template>

      <el-alert
        title="权限说明"
        type="info"
        :closable="false"
        style="margin-bottom: 20px"
      >
        <ul style="margin: 5px 0 0 20px; padding: 0">
          <li><strong>R (Read)</strong>: 读取权限 - 允许查看字段内容</li>
          <li><strong>W (Write)</strong>: 写入权限 - 允许编辑字段内容</li>
          <li><strong>D (Delete)</strong>: 删除权限 - 允许删除字段</li>
          <li>未配置时使用默认权限：Owner/Admin 全部权限，Editor 可读写，Viewer 仅读取</li>
        </ul>
      </el-alert>

      <el-empty v-if="fields.length === 0" description="暂无字段" />

      <div v-else class="permission-matrix-container">
        <el-table :data="permissionMatrix" border style="width: 100%" v-loading="loading">
          <el-table-column prop="fieldName" label="字段名称" min-width="150" fixed />
          <el-table-column prop="fieldType" label="字段类型" width="100">
            <template #default="{ row }">
              <el-tag :type="getFieldTypeColor(row.fieldType)" size="small">
                {{ getFieldTypeLabel(row.fieldType) }}
              </el-tag>
            </template>
          </el-table-column>

          <!-- 为每个角色创建一列 -->
          <el-table-column
            v-for="role in roles"
            :key="role.value"
            :label="role.label"
            min-width="120"
          >
            <template #header>
              <div class="role-header">
                <span>{{ role.label }}</span>
              </div>
            </template>
            <template #default="{ row }">
              <div class="permission-checkboxes">
                <el-checkbox
                  v-model="row.permissions[role.value].canRead"
                  :disabled="!canEditRole(role.value)"
                  @change="onPermissionChange"
                >
                  R
                </el-checkbox>
                <el-checkbox
                  v-model="row.permissions[role.value].canWrite"
                  :disabled="!canEditRole(role.value)"
                  @change="onPermissionChange"
                >
                  W
                </el-checkbox>
                <el-checkbox
                  v-model="row.permissions[role.value].canDelete"
                  :disabled="!canEditRole(role.value)"
                  @change="onPermissionChange"
                >
                  D
                </el-checkbox>
              </div>
            </template>
          </el-table-column>
        </el-table>

        <!-- 批量操作区域 -->
        <div class="batch-actions">
          <div class="batch-actions-title">批量操作</div>
          <div class="batch-actions-buttons">
            <el-button size="small" @click="selectAll('canRead')">全选读取</el-button>
            <el-button size="small" @click="selectAll('canWrite')">全选写入</el-button>
            <el-button size="small" @click="selectAll('canDelete')">全选删除</el-button>
            <el-button size="small" @click="clearAll()">清空全部</el-button>
            <el-dropdown split-button type="primary" size="small" @click="applyTemplate('editor')">
              应用模板
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item @click="applyTemplate('default')">默认权限</el-dropdown-item>
                  <el-dropdown-item @click="applyTemplate('viewer')">仅查看模式</el-dropdown-item>
                  <el-dropdown-item @click="applyTemplate('editor')">编辑模式</el-dropdown-item>
                  <el-dropdown-item @click="applyTemplate('strict')">严格模式</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
          </div>
        </div>
      </div>
    </el-card>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft } from '@element-plus/icons-vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tableAPI, fieldAPI } from '@/services/api'
import { usePermissionStore, type FieldPermission } from '@/stores/permissions'

interface Field {
  id: string
  name: string
  type: string
  required: boolean
}

interface PermissionCell {
  canRead: boolean
  canWrite: boolean
  canDelete: boolean
}

interface PermissionRow {
  fieldId: string
  fieldName: string
  fieldType: string
  permissions: Record<string, PermissionCell>
}

const route = useRoute()
const router = useRouter()
const tableId = route.params.id as string
const tableName = ref('')

const permissionStore = usePermissionStore()

const loading = ref(false)
const saving = ref(false)
const fields = ref<Field[]>([])

// 角色定义
const roles = [
  { value: 'owner', label: 'Owner' },
  { value: 'admin', label: 'Admin' },
  { value: 'editor', label: 'Editor' },
  { value: 'viewer', label: 'Viewer' },
]

// 权限矩阵数据
const permissionMatrix = ref<PermissionRow[]>([])

// 初始化权限矩阵
const initPermissionMatrix = () => {
  permissionMatrix.value = fields.value.map(field => {
    const permissions: Record<string, PermissionCell> = {}
    roles.forEach(role => {
      permissions[role.value] = {
        canRead: true,
        canWrite: role.value === 'owner' || role.value === 'admin' || role.value === 'editor',
        canDelete: role.value === 'owner' || role.value === 'admin',
      }
    })
    return {
      fieldId: field.id,
      fieldName: field.name,
      fieldType: field.type,
      permissions,
    }
  })
}

// 检查是否可以编辑该角色
const canEditRole = (role: string) => {
  // Owner 和 Admin 的权限不能编辑，始终拥有全部权限
  return role !== 'owner' && role !== 'admin'
}

// 加载字段列表
const loadFields = async () => {
  loading.value = true
  try {
    const response = await tableAPI.getFields(tableId)
    if (response.success && response.data) {
      fields.value = response.data.fields || []
      initPermissionMatrix()
      // 加载现有权限配置
      await loadExistingPermissions()
    }
  } catch (error) {
    ElMessage.error('加载字段列表失败')
  } finally {
    loading.value = false
  }
}

// 加载表信息
const loadTableInfo = async () => {
  try {
    const response = await tableAPI.get(tableId)
    if (response.success && response.data) {
      tableName.value = response.data.name || ''
    }
  } catch (error) {
    console.error('Failed to load table info:', error)
  }
}

// 加载现有权限配置
const loadExistingPermissions = async () => {
  try {
    const permissions = await permissionStore.loadFieldPermissions(tableId)
    if (permissions && permissions.length > 0) {
      // 将权限配置应用到矩阵中
      permissions.forEach((perm: FieldPermission) => {
        const row = permissionMatrix.value.find(r => r.fieldId === perm.field_id)
        if (row && row.permissions[perm.role]) {
          row.permissions[perm.role] = {
            canRead: perm.can_read,
            canWrite: perm.can_write,
            canDelete: perm.can_delete,
          }
        }
      })
    }
  } catch (error) {
    console.error('Failed to load existing permissions:', error)
  }
}

// 权限变化时的回调
const onPermissionChange = () => {
  // 可以在这里添加自动保存或标记为已修改的逻辑
}

// 保存权限配置
const savePermissions = async () => {
  saving.value = true
  try {
    // 构建批量权限配置
    const permissions: FieldPermission[] = []

    permissionMatrix.value.forEach(row => {
      roles.forEach(role => {
        // 只保存非 owner/admin 的权限，因为它们的权限是固定的
        if (canEditRole(role.value)) {
          permissions.push({
            field_id: row.fieldId,
            role: role.value,
            can_read: row.permissions[role.value].canRead,
            can_write: row.permissions[role.value].canWrite,
            can_delete: row.permissions[role.value].canDelete,
          })
        }
      })
    })

    const success = await permissionStore.batchSetFieldPermissions(tableId, permissions)
    if (success) {
      ElMessage.success('权限配置保存成功')
    }
  } catch (error) {
    ElMessage.error('保存失败')
  } finally {
    saving.value = false
  }
}

// 重置为默认权限
const resetToDefault = () => {
  ElMessageBox.confirm(
    '确定要重置所有字段权限为默认值吗？此操作将覆盖当前所有自定义权限配置。',
    '确认重置',
    {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    }
  ).then(() => {
    initPermissionMatrix()
    ElMessage.success('已重置为默认权限，请点击保存按钮保存更改')
  }).catch(() => {
    // 用户取消
  })
}

// 全选指定权限
const selectAll = (permissionKey: keyof PermissionCell) => {
  permissionMatrix.value.forEach(row => {
    roles.forEach(role => {
      if (canEditRole(role.value)) {
        row.permissions[role.value][permissionKey] = true
      }
    })
  })
  ElMessage.info('请点击保存按钮保存更改')
}

// 清空全部权限
const clearAll = () => {
  permissionMatrix.value.forEach(row => {
    roles.forEach(role => {
      if (canEditRole(role.value)) {
        row.permissions[role.value] = {
          canRead: false,
          canWrite: false,
          canDelete: false,
        }
      }
    })
  })
  ElMessage.info('请点击保存按钮保存更改')
}

// 应用权限模板
const applyTemplate = (template: string) => {
  permissionMatrix.value.forEach(row => {
    roles.forEach(role => {
      if (canEditRole(role.value)) {
        switch (template) {
          case 'default':
            // 默认：Editor 可读写，Viewer 仅读取
            if (role.value === 'editor') {
              row.permissions[role.value] = { canRead: true, canWrite: true, canDelete: false }
            } else if (role.value === 'viewer') {
              row.permissions[role.value] = { canRead: true, canWrite: false, canDelete: false }
            }
            break
          case 'viewer':
            // 仅查看：所有角色只能读取
            row.permissions[role.value] = { canRead: true, canWrite: false, canDelete: false }
            break
          case 'editor':
            // 编辑：Editor 可读写，Viewer 仅读取
            if (role.value === 'editor') {
              row.permissions[role.value] = { canRead: true, canWrite: true, canDelete: false }
            } else if (role.value === 'viewer') {
              row.permissions[role.value] = { canRead: true, canWrite: false, canDelete: false }
            }
            break
          case 'strict':
            // 严格：Viewer 无任何权限
            if (role.value === 'editor') {
              row.permissions[role.value] = { canRead: true, canWrite: true, canDelete: false }
            } else if (role.value === 'viewer') {
              row.permissions[role.value] = { canRead: false, canWrite: false, canDelete: false }
            }
            break
        }
      }
    })
  })
  ElMessage.success(`已应用"${template}"模板，请点击保存按钮保存更改`)
}

const getFieldTypeLabel = (type: string) => {
  const typeMap: Record<string, string> = {
    string: '字符串',
    number: '数字',
    boolean: '布尔',
    date: '日期',
    datetime: '时间',
    text: '文本',
    select: '单选',
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

const goBack = () => {
  router.push(`/tables/${tableId}/fields`)
}

onMounted(() => {
  loadTableInfo()
  loadFields()
})
</script>

<style scoped lang="scss">
.field-permissions {
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

    .header-actions {
      display: flex;
      gap: 10px;
    }
  }

  .permission-matrix-container {
    .role-header {
      font-weight: 600;
      text-align: center;
    }

    .permission-checkboxes {
      display: flex;
      gap: 8px;
      justify-content: center;

      :deep(.el-checkbox) {
        margin: 0;
      }

      :deep(.el-checkbox__label) {
        padding-left: 4px;
      }
    }

    .batch-actions {
      margin-top: 20px;
      padding: 16px;
      background-color: #f5f7fa;
      border-radius: 4px;

      .batch-actions-title {
        font-weight: 600;
        margin-bottom: 12px;
        color: #303133;
      }

      .batch-actions-buttons {
        display: flex;
        gap: 10px;
        flex-wrap: wrap;
      }
    }
  }
}
</style>
