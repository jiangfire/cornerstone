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
            <el-button type="primary" @click="handleCreate">新建记录</el-button>
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
        <el-button type="primary" @click="handleCreate">创建记录</el-button>
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
            <el-button size="small" @click="handleEdit(row)">编辑</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">删除</el-button>
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
    <el-dialog
      v-model="dialogVisible"
      :title="dialogTitle"
      width="700px"
      @close="resetForm"
    >
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
          <el-switch
            v-else-if="field.type === 'boolean'"
            v-model="form[field.name]"
          />

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
        >
          <el-button size="small" type="primary">选择文件</el-button>
          <template #tip>
            <div class="el-upload__tip">最多上传5个文件，单个文件不超过50MB</div>
          </template>
        </el-upload>

        <el-table :data="attachedFiles" style="width: 100%; margin-top: 20px" v-if="attachedFiles.length > 0">
          <el-table-column prop="file_name" label="文件名" />
          <el-table-column prop="file_size" label="大小" width="120">
            <template #default="{ row }">
              {{ formatFileSize(row.file_size) }}
            </template>
          </el-table-column>
          <el-table-column label="操作" width="150">
            <template #default="{ row }">
              <el-button size="small" @click="handleDownloadFile(row)">下载</el-button>
              <el-button size="small" type="danger" @click="handleDeleteFile(row)">删除</el-button>
            </template>
          </el-table-column>
        </el-table>
      </div>

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
import { ref, onMounted, computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ArrowLeft, Refresh, Search } from '@element-plus/icons-vue'
import type { FormInstance, FormRules } from 'element-plus'
import { ElMessage, ElMessageBox } from 'element-plus'
import { tableAPI, recordAPI, fieldAPI, fileAPI } from '@/services/api'

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
  data: Record<string, any>
  version: number
  created_at: string
  updated_at: string
}

const route = useRoute()
const router = useRouter()
const tableId = route.params.id as string
const tableName = ref('')

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

const formRef = ref<FormInstance>()
const form = ref<Record<string, any>>({})
const currentRecordId = ref('')

const dialogTitle = ref('新建记录')

// 文件管理相关变量
const fileList = ref<any[]>([])
const attachedFiles = ref<any[]>([])
const isEdit = computed(() => isEditMode.value)

// 动态生成表单验证规则
const computedRules = computed(() => {
  const rules: FormRules = {}
  fields.value.forEach(field => {
    if (field.required) {
      rules[field.name] = [
        { required: true, message: `请输入${field.name}`, trigger: 'blur' },
      ]
    }
  })
  return rules
})

const formatDate = (dateStr: string) => {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
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
  return optionsStr.split(',').map(opt => opt.trim())
}

const goBack = () => {
  router.push(`/databases`)
}

const loadFields = async () => {
  try {
    const response = await tableAPI.getFields(tableId)
    if (response.success && response.data) {
      fields.value = response.data.fields || []
    }
  } catch (error) {
    console.error('Failed to load fields:', error)
  }
}

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

const loadRecords = async () => {
  loading.value = true
  try {
    const response = await recordAPI.list({
      table_id: tableId,
      limit: pageSize.value,
      offset: (currentPage.value - 1) * pageSize.value,
    })
    if (response.success && response.data) {
      records.value = response.data.records || []
      total.value = response.data.total || 0
    }
  } catch (error) {
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

const handleSearch = () => {
  currentPage.value = 1
  // TODO: Implement search with filter parameter
  ElMessage.info('搜索功能开发中...')
}

const handleCreate = () => {
  isEditMode.value = false
  dialogTitle.value = '新建记录'
  currentRecordId.value = ''
  form.value = {}
  fields.value.forEach(field => {
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
    await ElMessageBox.confirm(
      '确定要删除这条记录吗？',
      '警告',
      {
        type: 'warning',
        confirmButtonText: '确定',
        cancelButtonText: '取消',
      }
    )
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
  } catch (error) {
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
  try {
    const res: any = await fileAPI.listByRecord(recordId)
    attachedFiles.value = res.data || []
  } catch (error) {
    console.error('加载附件失败', error)
  }
}

// 处理文件选择
const handleFileSelect = async (file: any) => {
  if (!currentRecordId.value) {
    ElMessage.warning('请先保存记录后再上传文件')
    return
  }

  try {
    await fileAPI.upload(currentRecordId.value, file.raw)
    ElMessage.success('文件上传成功')
    loadAttachedFiles(currentRecordId.value)
    fileList.value = []
  } catch (error: any) {
    ElMessage.error(error.response?.data?.message || '文件上传失败')
  }
}

// 下载文件
const handleDownloadFile = (file: any) => {
  const url = fileAPI.download(file.id)
  window.open(url, '_blank')
}

// 删除文件
const handleDeleteFile = async (file: any) => {
  try {
    await ElMessageBox.confirm('确定要删除该文件吗？', '提示', { type: 'warning' })
    await fileAPI.delete(file.id)
    ElMessage.success('删除成功')
    loadAttachedFiles(currentRecordId.value)
  } catch (error: any) {
    if (error !== 'cancel') {
      ElMessage.error('删除失败')
    }
  }
}

// 格式化文件大小
const formatFileSize = (bytes: number) => {
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(2) + ' KB'
  return (bytes / (1024 * 1024)).toFixed(2) + ' MB'
}

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
