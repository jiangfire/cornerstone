import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import DatabasesView from '../DatabasesView.vue'
import {
  clickButtonByText,
  flushUi,
  getSetupState,
  getInputByPlaceholder,
  mountView,
  setInputValue,
} from '@/test-utils/view-test-helpers'

const {
  pushMock,
  messageSuccess,
  messageError,
  messageWarning,
  confirmMock,
  databaseAPI,
  userAPI,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  messageWarning: vi.fn(),
  confirmMock: vi.fn(),
  databaseAPI: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
    getDetail: vi.fn(),
    getTables: vi.fn(),
    share: vi.fn(),
    listUsers: vi.fn(),
    updateUserRole: vi.fn(),
    removeUser: vi.fn(),
  },
  userAPI: {
    list: vi.fn(),
  },
}))

vi.mock('vue-router', () => ({
  useRouter: () => ({ push: pushMock }),
}))

vi.mock('@/services/api', () => ({
  databaseAPI,
  userAPI,
}))

vi.mock('element-plus', async () => {
  const actual = await vi.importActual<typeof import('element-plus')>('element-plus')
  return {
    ...actual,
    ElMessage: {
      success: messageSuccess,
      error: messageError,
      warning: messageWarning,
    },
    ElMessageBox: {
      confirm: confirmMock,
    },
  }
})

describe('DatabasesView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    confirmMock.mockResolvedValue(true)
    databaseAPI.list.mockResolvedValue({
      success: true,
      data: {
        databases: [
          {
            id: 'db_owner',
            name: 'Owner DB',
            description: 'owner desc',
            role: 'owner',
            is_public: true,
            created_at: '2026-03-29 10:00:00',
          },
          {
            id: 'db_admin',
            name: 'Admin DB',
            description: 'admin desc',
            role: 'admin',
            is_public: false,
            created_at: '2026-03-29 10:00:00',
          },
          {
            id: 'db_editor',
            name: 'Editor DB',
            description: 'editor desc',
            role: 'editor',
            is_public: false,
            created_at: '2026-03-29 10:00:00',
          },
          {
            id: 'db_viewer',
            name: 'Viewer DB',
            description: 'viewer desc',
            role: 'viewer',
            is_public: false,
            created_at: '2026-03-29 10:00:00',
          },
        ],
      },
    })
    databaseAPI.update.mockResolvedValue({ success: true })
    databaseAPI.create.mockResolvedValue({ success: true })
    databaseAPI.delete.mockResolvedValue({ success: true })
    databaseAPI.share.mockResolvedValue({ success: true })
    databaseAPI.updateUserRole.mockResolvedValue({ success: true })
    databaseAPI.removeUser.mockResolvedValue({ success: true })
    databaseAPI.listUsers.mockResolvedValue({
      success: true,
      data: {
        users: [
          {
            user_id: 'user_owner',
            username: 'owner',
            email: 'owner@example.com',
            role: 'owner',
            joined_at: '2026-03-29 10:00:00',
          },
          {
            user_id: 'user_editor',
            username: 'editor',
            email: 'editor@example.com',
            role: 'editor',
            joined_at: '2026-03-29 10:00:00',
          },
        ],
      },
    })
    userAPI.list.mockResolvedValue({
      success: true,
      data: {
        users: [{ id: 'user_viewer', username: 'viewer', email: 'viewer@example.com' }],
      },
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('shows manage and destructive actions only for permitted roles', async () => {
    mountView(DatabasesView)
    await flushUi()

    const shareButtons = Array.from(document.body.querySelectorAll('button')).filter((button) =>
      button.textContent?.includes('分享'),
    )
    const editButtons = Array.from(document.body.querySelectorAll('button')).filter((button) =>
      button.textContent?.includes('编辑'),
    )
    const deleteButtons = Array.from(document.body.querySelectorAll('button')).filter((button) =>
      button.textContent?.includes('删除'),
    )

    expect(shareButtons).toHaveLength(2)
    expect(editButtons).toHaveLength(2)
    expect(deleteButtons).toHaveLength(1)
  })

  it('submits edit requests with snake_case is_public field', async () => {
    mountView(DatabasesView)
    await flushUi()

    await clickButtonByText('编辑')
    await setInputValue(getInputByPlaceholder('请输入数据库名称'), 'Owner DB Updated')
    await clickButtonByText('确定')

    expect(databaseAPI.update).toHaveBeenCalledWith(
      'db_owner',
      expect.objectContaining({
        name: 'Owner DB Updated',
        description: 'owner desc',
        is_public: true,
      }),
    )
    expect(databaseAPI.update.mock.calls[0]?.[1]).not.toHaveProperty('isPublic')
    expect(messageSuccess).toHaveBeenCalledWith('更新成功')
  })

  it('loads share data and warns when no user is selected', async () => {
    mountView(DatabasesView)
    await flushUi()

    await clickButtonByText('分享')

    expect(databaseAPI.listUsers).toHaveBeenCalledWith('db_owner')
    expect(userAPI.list).toHaveBeenCalledWith({ db_id: 'db_owner' })

    await clickButtonByText('添加成员')

    expect(messageWarning).toHaveBeenCalledWith('请选择要分享的用户')
    expect(databaseAPI.share).not.toHaveBeenCalled()
  })

  it('creates a database with snake_case is_public field and refreshes the list', async () => {
    const wrapper = mountView(DatabasesView)
    await flushUi()

    const vm = getSetupState<any>(wrapper)
    vm.handleCreate()
    vm.formRef = {
      validate: vi.fn().mockResolvedValue(true),
      resetFields: vi.fn(),
    }
    vm.form = {
      name: 'Created DB',
      description: 'created desc',
      isPublic: true,
      id: '',
    }

    await vm.handleSubmit()

    expect(databaseAPI.create).toHaveBeenCalledWith({
      name: 'Created DB',
      description: 'created desc',
      is_public: true,
    })
    expect(databaseAPI.list).toHaveBeenCalledTimes(2)
    expect(messageSuccess).toHaveBeenCalledWith('创建成功')
  })

  it('reports create failures and keeps the user on the dialog flow', async () => {
    const wrapper = mountView(DatabasesView)
    await flushUi()

    databaseAPI.create.mockRejectedValueOnce(new Error('boom'))

    const vm = getSetupState<any>(wrapper)
    vm.handleCreate()
    vm.formRef = {
      validate: vi.fn().mockResolvedValue(true),
      resetFields: vi.fn(),
    }
    vm.form = {
      name: 'Broken DB',
      description: '',
      isPublic: false,
      id: '',
    }

    await vm.handleSubmit()

    expect(messageError).toHaveBeenCalledWith('创建失败')
    expect(databaseAPI.list).toHaveBeenCalledTimes(1)
  })

  it('navigates to the table view from the row action', async () => {
    mountView(DatabasesView)
    await flushUi()

    await clickButtonByText('表结构')

    expect(pushMock).toHaveBeenCalledWith('/databases/db_owner')
  })

  it('deletes a database after confirmation and refreshes the list', async () => {
    mountView(DatabasesView)
    await flushUi()

    await clickButtonByText('删除')

    expect(confirmMock).toHaveBeenCalled()
    expect(databaseAPI.delete).toHaveBeenCalledWith('db_owner')
    expect(databaseAPI.list).toHaveBeenCalledTimes(2)
    expect(messageSuccess).toHaveBeenCalledWith('删除成功')
  })

  it('suppresses delete errors on cancel but shows them on real failures', async () => {
    mountView(DatabasesView)
    await flushUi()

    confirmMock.mockRejectedValueOnce('cancel')
    await clickButtonByText('删除')

    expect(messageError).not.toHaveBeenCalled()
    expect(databaseAPI.delete).not.toHaveBeenCalled()

    confirmMock.mockResolvedValueOnce(true)
    databaseAPI.delete.mockRejectedValueOnce(new Error('failed'))
    await clickButtonByText('删除')

    expect(messageError).toHaveBeenCalledWith('删除失败')
  })

  it('shares a database, updates member roles, and removes members through the managed state', async () => {
    const wrapper = mountView(DatabasesView)
    await flushUi()

    const vm = getSetupState<any>(wrapper)

    await vm.handleManageUsers({
      id: 'db_owner',
      name: 'Owner DB',
      role: 'owner',
      is_public: true,
      created_at: '2026-03-29 10:00:00',
    })

    expect(databaseAPI.listUsers).toHaveBeenCalledWith('db_owner')
    expect(userAPI.list).toHaveBeenCalledWith({ db_id: 'db_owner' })

    vm.shareForm.userId = 'user_viewer'
    vm.shareForm.role = 'admin'
    await vm.handleShareSubmit()

    expect(databaseAPI.share).toHaveBeenCalledWith('db_owner', {
      user_id: 'user_viewer',
      role: 'admin',
    })
    expect(messageSuccess).toHaveBeenCalledWith('分享成功')
    expect(vm.shareForm.userId).toBe('')

    await vm.handleSharedRoleChange(
      {
        user_id: 'user_editor',
        username: 'editor',
        email: 'editor@example.com',
        role: 'editor',
        joined_at: '2026-03-29 10:00:00',
      },
      'viewer',
    )

    expect(databaseAPI.updateUserRole).toHaveBeenCalledWith('db_owner', 'user_editor', 'viewer')
    expect(messageSuccess).toHaveBeenCalledWith('角色更新成功')

    await vm.handleRemoveSharedUser({
      user_id: 'user_editor',
      username: 'editor',
      email: 'editor@example.com',
      role: 'editor',
      joined_at: '2026-03-29 10:00:00',
    })

    expect(databaseAPI.removeUser).toHaveBeenCalledWith('db_owner', 'user_editor')
    expect(messageSuccess).toHaveBeenCalledWith('成员已移除')
  })

  it('handles member-management failures without crashing and ignores owner mutations', async () => {
    const wrapper = mountView(DatabasesView)
    await flushUi()

    const vm = getSetupState<any>(wrapper)
    await vm.handleManageUsers({
      id: 'db_owner',
      name: 'Owner DB',
      role: 'owner',
      is_public: true,
      created_at: '2026-03-29 10:00:00',
    })

    databaseAPI.share.mockRejectedValueOnce(new Error('share failed'))
    vm.shareForm.userId = 'user_viewer'
    await vm.handleShareSubmit()
    expect(messageError).toHaveBeenCalledWith('分享失败')

    databaseAPI.updateUserRole.mockRejectedValueOnce(new Error('role failed'))
    await vm.handleSharedRoleChange(
      {
        user_id: 'user_editor',
        username: 'editor',
        email: 'editor@example.com',
        role: 'editor',
        joined_at: '2026-03-29 10:00:00',
      },
      'admin',
    )
    expect(messageError).toHaveBeenCalledWith('角色更新失败')

    databaseAPI.removeUser.mockRejectedValueOnce(new Error('remove failed'))
    await vm.handleRemoveSharedUser({
      user_id: 'user_editor',
      username: 'editor',
      email: 'editor@example.com',
      role: 'editor',
      joined_at: '2026-03-29 10:00:00',
    })
    expect(messageError).toHaveBeenCalledWith('移除失败')

    await vm.handleSharedRoleChange(
      {
        user_id: 'user_owner',
        username: 'owner',
        email: 'owner@example.com',
        role: 'owner',
        joined_at: '2026-03-29 10:00:00',
      },
      'viewer',
    )
    await vm.handleRemoveSharedUser({
      user_id: 'user_owner',
      username: 'owner',
      email: 'owner@example.com',
      role: 'owner',
      joined_at: '2026-03-29 10:00:00',
    })

    expect(databaseAPI.updateUserRole).toHaveBeenCalledTimes(1)
    expect(databaseAPI.removeUser).toHaveBeenCalledTimes(1)
  })

  it('reports list and share-dialog loading failures', async () => {
    databaseAPI.list.mockRejectedValueOnce(new Error('list failed'))
    databaseAPI.listUsers.mockRejectedValueOnce(new Error('users failed'))
    userAPI.list.mockRejectedValueOnce(new Error('candidates failed'))

    const wrapper = mountView(DatabasesView)
    await flushUi()

    expect(messageError).toHaveBeenCalledWith('加载数据库列表失败')

    const vm = getSetupState<any>(wrapper)
    await vm.handleManageUsers({
      id: 'db_owner',
      name: 'Owner DB',
      role: 'owner',
      is_public: true,
      created_at: '2026-03-29 10:00:00',
    })

    expect(messageError).toHaveBeenCalledWith('加载数据库成员失败')
    expect(messageError).toHaveBeenCalledWith('加载可分享用户失败')
  })
})
