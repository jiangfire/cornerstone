import { beforeEach, describe, expect, it, vi } from 'vitest'
import { computed, nextTick } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import { usePermissionStore, type FieldPermission } from '../permissions'

const fieldAPI = vi.hoisted(() => ({
  getPermissions: vi.fn(),
  setPermission: vi.fn(),
  batchSetPermissions: vi.fn(),
}))

const messageError = vi.hoisted(() => vi.fn())
const messageSuccess = vi.hoisted(() => vi.fn())

vi.mock('@/services/api', () => ({ fieldAPI }))

vi.mock('element-plus', () => ({
  ElMessage: {
    error: messageError,
    success: messageSuccess,
  },
}))

const makePerm = (overrides: Partial<FieldPermission> = {}): FieldPermission => ({
  field_id: 'f1',
  role: 'viewer',
  can_read: true,
  can_write: false,
  can_delete: false,
  ...overrides,
})

describe('permissions store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    fieldAPI.getPermissions.mockReset()
    fieldAPI.setPermission.mockReset()
    fieldAPI.batchSetPermissions.mockReset()
    messageError.mockReset()
    messageSuccess.mockReset()
  })

  describe('default role permissions (no config loaded)', () => {
    it('owner/admin can do everything', () => {
      const store = usePermissionStore()
      store.setCurrentRole('owner')
      expect(store.checkFieldPermission('any', 'read')).toBe(true)
      expect(store.checkFieldPermission('any', 'write')).toBe(true)
      expect(store.checkFieldPermission('any', 'delete')).toBe(true)

      store.setCurrentRole('admin')
      expect(store.checkFieldPermission('any', 'read')).toBe(true)
      expect(store.checkFieldPermission('any', 'write')).toBe(true)
      expect(store.checkFieldPermission('any', 'delete')).toBe(true)
    })

    it('editor can read/write but not delete', () => {
      const store = usePermissionStore()
      store.setCurrentRole('editor')
      expect(store.checkFieldPermission('any', 'read')).toBe(true)
      expect(store.checkFieldPermission('any', 'write')).toBe(true)
      expect(store.checkFieldPermission('any', 'delete')).toBe(false)
    })

    it('viewer can only read', () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')
      expect(store.checkFieldPermission('any', 'read')).toBe(true)
      expect(store.checkFieldPermission('any', 'write')).toBe(false)
      expect(store.checkFieldPermission('any', 'delete')).toBe(false)
    })

    it('unknown role denies everything', () => {
      const store = usePermissionStore()
      store.setCurrentRole('stranger')
      expect(store.checkFieldPermission('any', 'read')).toBe(false)
      expect(store.checkFieldPermission('any', 'write')).toBe(false)
      expect(store.checkFieldPermission('any', 'delete')).toBe(false)
    })
  })

  describe('loadFieldPermissions', () => {
    it('caches API response under tableId and updates user permission map', async () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')
      const perms: FieldPermission[] = [
        makePerm({ field_id: 'f1', role: 'viewer', can_read: false }),
        makePerm({ field_id: 'f2', role: 'viewer', can_read: true, can_write: true }),
      ]
      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: { permissions: perms },
      })

      const result = await store.loadFieldPermissions('table-1')

      expect(result).toEqual(perms)
      expect(store.permissionsByTable('table-1')).toEqual(perms)
      expect(store.checkFieldPermission('f1', 'read')).toBe(false)
      expect(store.checkFieldPermission('f2', 'write')).toBe(true)
    })

    it('returns empty array and shows error toast on rejection', async () => {
      const store = usePermissionStore()
      fieldAPI.getPermissions.mockRejectedValueOnce(new Error('boom'))

      const result = await store.loadFieldPermissions('table-1')

      expect(result).toEqual([])
      expect(messageError).toHaveBeenCalledWith('boom')
    })

    it('returns empty array on unsuccessful response without throwing', async () => {
      const store = usePermissionStore()
      fieldAPI.getPermissions.mockResolvedValueOnce({ code: 400 })

      const result = await store.loadFieldPermissions('table-1')

      expect(result).toEqual([])
      expect(messageError).not.toHaveBeenCalled()
    })
  })

  describe('reactivity (regression: ref<Map>().set() did not notify computeds)', () => {
    it('computed reading checkFieldPermission re-evaluates after loadFieldPermissions', async () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')

      const canWriteF1 = computed(() => store.checkFieldPermission('f1', 'write'))
      expect(canWriteF1.value).toBe(false) // viewer default → no write

      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: {
          permissions: [makePerm({ field_id: 'f1', role: 'viewer', can_write: true })],
        },
      })
      await store.loadFieldPermissions('table-1')
      await nextTick()

      expect(canWriteF1.value).toBe(true)
    })

    it('computed reading permissionsByTable re-evaluates after load', async () => {
      const store = usePermissionStore()
      const list = computed(() => store.permissionsByTable('table-1'))
      expect(list.value).toEqual([])

      const perms = [makePerm({ field_id: 'fx' })]
      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: { permissions: perms },
      })
      await store.loadFieldPermissions('table-1')
      await nextTick()

      expect(list.value).toEqual(perms)
    })

    it('clearPermissions causes computeds to revert to defaults', async () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')

      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: {
          permissions: [makePerm({ field_id: 'f1', role: 'viewer', can_read: false })],
        },
      })
      await store.loadFieldPermissions('table-1')

      const canReadF1 = computed(() => store.checkFieldPermission('f1', 'read'))
      expect(canReadF1.value).toBe(false)

      store.clearPermissions()
      await nextTick()

      expect(canReadF1.value).toBe(true) // back to viewer default
      expect(store.permissionsByTable('table-1')).toEqual([])
    })
  })

  describe('configured permission overrides default', () => {
    it('explicit can_read=false beats viewer default', async () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')

      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: {
          permissions: [
            makePerm({ field_id: 'secret', role: 'viewer', can_read: false }),
          ],
        },
      })
      await store.loadFieldPermissions('table-1')

      expect(store.checkFieldPermission('secret', 'read')).toBe(false)
    })

    it('role argument overrides currentRole', async () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')

      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: {
          permissions: [
            makePerm({
              field_id: 'shared',
              role: 'editor',
              can_read: true,
              can_write: true,
            }),
          ],
        },
      })
      await store.loadFieldPermissions('table-1')

      // 当前角色是 viewer，但显式传 editor 时应读到 editor 配置
      expect(store.checkFieldPermission('shared', 'write', 'editor')).toBe(true)
      expect(store.checkFieldPermission('shared', 'write')).toBe(false)
    })
  })

  describe('filterAuthorizedFields', () => {
    it('filters by configured read permission', async () => {
      const store = usePermissionStore()
      store.setCurrentRole('viewer')

      fieldAPI.getPermissions.mockResolvedValueOnce({
        code: 0,
        data: {
          permissions: [
            makePerm({ field_id: 'visible', role: 'viewer', can_read: true }),
            makePerm({ field_id: 'hidden', role: 'viewer', can_read: false }),
          ],
        },
      })
      await store.loadFieldPermissions('table-1')

      const filtered = store.filterAuthorizedFields([
        { id: 'visible' },
        { id: 'hidden' },
      ])
      expect(filtered).toEqual([{ id: 'visible' }])
    })
  })
})
