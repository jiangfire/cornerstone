import { beforeEach, describe, expect, it, vi } from 'vitest'

import { flushUi, mountView } from '@/test-utils/view-test-helpers'
import DataBrowserView from '@/views/DataBrowserView.vue'

const { listDatabasesMock, listTablesMock, listFieldsMock, listRecordsMock } = vi.hoisted(() => ({
  listDatabasesMock: vi.fn(),
  listTablesMock: vi.fn(),
  listFieldsMock: vi.fn(),
  listRecordsMock: vi.fn(),
}))

vi.mock('@/services/api', () => ({
  databaseAPI: {
    list: listDatabasesMock,
    create: vi.fn(),
    createWithTables: vi.fn(),
    delete: vi.fn(),
  },
  tableAPI: {
    list: listTablesMock,
    create: vi.fn(),
  },
  fieldAPI: {
    list: listFieldsMock,
    delete: vi.fn(),
  },
  recordAPI: {
    list: listRecordsMock,
  },
}))

describe('DataBrowserView', () => {
  beforeEach(() => {
    listDatabasesMock.mockReset()
    listTablesMock.mockReset()
    listFieldsMock.mockReset()
    listRecordsMock.mockReset()
  })

  it('loads records for the selected table and shows record details', async () => {
    listDatabasesMock.mockResolvedValue({
      code: 0,
      data: {
        databases: [{ id: 'db_1', name: 'CRM', description: 'crm', created_at: '', updated_at: '' }],
      },
    })
    listTablesMock.mockResolvedValue({
      code: 0,
      data: {
        tables: [{ id: 'tbl_1', database_id: 'db_1', name: 'customers', description: '', created_at: '', updated_at: '' }],
      },
    })
    listFieldsMock.mockResolvedValue({
      code: 0,
      data: {
        items: [{ id: 'fld_1', table_id: 'tbl_1', name: 'name', type: 'string', description: '', required: true, options: '', created_at: '', updated_at: '' }],
      },
    })
    listRecordsMock.mockResolvedValue({
      code: 0,
      data: {
        items: [{ id: 'rec_1', table_id: 'tbl_1', data: { name: 'Alice' }, version: 1, created_at: '', updated_at: '' }],
        total: 1,
        has_more: false,
      },
    })

    const wrapper = mountView(DataBrowserView)
    await flushUi()

    await wrapper.get('.db-list li').trigger('click')
    await flushUi()
    await wrapper.get('.table-list li').trigger('click')
    await flushUi()

    expect(listRecordsMock).toHaveBeenCalled()
    expect(wrapper.text()).toContain('Alice')

    await wrapper.get('.records-table tbody tr').trigger('click')
    await flushUi()

    expect(wrapper.text()).toContain('"name": "Alice"')
  })
})
