import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import PluginsView from '../PluginsView.vue'
import {
  clickButtonByText,
  clickExactButtonByText,
  flushUi,
  getSetupState,
  getInputByPlaceholder,
  getTextareaByPlaceholder,
  mountView,
  setInputValue,
} from '@/test-utils/view-test-helpers'

const { messageSuccess, messageError, messageWarning, confirmMock, pluginAPI, databaseAPI } =
  vi.hoisted(() => ({
    messageSuccess: vi.fn(),
    messageError: vi.fn(),
    messageWarning: vi.fn(),
    confirmMock: vi.fn(),
    pluginAPI: {
      list: vi.fn(),
      get: vi.fn(),
      create: vi.fn(),
      update: vi.fn(),
      delete: vi.fn(),
      bind: vi.fn(),
      getBindings: vi.fn(),
      unbind: vi.fn(),
      execute: vi.fn(),
      listExecutions: vi.fn(),
    },
    databaseAPI: {
      list: vi.fn(),
      getTables: vi.fn(),
    },
  }))

vi.mock('@/services/api', () => ({
  pluginAPI,
  databaseAPI,
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

describe('PluginsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    confirmMock.mockResolvedValue(true)
    pluginAPI.list.mockResolvedValue({
      data: [
        {
          id: 'plg_1',
          name: 'Sync Plugin',
          description: 'sync',
          language: 'bash',
          entry_file: 'main.sh',
          timeout: 30,
          created_by: 'usr_1',
          created_at: '2026-03-29 10:00:00',
        },
      ],
    })
    pluginAPI.create.mockResolvedValue({ data: { id: 'plg_2' } })
    pluginAPI.update.mockResolvedValue({ data: { id: 'plg_1' } })
    pluginAPI.delete.mockResolvedValue({})
    pluginAPI.bind.mockResolvedValue({})
    pluginAPI.unbind.mockResolvedValue({})
    pluginAPI.execute.mockResolvedValue({ data: { id: 'pex_1' } })
    pluginAPI.listExecutions.mockResolvedValue({
      data: [
        {
          id: 'pex_1',
          status: 'success',
          trigger: 'manual',
          table_id: 'tbl_1',
          record_id: 'rec_1',
          duration_ms: 12,
          created_at: '2026-03-29 10:00:00',
        },
      ],
    })
    databaseAPI.list.mockResolvedValue({
      data: {
        databases: [{ id: 'db_1' }, { id: 'db_2' }],
      },
    })
    databaseAPI.getTables.mockImplementation(async (databaseId: string) => ({
      data: {
        tables:
          databaseId === 'db_1'
            ? [{ id: 'tbl_1', name: 'Orders' }]
            : [{ id: 'tbl_2', name: 'Invoices' }],
      },
    }))
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  it('warns when plugin has no manual binding to execute', async () => {
    pluginAPI.getBindings.mockResolvedValue({
      data: [
        {
          table_id: 'tbl_1',
          table_name: 'Orders',
          database_name: 'Main DB',
          trigger: 'create',
        },
      ],
    })

    mountView(PluginsView)
    await flushUi()
    await clickButtonByText('手动执行')

    expect(pluginAPI.getBindings).toHaveBeenCalledWith('plg_1')
    expect(messageWarning).toHaveBeenCalledWith('请先为该插件绑定一个 manual 触发器')
    expect(pluginAPI.execute).not.toHaveBeenCalled()
  })

  it('blocks execution when payload is invalid JSON', async () => {
    pluginAPI.getBindings.mockResolvedValue({
      data: [
        {
          table_id: 'tbl_1',
          table_name: 'Orders',
          database_name: 'Main DB',
          trigger: 'manual',
        },
      ],
    })

    mountView(PluginsView)
    await flushUi()
    await clickButtonByText('手动执行')

    await setInputValue(getTextareaByPlaceholder('可选，JSON 对象，例如：{"source":"manual"}'), '{')
    await clickExactButtonByText('执行')

    expect(messageError).toHaveBeenCalledWith('Payload 必须是合法 JSON')
    expect(pluginAPI.execute).not.toHaveBeenCalled()
  })

  it('executes plugin with parsed payload and loads execution history', async () => {
    pluginAPI.getBindings.mockResolvedValue({
      data: [
        {
          table_id: 'tbl_1',
          table_name: 'Orders',
          database_name: 'Main DB',
          trigger: 'manual',
        },
      ],
    })

    mountView(PluginsView)
    await flushUi()

    await clickButtonByText('手动执行')
    await setInputValue(getInputByPlaceholder('可选，关联记录 ID'), 'rec_1')
    await setInputValue(
      getTextareaByPlaceholder('可选，JSON 对象，例如：{"source":"manual"}'),
      '{"source":"manual"}',
    )
    await clickExactButtonByText('执行')

    expect(pluginAPI.execute).toHaveBeenCalledWith('plg_1', {
      table_id: 'tbl_1',
      trigger: 'manual',
      record_id: 'rec_1',
      payload: { source: 'manual' },
    })
    expect(messageSuccess).toHaveBeenCalledWith('插件执行成功')

    await clickButtonByText('执行记录')

    expect(pluginAPI.listExecutions).toHaveBeenCalledWith('plg_1')
    expect(document.body.textContent).toContain('success')
  })

  it('creates a plugin with serialized config items and resets the create dialog state', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()

    const vm = getSetupState<any>(wrapper)
    vm.showCreateDialog()
    vm.form = {
      name: 'Created Plugin',
      description: 'created',
      language: 'python',
      entry_file: 'main.py',
      timeout: 45,
      config: '',
      config_values: '{"key":"value"}',
    }
    vm.configItems = [
      {
        name: 'source',
        type: 'string',
        default: 'manual',
        required: true,
      },
    ]

    await vm.handleSubmit()

    expect(pluginAPI.create).toHaveBeenCalledWith({
      name: 'Created Plugin',
      description: 'created',
      language: 'python',
      entry_file: 'main.py',
      timeout: 45,
      config: JSON.stringify(vm.configItems),
      config_values: '{"key":"value"}',
    })
    expect(pluginAPI.list).toHaveBeenCalledTimes(2)
    expect(messageSuccess).toHaveBeenCalledWith('创建成功')
  })

  it('edits plugins, parses stored config, and falls back safely on invalid config', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()

    const vm = getSetupState<any>(wrapper)
    vm.handleEdit({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      config: '[{"name":"enabled","type":"boolean","default":"true","required":false}]',
      config_values: '{"enabled":true}',
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })

    expect(vm.isEdit).toBe(true)
    expect(vm.configItems).toHaveLength(1)

    vm.form.description = 'updated'
    await vm.handleSubmit()

    expect(pluginAPI.update).toHaveBeenCalledWith('plg_1', {
      name: 'Sync Plugin',
      description: 'updated',
      timeout: 30,
      config: '[{"name":"enabled","type":"boolean","default":"true","required":false}]',
      config_values: '{"enabled":true}',
    })
    expect(messageSuccess).toHaveBeenCalledWith('更新成功')

    vm.handleEdit({
      id: 'plg_bad',
      name: 'Broken Config',
      description: 'broken',
      language: 'go',
      entry_file: 'main.go',
      timeout: 30,
      config: '{',
      config_values: '',
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })

    expect(vm.configItems).toEqual([])
  })

  it('reports operation failures when create or update submission throws', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    pluginAPI.create.mockRejectedValueOnce(new Error('create failed'))
    vm.showCreateDialog()
    await vm.handleSubmit()
    expect(messageError).toHaveBeenCalledWith('操作失败')

    pluginAPI.update.mockRejectedValueOnce(new Error('update failed'))
    vm.handleEdit({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      config: '',
      config_values: '',
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })
    await vm.handleSubmit()
    expect(messageError).toHaveBeenCalledWith('操作失败')
  })

  it('deletes plugins and handles cancel or failure paths safely', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    await vm.handleDelete({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })

    expect(pluginAPI.delete).toHaveBeenCalledWith('plg_1')
    expect(messageSuccess).toHaveBeenCalledWith('删除成功')

    confirmMock.mockRejectedValueOnce('cancel')
    await vm.handleDelete({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })
    expect(messageError).not.toHaveBeenCalledWith('删除失败')

    confirmMock.mockResolvedValueOnce(true)
    pluginAPI.delete.mockRejectedValueOnce(new Error('delete failed'))
    await vm.handleDelete({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })
    expect(messageError).toHaveBeenCalledWith('删除失败')
  })

  it('loads bindable tables, opens the bind dialog, and submits bindings', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    await vm.handleBind({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })

    expect(databaseAPI.list).toHaveBeenCalled()
    expect(databaseAPI.getTables).toHaveBeenCalledTimes(2)
    expect(vm.tables).toEqual([
      { id: 'tbl_1', name: 'Orders' },
      { id: 'tbl_2', name: 'Invoices' },
    ])

    vm.bindForm = { table_id: 'tbl_2', trigger: 'update' }
    await vm.handleConfirmBind()

    expect(pluginAPI.bind).toHaveBeenCalledWith('plg_1', { table_id: 'tbl_2', trigger: 'update' })
    expect(messageSuccess).toHaveBeenCalledWith('绑定成功')
  })

  it('handles bind loading, missing plugin, and bind failures correctly', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    databaseAPI.list.mockRejectedValueOnce(new Error('tables failed'))
    await vm.handleBind({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })
    expect(messageError).toHaveBeenCalledWith('加载可绑定表失败')
    expect(vm.tables).toEqual([])

    vm.currentPlugin = null
    await vm.handleConfirmBind()
    expect(pluginAPI.bind).toHaveBeenCalledTimes(0)

    vm.currentPlugin = { id: 'plg_1' }
    pluginAPI.bind.mockRejectedValueOnce(new Error('bind failed'))
    await vm.handleConfirmBind()
    expect(messageError).toHaveBeenCalledWith('绑定失败')
  })

  it('loads bindings and execution history and reports their failures', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    pluginAPI.getBindings.mockResolvedValueOnce({
      data: [
        {
          id: 'bind_1',
          table_id: 'tbl_1',
          table_name: 'Orders',
          database_name: 'Main DB',
          trigger: 'create',
          created_at: '2026-03-29 10:00:00',
        },
      ],
    })
    await vm.handleViewBindings({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })
    expect(vm.bindingsList).toHaveLength(1)

    pluginAPI.getBindings.mockRejectedValueOnce(new Error('bindings failed'))
    await vm.loadBindings('plg_1')
    expect(messageError).toHaveBeenCalledWith('加载绑定列表失败')

    pluginAPI.listExecutions.mockRejectedValueOnce(new Error('executions failed'))
    await vm.handleViewExecutions({
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    })
    expect(messageError).toHaveBeenCalledWith('加载执行记录失败')
  })

  it('warns on missing manual binding selection and reports execution failures', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    vm.currentPlugin = {
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    }
    vm.executeForm.bindingKey = ''
    await vm.handleExecuteSubmit()
    expect(messageWarning).toHaveBeenCalledWith('请选择要执行的绑定')

    vm.executeForm.bindingKey = 'tbl_1:manual'
    vm.executeForm.payload = '{"source":"manual"}'
    pluginAPI.execute.mockRejectedValueOnce(new Error('execute failed'))
    await vm.handleExecuteSubmit()
    expect(messageError).toHaveBeenCalledWith('插件执行失败')

    pluginAPI.getBindings.mockRejectedValueOnce(new Error('bindings failed'))
    await vm.handleRunPlugin(vm.currentPlugin)
    expect(messageError).toHaveBeenCalledWith('加载可执行绑定失败')
  })

  it('unbinds plugins and preserves cancel and failure semantics', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    vm.currentPlugin = {
      id: 'plg_1',
      name: 'Sync Plugin',
      description: 'sync',
      language: 'bash',
      entry_file: 'main.sh',
      timeout: 30,
      created_by: 'usr_1',
      created_at: '2026-03-29 10:00:00',
    }
    pluginAPI.getBindings.mockResolvedValue({ data: [] })

    await vm.handleUnbind({ id: 'bind_1', table_name: 'Orders', table_id: 'tbl_1' })
    expect(pluginAPI.unbind).toHaveBeenCalledWith('plg_1', { table_id: 'tbl_1' })
    expect(messageSuccess).toHaveBeenCalledWith('解绑成功')

    confirmMock.mockRejectedValueOnce('cancel')
    await vm.handleUnbind({ id: 'bind_1', table_name: 'Orders', table_id: 'tbl_1' })
    expect(messageError).not.toHaveBeenCalledWith('解绑失败')

    confirmMock.mockResolvedValueOnce(true)
    pluginAPI.unbind.mockRejectedValueOnce(new Error('unbind failed'))
    await vm.handleUnbind({ id: 'bind_1', table_name: 'Orders', table_id: 'tbl_1' })
    expect(messageError).toHaveBeenCalledWith('解绑失败')

    vm.currentPlugin = null
    await vm.handleUnbind({ id: 'bind_2', table_name: 'Invoices', table_id: 'tbl_2' })
    expect(pluginAPI.unbind).toHaveBeenCalledTimes(2)
  })

  it('covers config editor helpers and plugin list load errors', async () => {
    pluginAPI.list.mockRejectedValueOnce(new Error('network down'))
    const wrapper = mountView(PluginsView)
    await flushUi()

    expect(messageError).toHaveBeenCalledWith('network down')

    const vm = getSetupState<any>(wrapper)
    vm.showConfigEditor()
    expect(vm.configDialogVisible).toBe(true)

    vm.addConfigItem()
    vm.addConfigItem()
    expect(vm.configItems).toHaveLength(2)

    vm.removeConfigItem(0)
    expect(vm.configItems).toHaveLength(1)
  })

  it('covers mapping helpers for languages, triggers, statuses, and config parsing fallbacks', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    expect(vm.getLanguageType('go')).toBe('success')
    expect(vm.getLanguageType('python')).toBe('warning')
    expect(vm.getLanguageType('unknown')).toBe('')

    expect(vm.getTriggerLabel('delete')).toBe('删除时')
    expect(vm.getTriggerLabel('custom')).toBe('custom')
    expect(vm.getTriggerType('manual')).toBe('info')
    expect(vm.getTriggerType('other')).toBe('')

    expect(vm.getExecutionStatusType('timeout')).toBe('warning')
    expect(vm.getExecutionStatusType('other')).toBe('info')
    expect(vm.parseConfig('invalid-json')).toEqual([])
  })

  it('renders create, bind, execution, bindings, and config dialogs with their dynamic content', async () => {
    const wrapper = mountView(PluginsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    vm.showCreateDialog()
    vm.bindDialogVisible = true
    vm.tables = [{ id: 'tbl_1', name: 'Orders' }]
    vm.bindingsDialogVisible = true
    vm.bindingsList = [
      {
        id: 'bind_1',
        table_id: 'tbl_1',
        table_name: 'Orders',
        database_name: 'Main DB',
        trigger: 'manual',
        created_at: '2026-03-29 10:00:00',
      },
    ]
    vm.executeDialogVisible = true
    vm.manualBindings = [
      {
        table_id: 'tbl_1',
        table_name: 'Orders',
        database_name: 'Main DB',
        trigger: 'manual',
      },
    ]
    vm.executionsDialogVisible = true
    vm.executionsList = [
      {
        id: 'pex_running',
        status: 'running',
        trigger: 'manual',
        table_id: 'tbl_1',
        record_id: 'rec_1',
        duration_ms: 10,
        created_at: '2026-03-29 10:00:00',
      },
    ]
    vm.configDialogVisible = true
    vm.configItems = [
      {
        name: 'flag',
        type: 'boolean',
        default: 'true',
        required: true,
      },
    ]

    await flushUi()

    expect(document.body.textContent).toContain('创建插件')
    expect(document.body.textContent).toContain('绑定插件到表')
    expect(document.body.textContent).toContain('插件绑定管理')
    expect(document.body.textContent).toContain('手动执行插件')
    expect(document.body.textContent).toContain('插件执行记录')
    expect(document.body.textContent).toContain('配置插件参数')
    expect(document.body.textContent).toContain('Orders')
    expect(document.body.textContent).toContain('running')
    expect(document.body.querySelector('input[placeholder="参数名"]')).not.toBeNull()
  })
})
