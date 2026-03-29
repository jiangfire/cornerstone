import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import RecordsView from '../RecordsView.vue'
import { clickButtonByText, flushUi, getSetupState, mountView } from '@/test-utils/view-test-helpers'

const {
  pushMock,
  messageSuccess,
  messageError,
  messageWarning,
  confirmMock,
  tableAPI,
  recordAPI,
  fileAPI,
  databaseAPI,
  exportAPI,
  defaultApi,
} = vi.hoisted(() => ({
  pushMock: vi.fn(),
  messageSuccess: vi.fn(),
  messageError: vi.fn(),
  messageWarning: vi.fn(),
  confirmMock: vi.fn(),
  tableAPI: {
    get: vi.fn(),
    getFields: vi.fn(),
    create: vi.fn(),
    delete: vi.fn(),
  },
  recordAPI: {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    delete: vi.fn(),
  },
  fileAPI: {
    listByRecord: vi.fn(),
    downloadBlob: vi.fn(),
    download: vi.fn(),
    delete: vi.fn(),
  },
  databaseAPI: {
    getDetail: vi.fn(),
  },
  exportAPI: {
    downloadRecords: vi.fn(),
  },
  defaultApi: {
    post: vi.fn(),
  },
}))

vi.mock('vue-router', () => ({
  useRoute: () => ({ params: { id: 'tbl_1' } }),
  useRouter: () => ({ push: pushMock }),
}))

vi.mock('@/services/api', () => ({
  tableAPI,
  recordAPI,
  fileAPI,
  databaseAPI,
  exportAPI,
  default: defaultApi,
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

const defaultFields = [
  {
    id: 'fld_1',
    name: 'title',
    type: 'string',
    required: false,
    config: undefined,
    created_at: '2026-03-29 10:00:00',
  },
]

const defaultRecord = {
  id: 'rec_1',
  data: { title: 'First record' },
  version: 1,
  created_at: '2026-03-29 10:00:00',
  updated_at: '2026-03-29 10:00:00',
}

let createObjectURLMock: ReturnType<typeof vi.fn<(object: Blob) => string>>
let revokeObjectURLMock: ReturnType<typeof vi.fn<(url: string) => void>>

describe('RecordsView', () => {
  beforeEach(() => {
    vi.clearAllMocks()

    confirmMock.mockResolvedValue(true)
    tableAPI.get.mockResolvedValue({
      success: true,
      data: { name: 'Orders', database_id: 'db_1' },
    })
    tableAPI.getFields.mockResolvedValue({
      success: true,
      data: { fields: defaultFields },
    })
    databaseAPI.getDetail.mockResolvedValue({
      success: true,
      data: { role: 'owner' },
    })
    recordAPI.list.mockResolvedValue({
      success: true,
      data: {
        records: [defaultRecord],
        total: 1,
      },
    })
    recordAPI.create.mockResolvedValue({ success: true })
    recordAPI.update.mockResolvedValue({ success: true })
    recordAPI.delete.mockResolvedValue({ success: true })
    fileAPI.listByRecord.mockResolvedValue({
      data: [],
    })
    fileAPI.delete.mockResolvedValue({})
    fileAPI.downloadBlob.mockResolvedValue(new Blob(['file']))
    fileAPI.download.mockReturnValue('/api/files/direct-link')
    exportAPI.downloadRecords.mockResolvedValue(new Blob(['id,title'], { type: 'text/csv' }))
    defaultApi.post.mockResolvedValue({})

    createObjectURLMock = vi.fn(() => 'blob:mock-object-url')
    revokeObjectURLMock = vi.fn()
    Object.defineProperty(window.URL, 'createObjectURL', {
      configurable: true,
      writable: true,
      value: createObjectURLMock,
    })
    Object.defineProperty(window.URL, 'revokeObjectURL', {
      configurable: true,
      writable: true,
      value: revokeObjectURLMock,
    })
  })

  afterEach(() => {
    document.body.innerHTML = ''
  })

  const openEditDialogWithFiles = async (
    files: Array<{ id: string; file_name?: string; file_size: number; created_at: string }>,
  ) => {
    fileAPI.listByRecord.mockResolvedValueOnce({ data: files })

    mountView(RecordsView)
    await flushUi()
    await clickButtonByText('编辑')
    await flushUi()

    expect(fileAPI.listByRecord).toHaveBeenCalledWith('rec_1')
  }

  it('previews image attachments through blob download instead of direct links', async () => {
    const imageBlob = new Blob(['image'], { type: 'image/png' })
    fileAPI.downloadBlob.mockResolvedValueOnce(imageBlob)
    createObjectURLMock.mockReturnValueOnce('blob:image-preview')

    await openEditDialogWithFiles([
      {
        id: 'file_img',
        file_name: 'cover.png',
        file_size: 128,
        created_at: '2026-03-29 10:00:00',
      },
    ])

    await clickButtonByText('预览')

    expect(fileAPI.downloadBlob).toHaveBeenCalledWith('file_img')
    expect(fileAPI.download).not.toHaveBeenCalled()
    expect(createObjectURLMock).toHaveBeenCalledWith(imageBlob)

    const image = document.body.querySelector('img')
    expect(image).not.toBeNull()
    expect(image?.getAttribute('src')).toBe('blob:image-preview')
  })

  it('previews pdf attachments inside an iframe backed by a blob URL', async () => {
    const pdfBlob = new Blob(['%PDF-1.4'], { type: 'application/pdf' })
    fileAPI.downloadBlob.mockResolvedValueOnce(pdfBlob)
    createObjectURLMock.mockReturnValueOnce('blob:pdf-preview')

    await openEditDialogWithFiles([
      {
        id: 'file_pdf',
        file_name: 'manual.pdf',
        file_size: 256,
        created_at: '2026-03-29 10:00:00',
      },
    ])

    await clickButtonByText('预览')

    expect(fileAPI.downloadBlob).toHaveBeenCalledWith('file_pdf')
    expect(fileAPI.download).not.toHaveBeenCalled()

    const frame = document.body.querySelector('iframe')
    expect(frame).not.toBeNull()
    expect(frame?.getAttribute('src')).toBe('blob:pdf-preview')
  })

  it('surfaces preview errors from the protected API and does not fall back to direct URLs', async () => {
    fileAPI.downloadBlob.mockRejectedValueOnce({
      response: {
        data: {
          message: '无权预览该文件',
        },
      },
    })

    await openEditDialogWithFiles([
      {
        id: 'file_denied',
        file_name: 'secret.png',
        file_size: 64,
        created_at: '2026-03-29 10:00:00',
      },
    ])

    await clickButtonByText('预览')

    expect(messageError).toHaveBeenCalledWith('无权预览该文件')
    expect(fileAPI.download).not.toHaveBeenCalled()
    expect(document.body.querySelector('img')).toBeNull()
    expect(document.body.querySelector('iframe')).toBeNull()
  })

  it('downloads attachments via blob response and falls back to file id when file name is missing', async () => {
    const archiveBlob = new Blob(['archive'], { type: 'application/zip' })
    const anchorClick = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined)
    const appendChildSpy = vi.spyOn(document.body, 'appendChild')

    fileAPI.downloadBlob.mockResolvedValueOnce(archiveBlob)
    createObjectURLMock.mockReturnValueOnce('blob:download-link')

    await openEditDialogWithFiles([
      {
        id: 'file_zip',
        file_name: undefined,
        file_size: 512,
        created_at: '2026-03-29 10:00:00',
      },
    ])

    await clickButtonByText('下载')

    expect(fileAPI.downloadBlob).toHaveBeenCalledWith('file_zip')
    expect(fileAPI.download).not.toHaveBeenCalled()
    expect(createObjectURLMock).toHaveBeenCalledWith(archiveBlob)
    expect(anchorClick).toHaveBeenCalledTimes(1)

    const appendedAnchor = appendChildSpy.mock.calls
      .map(([node]) => node)
      .find((node): node is HTMLAnchorElement => node instanceof HTMLAnchorElement)

    expect(appendedAnchor).toBeDefined()
    expect(appendedAnchor?.download).toBe('file-file_zip')
    expect(appendedAnchor?.href).toContain('blob:download-link')
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:download-link')
  })

  it('hides create and destructive actions for viewer roles', async () => {
    databaseAPI.getDetail.mockResolvedValueOnce({
      success: true,
      data: { role: 'viewer' },
    })

    mountView(RecordsView)
    await flushUi()

    expect(document.body.textContent).not.toContain('新建记录')
    expect(document.body.textContent).not.toContain('编辑')
    expect(document.body.textContent).not.toContain('删除')
  })

  it('initializes create form defaults across field types and navigates back to databases', async () => {
    tableAPI.getFields.mockResolvedValueOnce({
      success: true,
      data: {
        fields: [
          { id: 'fld_string', name: 'title', type: 'string', required: true, created_at: '2026-03-29 10:00:00' },
          { id: 'fld_boolean', name: 'active', type: 'boolean', required: false, created_at: '2026-03-29 10:00:00' },
          { id: 'fld_date', name: 'start_date', type: 'date', required: false, created_at: '2026-03-29 10:00:00' },
          { id: 'fld_datetime', name: 'scheduled_at', type: 'datetime', required: false, created_at: '2026-03-29 10:00:00' },
          { id: 'fld_text', name: 'notes', type: 'text', required: false, created_at: '2026-03-29 10:00:00' },
          {
            id: 'fld_select',
            name: 'status',
            type: 'select',
            required: false,
            config: { options: ['todo', 'done'] },
            created_at: '2026-03-29 10:00:00',
          },
          {
            id: 'fld_multiselect',
            name: 'tags',
            type: 'multiselect',
            required: false,
            config: { options: ['a', 'b'] },
            created_at: '2026-03-29 10:00:00',
          },
        ],
      },
    })
    recordAPI.list.mockResolvedValueOnce({
      success: true,
      data: {
        records: [
          {
            id: 'rec_types',
            data: {
              title: 'Typed row',
              active: true,
              start_date: '2026-03-29 08:00:00',
              scheduled_at: '2026-03-29 08:30:00',
              notes: 'notes',
              status: 'todo',
              tags: ['a'],
            },
            version: 1,
            created_at: '2026-03-29 10:00:00',
            updated_at: '2026-03-29 10:00:00',
          },
        ],
        total: 1,
      },
    })

    const wrapper = mountView(RecordsView)
    await flushUi()

    const vm = getSetupState<any>(wrapper)
    vm.handleCreate()
    await flushUi()

    expect(vm.form.active).toBe(false)
    expect(vm.computedRules.title[0].message).toBe('请输入title')
    expect(document.body.textContent).toContain('是')
    expect(document.body.textContent).toContain('2026-03-29')

    await clickButtonByText('返回表列表')
    expect(pushMock).toHaveBeenCalledWith('/databases')
  })

  it('refreshes, searches, and debounces record loading with sanitized filters', async () => {
    vi.useFakeTimers()
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    recordAPI.list.mockClear()
    vm.searchText = '  keyword  '
    await flushUi()
    vi.advanceTimersByTime(300)
    await flushUi()

    expect(recordAPI.list).toHaveBeenLastCalledWith({
      table_id: 'tbl_1',
      limit: 20,
      offset: 0,
      filter: 'keyword',
    })

    vm.searchText = ''
    vm.currentPage = 3
    vm.handleRefresh()
    await flushUi()

    expect(vm.currentPage).toBe(1)
    expect(vm.searchText).toBe('')
    expect(recordAPI.list).toHaveBeenLastCalledWith({
      table_id: 'tbl_1',
      limit: 20,
      offset: 0,
      filter: '',
    })

    vi.useRealTimers()
  })

  it('reports record loading failures', async () => {
    recordAPI.list.mockRejectedValueOnce(new Error('list failed'))

    mountView(RecordsView)
    await flushUi()

    expect(messageError).toHaveBeenCalledWith('加载记录列表失败')
  })

  it('exports records as blob downloads and reports export failures', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)
    const anchorClick = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined)
    const appendChildSpy = vi.spyOn(document.body, 'appendChild')

    vm.tableName = 'Orders/2026'
    exportAPI.downloadRecords.mockResolvedValueOnce('{"ok":true}')
    createObjectURLMock.mockReturnValueOnce('blob:export-json')

    await vm.handleExport('json')

    expect(exportAPI.downloadRecords).toHaveBeenCalledWith('tbl_1', 'json', '')
    expect(createObjectURLMock).toHaveBeenCalled()
    expect(anchorClick).toHaveBeenCalledTimes(1)

    const appendedAnchor = appendChildSpy.mock.calls
      .map(([node]) => node)
      .find((node): node is HTMLAnchorElement => node instanceof HTMLAnchorElement)

    expect(appendedAnchor?.download).toMatch(/^Orders_2026_.*\.json$/)
    expect(messageSuccess).toHaveBeenCalledWith('导出成功')

    exportAPI.downloadRecords.mockRejectedValueOnce(new Error('导出服务异常'))
    await vm.handleExport('csv')
    expect(messageError).toHaveBeenCalledWith('导出服务异常')
  })

  it('creates and updates records through validated form submission', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    vm.formRef = { validate: vi.fn().mockResolvedValue(true), resetFields: vi.fn() }
    vm.form = { title: 'Created' }
    vm.isEditMode = false
    await vm.handleSubmit()

    expect(recordAPI.create).toHaveBeenCalledWith({
      table_id: 'tbl_1',
      data: { title: 'Created' },
    })
    expect(messageSuccess).toHaveBeenCalledWith('创建成功')

    vm.formRef = { validate: vi.fn().mockResolvedValue(true), resetFields: vi.fn() }
    vm.currentRecordId = 'rec_1'
    vm.form = { title: 'Updated' }
    vm.isEditMode = true
    await vm.handleSubmit()

    expect(recordAPI.update).toHaveBeenCalledWith('rec_1', {
      data: { title: 'Updated' },
    })
    expect(messageSuccess).toHaveBeenCalledWith('更新成功')
  })

  it('short-circuits invalid forms and reports create or update submission failures', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    vm.formRef = { validate: vi.fn().mockResolvedValue(false), resetFields: vi.fn() }
    await vm.handleSubmit()
    expect(recordAPI.create).not.toHaveBeenCalled()
    expect(recordAPI.update).not.toHaveBeenCalled()

    recordAPI.create.mockRejectedValueOnce(new Error('create failed'))
    vm.formRef = { validate: vi.fn().mockResolvedValue(true), resetFields: vi.fn() }
    vm.form = { title: 'Create failure' }
    vm.isEditMode = false
    await vm.handleSubmit()
    expect(messageError).toHaveBeenCalledWith('创建失败')

    recordAPI.update.mockRejectedValueOnce(new Error('update failed'))
    vm.formRef = { validate: vi.fn().mockResolvedValue(true), resetFields: vi.fn() }
    vm.currentRecordId = 'rec_1'
    vm.form = { title: 'Update failure' }
    vm.isEditMode = true
    await vm.handleSubmit()
    expect(messageError).toHaveBeenCalledWith('更新失败')
  })

  it('deletes records and preserves cancel and failure semantics', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    await vm.handleDelete(defaultRecord)
    expect(recordAPI.delete).toHaveBeenCalledWith('rec_1')
    expect(messageSuccess).toHaveBeenCalledWith('删除成功')

    confirmMock.mockRejectedValueOnce('cancel')
    await vm.handleDelete(defaultRecord)
    expect(messageError).not.toHaveBeenCalledWith('删除失败')

    confirmMock.mockResolvedValueOnce(true)
    recordAPI.delete.mockRejectedValueOnce(new Error('delete failed'))
    await vm.handleDelete(defaultRecord)
    expect(messageError).toHaveBeenCalledWith('删除失败')
  })

  it('handles upload warnings, progress, and upload failures with API messages', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    await vm.handleFileSelect({ raw: new File(['x'], 'report.txt') })
    expect(messageWarning).toHaveBeenCalledWith('请先保存记录后再上传文件')

    vm.currentRecordId = 'rec_1'
    defaultApi.post.mockImplementationOnce(
      async (
        _url: string,
        _formData: FormData,
        config: { onUploadProgress?: (progress: { loaded: number; total?: number }) => void },
      ) => {
        config.onUploadProgress?.({ loaded: 5, total: 10 })
        return {}
      },
    )

    await vm.handleFileSelect({ raw: new File(['x'], 'report.txt') })

    expect(defaultApi.post).toHaveBeenCalled()
    expect(messageSuccess).toHaveBeenCalledWith('文件上传成功')
    expect(vm.uploadProgress).toBe(0)
    expect(fileAPI.listByRecord).toHaveBeenCalledWith('rec_1')

    defaultApi.post.mockRejectedValueOnce({
      response: { data: { message: '上传被拒绝' } },
    })
    await vm.handleFileSelect({ raw: new File(['x'], 'report.txt') })
    expect(messageError).toHaveBeenCalledWith('上传被拒绝')
  })

  it('warns on upload exceed, deletes files, and handles delete-file failures', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)
    vm.currentRecordId = 'rec_1'

    vm.handleExceed()
    expect(messageWarning).toHaveBeenCalledWith('最多只能上传5个文件')

    await vm.handleDeleteFile({ id: 'file_1' })
    expect(fileAPI.delete).toHaveBeenCalledWith('file_1')
    expect(messageSuccess).toHaveBeenCalledWith('删除成功')

    confirmMock.mockRejectedValueOnce('cancel')
    await vm.handleDeleteFile({ id: 'file_1' })
    expect(messageError).not.toHaveBeenCalledWith('删除失败')

    confirmMock.mockResolvedValueOnce(true)
    fileAPI.delete.mockRejectedValueOnce(new Error('delete failed'))
    await vm.handleDeleteFile({ id: 'file_1' })
    expect(messageError).toHaveBeenCalledWith('删除失败')
  })

  it('shows unsupported previews, download failures, and clears preview URLs safely', async () => {
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    const archiveBlob = new Blob(['zip'], { type: 'application/zip' })
    fileAPI.downloadBlob.mockResolvedValueOnce(archiveBlob)
    createObjectURLMock.mockReturnValueOnce('blob:archive-preview')

    await vm.handlePreviewFile({ id: 'file_zip', file_name: 'archive.zip' })
    await flushUi()

    expect(document.body.textContent).toContain('无法预览')

    vm.previewFile = {
      id: 'file_preview',
      url: 'blob:old-preview',
      isImage: false,
      isPdf: false,
    }
    vm.clearPreviewFile()
    expect(revokeObjectURLMock).toHaveBeenCalledWith('blob:old-preview')

    fileAPI.downloadBlob.mockRejectedValueOnce({
      response: { data: { message: '下载失败：无权限' } },
    })
    await vm.handleDownloadFile({ id: 'file_denied', file_name: 'denied.txt' })
    expect(messageError).toHaveBeenCalledWith('下载失败：无权限')
  })

  it('logs field, table, and attachment loading failures without breaking the page', async () => {
    const errorSpy = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    tableAPI.getFields.mockRejectedValueOnce(new Error('fields failed'))
    tableAPI.get.mockRejectedValueOnce(new Error('table failed'))
    fileAPI.listByRecord.mockRejectedValueOnce(new Error('files failed'))

    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    await vm.loadAttachedFiles('rec_1')

    expect(errorSpy).toHaveBeenCalledWith('Failed to load fields:', expect.any(Error))
    expect(errorSpy).toHaveBeenCalledWith('Failed to load table info:', expect.any(Error))
    expect(errorSpy).toHaveBeenCalledWith('加载附件失败', expect.any(Error))

    errorSpy.mockRestore()
  })

  it('covers helper fallbacks, reset behavior, and empty-search debounce reloads', async () => {
    vi.useFakeTimers()
    const wrapper = mountView(RecordsView)
    await flushUi()
    const vm = getSetupState<any>(wrapper)

    expect(vm.formatDateTime('', 'date')).toBe('-')
    expect(vm.formatDateTime('2026-03-29 10:00:00', 'date')).toBe('2026-03-29')
    expect(vm.formatDateTime('2026-03-29 10:00:00', 'datetime')).toBe('2026-03-29 10:00:00')
    expect(vm.getFieldWidth('unknown')).toBe(150)
    expect(vm.getFieldOptions(undefined)).toEqual([])
    expect(vm.beforeUpload()).toBe(true)
    expect(vm.getErrorMessage(null, 'fallback')).toBe('fallback')

    const resetFields = vi.fn()
    vm.formRef = { validate: vi.fn().mockResolvedValue(true), resetFields }
    vm.fileList = [{ file_name: 'a.txt', file_size: 1, url: 'blob:a', isImage: false, isPdf: false }]
    vm.attachedFiles = [{ id: 'file_1', file_name: 'a.txt', file_size: 1, created_at: '2026-03-29 10:00:00' }]
    vm.resetForm()

    expect(resetFields).toHaveBeenCalled()
    expect(vm.fileList).toEqual([])
    expect(vm.attachedFiles).toEqual([])

    recordAPI.list.mockClear()
    vm.searchText = 'abc'
    await flushUi()
    vi.advanceTimersByTime(300)
    await flushUi()

    vm.searchText = ''
    await flushUi()
    vi.advanceTimersByTime(300)
    await flushUi()

    expect(recordAPI.list).toHaveBeenLastCalledWith({
      table_id: 'tbl_1',
      limit: 20,
      offset: 0,
      filter: '',
    })

    vi.useRealTimers()
  })
})
