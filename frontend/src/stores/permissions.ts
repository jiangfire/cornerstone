import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import { fieldAPI } from '@/services/api'
import { ElMessage } from 'element-plus'

// 字段权限接口
export interface FieldPermission {
  id?: string
  table_id?: string
  field_id: string
  role: string
  can_read: boolean
  can_write: boolean
  can_delete: boolean
  created_at?: string
  updated_at?: string
}

// 用户字段权限映射
interface UserFieldPermission {
  fieldId: string
  canRead: boolean
  canWrite: boolean
  canDelete: boolean
}

export const usePermissionStore = defineStore('permissions', () => {
  // State
  const fieldPermissions = ref<Map<string, FieldPermission[]>>(new Map())
  const userPermissions = ref<Map<string, UserFieldPermission>>(new Map())
  const loading = ref(false)
  const currentRole = ref<string>('viewer')

  // Getters
  const permissionsByTable = computed(() => {
    return (tableId: string) => fieldPermissions.value.get(tableId) || []
  })

  const getFieldPermission = computed(() => {
    return (fieldId: string, role?: string) => {
      const effectiveRole = role || currentRole.value
      const key = `${fieldId}_${effectiveRole}`
      return userPermissions.value.get(key)
    }
  })

  // Actions
  // 加载表的字段权限配置
  const loadFieldPermissions = async (tableId: string) => {
    loading.value = true
    try {
      const response = await fieldAPI.getPermissions(tableId)
      if (response.success && response.data?.permissions) {
        fieldPermissions.value.set(tableId, response.data.permissions)
        // 更新用户权限缓存
        updateUserPermissionCache(response.data.permissions)
        return response.data.permissions
      }
      return []
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : '加载字段权限失败')
      return []
    } finally {
      loading.value = false
    }
  }

  // 更新用户权限缓存
  const updateUserPermissionCache = (permissions: FieldPermission[]) => {
    permissions.forEach((perm) => {
      const key = `${perm.field_id}_${perm.role}`
      userPermissions.value.set(key, {
        fieldId: perm.field_id,
        canRead: perm.can_read,
        canWrite: perm.can_write,
        canDelete: perm.can_delete,
      })
    })
  }

  // 检查字段权限
  const checkFieldPermission = (
    fieldId: string,
    action: 'read' | 'write' | 'delete',
    role?: string,
  ): boolean => {
    const effectiveRole = role || currentRole.value
    const key = `${fieldId}_${effectiveRole}`
    const perm = userPermissions.value.get(key)

    // 如果没有配置字段级权限，使用默认权限
    if (!perm) {
      switch (effectiveRole) {
        case 'owner':
        case 'admin':
          return true
        case 'editor':
          return action !== 'delete'
        case 'viewer':
          return action === 'read'
        default:
          return false
      }
    }

    // 使用配置的权限
    switch (action) {
      case 'read':
        return perm.canRead
      case 'write':
        return perm.canWrite
      case 'delete':
        return perm.canDelete
      default:
        return false
    }
  }

  // 设置字段权限
  const setFieldPermission = async (tableId: string, permission: FieldPermission) => {
    loading.value = true
    try {
      const response = await fieldAPI.setPermission(tableId, permission)
      if (response.success) {
        ElMessage.success('权限设置成功')
        // 重新加载权限
        await loadFieldPermissions(tableId)
        return true
      }
      return false
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : '权限设置失败')
      return false
    } finally {
      loading.value = false
    }
  }

  // 批量设置字段权限
  const batchSetFieldPermissions = async (tableId: string, permissions: FieldPermission[]) => {
    loading.value = true
    try {
      const response = await fieldAPI.batchSetPermissions(tableId, permissions)
      if (response.success) {
        ElMessage.success(`成功设置 ${permissions.length} 条权限`)
        // 重新加载权限
        await loadFieldPermissions(tableId)
        return true
      }
      return false
    } catch (error: unknown) {
      ElMessage.error(error instanceof Error ? error.message : '批量权限设置失败')
      return false
    } finally {
      loading.value = false
    }
  }

  // 设置当前用户角色
  const setCurrentRole = (role: string) => {
    currentRole.value = role
  }

  // 清空权限缓存
  const clearPermissions = () => {
    fieldPermissions.value.clear()
    userPermissions.value.clear()
  }

  // 获取用户对特定字段的所有权限
  const getFieldPermissions = (fieldId: string, role?: string) => {
    const effectiveRole = role || currentRole.value
    const key = `${fieldId}_${effectiveRole}`
    return userPermissions.value.get(key)
  }

  // 过滤有权限的字段
  const filterAuthorizedFields = (
    fields: Array<{ id: string }>,
    action: 'read' | 'write' | 'delete' = 'read',
  ) => {
    return fields.filter((field) => checkFieldPermission(field.id, action))
  }

  return {
    // State
    fieldPermissions,
    userPermissions,
    loading,
    currentRole,

    // Getters
    permissionsByTable,
    getFieldPermission,

    // Actions
    loadFieldPermissions,
    checkFieldPermission,
    setFieldPermission,
    batchSetFieldPermissions,
    setCurrentRole,
    clearPermissions,
    getFieldPermissions,
    filterAuthorizedFields,
  }
})
