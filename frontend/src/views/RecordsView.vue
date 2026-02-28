<template>
  <div class="records">
    <el-card class="box-card">
      <template #header>
        <div class="card-header">
          <div class="header-left">
            <el-button link @click="goBack">
              <el-icon><ArrowLeft /></el-icon>
              返回表列表
            </el-button>
            <span class="table-name">{{ tableName }} - 数据记录</span>
          </div>
          <div class="header-right">
            <el-button @click="handleRefresh">
              <el-icon><Refresh /></el-icon>
              刷新
            </el-button>
            <el-dropdown @command="handleExport" :disabled="exporting">
              <el-button :loading="exporting">
                <el-icon><Download /></el-icon>
                导出
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="csv">导出 CSV</el-dropdown-item>
                  <el-dropdown-item command="json">导出 JSON</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button v-if="canCreate" type="primary" @click="handleCreate">新建记录</el-button>
          </div>
        </div>
      </template>

      <!-- 筛选和搜索 -->
      <div class="filter-bar">
        <el-input
          v-model="searchText"
          placeholder="搜索记录..."
          clearable
          style="width: 300px"
          @clear="loadRecords"
          @keyup.enter="handleSearch"
        >
          <template #append>
            <el-button :icon="Search" @click="handleSearch" />
          </template>
        </el-input>
      </div>

      <el-empty v-if="records.length === 0" description="暂无数据记录">
        <el-button v-if="canCreate" type="primary" @click="handleCreate">创建记录</el-button>
      </el-empty>

      <el-table v-else :data="records" style="width: 100%" v-loading="loading" border>
        <el-table-column
          v-for="field in fields"
          :key="field.id"
          :prop="field.name"
          :label="field.name"
          :min-width="getFieldWidth(field.type)"
        >
          <template #default="{ row }">
            <span v-if="field.type === 'boolean'">
              <el-tag :type="row.data[field.name] ? 'success' : 'info'">
                {{ row.data[field.name] ? '是' : '否' }}
              </el-tag>
            </span>
            <span v-else-if="field.type === 'date' || field.type === 'datetime'">
              {{ formatDateTime(row.data[field.name], field.type) }}
            </span>
            <span v-else>{{ row.data[field.name] || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column prop="created_at" label="创建时间" width="180" fixed="right">
          <template #default="{ row }">
            {{ formatDate(row.created_at) }}
          </template>
        </el-table-column>
        <el-table-column label="操作" width="150" fixed="right">
          <template #default="{ row }">
            <el-button v-if="canEdit" size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button v-if="canDelete" size="small" type="danger" @click="handleDelete(row)"
              >删除</el-button
            >
          </template>
        </el-table-column>
      </el-table>

      <!-- 分页 -->
      <div class="pagination" v-if="records.length > 0">
        <el-pagination
          v-model:current-page="currentPage"
          v-model:page-size="pageSize"
          :page-sizes="[10, 20, 50, 100]"
          :total="total"
          layout="total, sizes, prev, pager, next, jumper"
          @size-change="loadRecords"
          @current-change="loadRecords"
        />
      </div>
    </el-card>

    <!-- 创建/编辑对话框 -->
    <el-dialog v-model="dialogVisible" :title="dialogTitle" width="700px" @close="resetForm">
      <el-form
        ref="formRef"
        :model="form"
        :rules="computedRules"
        label-width="120px"
        :loading="submitting"
      >
        <el-form-item
          v-for="field in fields"
          :key="field.id"
          :label="field.name"
          :prop="field.name"
        >
          <!-- 字符串类型 -->
          <el-input
            v-if="field.type === 'string'"
            v-model="form[field.name]"
            :placeholder="`请输入${field.name}`"
          />

          <!-- 数字类型 -->
          <el-input-number
            v-else-if="field.type === 'number'"
            v-model="form[field.name]"
            :placeholder="`请输入${field.name}`"
            style="width: 100%"
          />

          <!-- 布尔类型 -->
          <el-switch v-else-if="field.type === 'boolean'" v-model="form[field.name]" />

          <!-- 日期类型 -->
          <el-date-picker
            v-else-if="field.type === 'date'"
            v-model="form[field.name]"
            type="date"
            :placeholder="`请选择${field.name}`"
            style="width: 100%"
            value-format="YYYY-MM-DD"
          />

          <!-- 日期时间类型 -->
          <el-date-picker
            v-else-if="field.type === 'datetime'"
            v-model="form[field.name]"
            type="datetime"
            :placeholder="`请选择${field.name}`"
            style="width: 100%"
            value-format="YYYY-MM-DD HH:mm:ss"
          />

          <!-- 文本类型 -->
          <el-input
            v-else-if="field.type === 'text'"
            v-model="form[field.name]"
            type="textarea"
            :rows="4"
            :placeholder="`请输入${field.name}`"
          />

          <!-- 下拉选择类型 -->
          <el-select
            v-else-if="field.type === 'select'"
            v-model="form[field.name]"
            :placeholder="`请选择${field.name}`"
            style="width: 100%"
          >
            <el-option
              v-for="option in getFieldOptions(field.options)"
              :key="option"
              :label="option"
              :value="option"
            />
          </el-select>

          <!-- 多选类型 -->
          <el-select
            v-else-if="field.type === 'multiselect'"
            v-model="form[field.name]"
            :placeholder="`请选择${field.name}`"
            style="width: 100%"
            multiple
          >
            <el-option
              v-for="option in getFieldOptions(field.options)"
              :key="option"
              :label="option"
              :value="option"
            />
          </el-select>
        </el-form-item>
      </el-form>

      <!-- 文件管理区域 -->
      <el-divider v-if="isEdit">附件管理</el-divider>
      <div v-if="isEdit" class="file-section">
        <el-upload
          :auto-upload="false"
          :on-change="handleFileSelect"
          :file-list="fileList"
          :limit="5"
          :on-exceed="handleExceed"
          :before-upload="beforeUpload"
        >
          <el-button size="small" type="primary">
            <el-icon><Upload /></el-icon> 选择文件
          </el-button>
          <template #tip>
            <div class="el-upload__tip">支持上传图片、文档、压缩包等文件，单文件不超过50MB</div>
          </template>
        </el-upload>

        <!-- 上传进度 -->
        <div v-if="uploadProgress > 0 && uploadProgress < 100" class="upload-progress">
          <el-progress :percentage="uploadProgress" />
        </div>

        <el-table
          :data="attachedFiles"
          style="width: 100%; margin-top: 20px"
          v-if="attachedFiles.length > 0"
          v-loading="loadingFiles"
        >
          <el-table-column label="文件" min-width="200">
            <template #default="{ row }">
              <div class="file-item">
                <el-icon class="file-icon"><Document /></el-icon>
                <span class="file-name">{{ row.file_name }}</span>
              </div>
            </template>
          </el-table-column>
          <el-table-column prop="file_size" label="大小" width="100">
            <template #default="{ row }">
              {{ formatFileSize(row.file_size) }}
            </template>
          </el-table-column>
          <el-table-column prop="created_at" label="上传时间" width="180">
            <template #default="{ row }">
              {{ formatDate(row.created_at) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="180">
            <template #default="{ row }">
              <el-button size="small" @click="handlePreviewFile(row)">预览</el-button>
              <el-button size="small" @click="handleDownloadFile(row)">下载</el-button>
              <el-button size="small" type="danger" @click="handleDeleteFile(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

      <template #footer>
        <span class="dialog-footer">
          <el-button @click="dialogVisible = false">取消</el-button>
          <el-button type="primary" @click="handleSubmit" :loading="submitting"> 确定 </el-button>
        </span>
      </template>
    </el-dialog>

    <!-- 文件预览对话框 -->
    <el-dialog v-model="previewDialogVisible" title="文件预览" width="800px">
      <div v-if="previewFile" class="file-preview">
        <img v-if="previewFile.isImage" :src="previewFile.url" style="max-width: 100%" />
        <div v-else-if="previewFile.isPdf">
          <iframe :src="previewFile.url" style="width: 100%; height: 600px"></iframe>
        </div>
        <div v-else>
          <el-result icon="warning" title="无法预览" sub-title="此文件类型不支持预览，请下载后查看">
            <template #extra>
              <el-button type="primary" @click="handleDownloadFile(previewFile)"
                >下载文件</el-button
              >
            </template>
          </el-result>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Refresh, Search, Upload, Document, Download } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tableAPI, recordAPI, fileAPI, databaseAPI, exportAPI } from '@/services/api'
import { formatDate, formatFileSize } from '@/utils/format'

interface Field {
  id: string
  name: string
  type: string
  required: boolean
  options?: string
  created_at: string
}

interface RecordData {
  id: string
  data: Record<string, unknown>
  version: number
  created_at: string
  updated_at: string
}

const route = useRoute()
const router = useRouter()
const tableId = route.params.id as string
const tableName = ref('')
const databaseId = ref('')
const userRole = ref('')

const loading = ref(false)
const submitting = ref(false)
const dialogVisible = ref(false)
const isEditMode = ref(false)
const records = ref<RecordData[]>([])
const fields = ref<Field[]>([])

const searchText = ref('')
const currentPage = ref(1)
const pageSize = ref(20)
const total = ref(0)
const exporting = ref(false)

const formRef = ref<FormInstance>()
const form = ref<Record<string, unknown>>({})
const currentRecordId = ref('')

const dialogTitle = ref('新建记录')

// 文件管理相关变量
const fileList = ref<
  Array<{ file_name: string; file_size: number; url: string; isImage: boolean; isPdf: boolean }>
>([])
const attachedFiles = ref<
  Array<{ id: string; file_name: string; file_size: number; created_at: string }>
>([])
const uploadProgress = ref(0)
const loadingFiles = ref(false)
const previewDialogVisible = ref(false)
const previewFile = ref<{ id: string; url: string; isImage: boolean; isPdf: boolean } | null>(null)
const isEdit = computed(() => isEditMode.value)

// 权限判断
const canCreate = computed(() => ['owner', 'admin', 'editor'].includes(userRole.value))
const canEdit = computed(() => ['owner', 'admin', 'editor'].includes(userRole.value))
const canDelete = computed(() => ['owner', 'admin'].includes(userRole.value))

// 动态生成表单验证规则
const computedRules = computed(() => {
  const rules: FormRules = {}
  fields.value.forEach((field) => {
    if (field.required) {
      rules[field.name] = [{ required: true, message: `请输入${field.name}`, trigger: 'blur' }]
    }
  })
  return rules
})

const goBack = () => {
  router.push(`/databases`)
}

const formatDateTime = (value: string, type: string) => {
  if (!value) return '-'
  if (type === 'date') {
    return value.split(' ')[0]
  }
  return value
}

const getFieldWidth = (type: string) => {
  const widthMap: Record<string, number> = {
    string: 150,
    number: 120,
    boolean: 100,
    date: 120,
    datetime: 180,
    text: 200,
    select: 150,
    multiselect: 200,
  }
  return widthMap[type] || 150
}

const getFieldOptions = (optionsStr: string | undefined) => {
  if (!optionsStr) return []
  return optionsStr.split(',').map((opt) => opt.trim())
}

const loadFields = async () => {
  try {
    const response = await tableAPI.getFields(tableId)
    if (response.success && response.data) {
      fields.value = response.data.fields || []
    }
  } catch (err) {
    console.error('Failed to load fields:', err)
  }
}

const loadTableInfo = async () => {
  try {
    const response = await tableAPI.get(tableId)
    if (response.success && response.data) {
      tableName.value = response.data.name || ''
      databaseId.value = response.data.database_id || ''

      // 获取数据库角色
      if (databaseId.value) {
        const dbResponse = await databaseAPI.getDetail(databaseId.value)
        if (dbResponse.success && dbResponse.data) {
          userRole.value = dbResponse.data.role || 'viewer'
        }
      }
    }
  } catch (error) {
    console.error('Failed to load table info:', error)
  }
}

const loadRecords = async () => {
  loading.value = true
  try {
    const params = {
      table_id: tableId,
      limit: pageSize.value,
      offset: (currentPage.value - 1) * pageSize.value,
      filter: searchText.value.trim(),
    }
    // Add filter parameter if searching
    if (searchText.value.trim()) {
      params.filter = searchText.value.trim()
    }

    const response = await recordAPI.list(params)
    if (response.success && response.data) {
      records.value = response.data.records || []
      total.value = response.data.total || 0
    }
  } catch {
    ElMessage.error('加载记录列表失败')
  } finally {
    loading.value = false
  }
}

const handleRefresh = () => {
  searchText.value = ''
  currentPage.value = 1
  loadRecords()
}

const handleSearch = async () => {
  currentPage.value = 1
  await loadRecords()
}

const handleExport = async (format: 'csv' | 'json') => {
  exporting.value = true
  try {
    const blobData = await exportAPI.downloadRecords(tableId, format, searchText.value)
    const blob =
      blobData instanceof Blob
        ? blobData
        : new Blob([blobData], { type: format === 'csv' ? 'text/csv' : 'application/json' })

    const link = document.createElement('a')
    const url = window.URL.createObjectURL(blob)
    link.href = url
    const safeTableName = (tableName.value || tableId).replace(/[\\/:*?"<>|]/g, '_')
    link.download = `${safeTableName}_${new Date().toISOString().replace(/[:.]/g, '-')}.${format}`
    document.body.appendChild(link)
    link.click()
    document.body.removeChild(link)
    window.URL.revokeObjectURL(url)

    ElMessage.success('导出成功')
  } catch (error: unknown) {
    ElMessage.error(error instanceof Error ? error.message : '导出失败')
  } finally {
    exporting.value = false
  }
}

const handleCreate = () => {
  isEditMode.value = false
  dialogTitle.value = '新建记录'
  currentRecordId.value = ''
  form.value = {}
  fields.value.forEach((field) => {
    form.value[field.name] = field.type === 'boolean' ? false : undefined
  })
  dialogVisible.value = true
}

const handleEdit = (row: RecordData) => {
  isEditMode.value = true
  dialogTitle.value = '编辑记录'
  currentRecordId.value = row.id
  form.value = { ...row.data }
  dialogVisible.value = true
  loadAttachedFiles(row.id)
}

const handleDelete = async (row: RecordData) => {
  try {
    await ElMessageBox.confirm('确定要删除这条记录吗？', '警告', {
      type: 'warning',
      confirmButtonText: '确定',
      cancelButtonText: '取消',
    })
    const response = await recordAPI.delete(row.id)
    if (response.success) {
      ElMessage.success('删除成功')
      await loadRecords()
    }
  } catch (error) {
    if (error !== 'cancel') {
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
      const response = await recordAPI.update(currentRecordId.value, {
        data: form.value,
      })
      if (response.success) {
        ElMessage.success('更新成功')
        dialogVisible.value = false
        await loadRecords()
      }
    } else {
      const response = await recordAPI.create({
        table_id: tableId,
        data: form.value,
      })
      if (response.success) {
        ElMessage.success('创建成功')
        dialogVisible.value = false
        await loadRecords()
      }
    }
  } catch {
    ElMessage.error(isEditMode.value ? '更新失败' : '创建失败')
  } finally {
    submitting.value = false
  }
}

const resetForm = () => {
  if (formRef.value) {
    formRef.value.resetFields()
  }
  fileList.value = []
  attachedFiles.value = []
}

// 加载记录的附件
const loadAttachedFiles = async (recordId: string) => {
  loadingFiles.value = true
  try {
    const response = await fileAPI.listByRecord(recordId)
    attachedFiles.value = response.data || []
  } catch (error) {
    console.error('加载附件失败', error)
  } finally {
    loadingFiles.value = false
  }
}

// 文件上传前校验
const beforeUpload = (file: { size: number }) => {
  const isLt50M = file.size / 1024 / 1024 < 50
  if (!isLt50M) {
    ElMessage.error('文件大小不能超过50MB')
    return false
  }
  return true
}

// 处理文件超出限制
const handleExceed = () => {
  ElMessage.warning('最多只能上传5个文件')
}

// 处理文件选择（带进度）
const handleFileSelect = async (file: { raw: File }) => {
  if (!currentRecordId.value) {
    ElMessage.warning('请先保存记录后再上传文件')
    return
  }

  uploadProgress.value = 0
  try {
    // Use axios directly to track upload progress
    const formData = new FormData()
    formData.append('record_id', currentRecordId.value)
    formData.append('file', file.raw)

    const apiInstance = (await import('@/services/api')).default
    await apiInstance.post('/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      onUploadProgress: (progressEvent: { loaded: number; total?: number }) => {
        uploadProgress.value = Math.floor((progressEvent.loaded * 100) / (progressEvent.total || 1))
      },
    })

    ElMessage.success('文件上传成功')
    loadAttachedFiles(currentRecordId.value)
    fileList.value = []
    uploadProgress.value = 0
  } catch {
    ElMessage.error('文件上传失败')
    uploadProgress.value = 0
  }
}

// 预览文件
const handlePreviewFile = (file: { id: string; file_name: string }) => {
  const extension = file.file_name.split('.').pop()?.toLowerCase()
  const imageExtensions = ['jpg', 'jpeg', 'png', 'gif', 'bmp', 'webp']

  previewFile.value = {
    id: file.id,
    url: fileAPI.download(file.id),
    isImage: imageExtensions.includes(extension || ''),
    isPdf: extension === 'pdf',
  }
  previewDialogVisible.value = true
}

// 下载文件
const handleDownloadFile = (file: { id: string }) => {
  const url = fileAPI.download(file.id)
  window.open(url, '_blank')
}

// 删除文件
const handleDeleteFile = async (file: { id: string }) => {
  try {
    await ElMessageBox.confirm('确定要删除该文件吗？', '提示', { type: 'warning' })
    await fileAPI.delete(file.id)
    ElMessage.success('删除成功')
    loadAttachedFiles(currentRecordId.value)
  } catch (err) {
    if (err !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 搜索防抖 - 当搜索文本改变时自动触发搜索
let searchTimeout: ReturnType<typeof setTimeout> | null = null
watch(searchText, () => {
  if (searchTimeout) {
    clearTimeout(searchTimeout)
  }
  searchTimeout = setTimeout(() => {
    if (searchText.value === '') {
      loadRecords()
    } else {
      handleSearch()
    }
  }, 300)
})

onMounted(() => {
  loadTableInfo()
  loadFields()
  loadRecords()
})
</script>

<style scoped lang="scss">
.records {
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

    .header-right {
      display: flex;
      gap: 8px;
    }
  }

  .filter-bar {
    margin-bottom: 16px;
  }

  .pagination {
    margin-top: 16px;
    display: flex;
    justify-content: flex-end;
  }
}
</style>
