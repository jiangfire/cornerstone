<template>
  <div class="data-browser">
    <div class="browser-header">
      <h1>数据浏览</h1>
      <div class="actions">
        <button class="btn btn-primary" @click="showCreateDbModal = true">新建数据库</button>
        <button class="btn btn-secondary" @click="showBulkCreateModal = true">批量创建</button>
      </div>
    </div>

    <div class="browser-content">
      <div class="sidebar">
        <h3>数据库列表</h3>
        <ul class="db-list">
          <li v-for="db in databases" :key="db.id" :class="{ active: selectedDb?.id === db.id }" @click="selectDb(db)">
            <span class="db-name">{{ db.name }}</span>
            <span class="db-actions">
              <button class="btn-icon" @click.stop="editDb(db)" title="编辑">✏️</button>
              <button class="btn-icon danger" @click.stop="deleteDb(db)" title="删除">🗑️</button>
            </span>
          </li>
        </ul>
        <p v-if="databases.length === 0" class="empty-text">暂无数据库</p>
      </div>

      <div class="main-panel">
        <div v-if="selectedDb" class="db-detail">
          <div class="db-detail-header">
            <h2>{{ selectedDb.name }}</h2>
            <p>{{ selectedDb.description || '暂无描述' }}</p>
            <div class="actions">
              <button class="btn btn-secondary" @click="showCreateTableModal = true">新建表</button>
            </div>
          </div>

          <div class="tables-section">
            <h3>表列表</h3>
            <ul class="table-list">
              <li v-for="table in tables" :key="table.id" :class="{ active: selectedTable?.id === table.id }" @click="selectTable(table)">
                <span class="table-name">{{ table.name }}</span>
                <span class="table-desc">{{ table.description || '' }}</span>
              </li>
            </ul>
            <p v-if="tables.length === 0" class="empty-text">该数据库下暂无表</p>
          </div>

          <div v-if="selectedTable" class="table-detail">
            <h3>字段列表</h3>
            <table class="data-table" v-if="fields.length > 0">
              <thead>
                <tr>
                  <th>字段名</th>
                  <th>类型</th>
                  <th>必填</th>
                  <th>描述</th>
                  <th>操作</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="field in fields" :key="field.id">
                  <td>{{ field.name }}</td>
                  <td><code>{{ field.type }}</code></td>
                  <td>{{ field.required ? '是' : '否' }}</td>
                  <td>{{ field.description || '-' }}</td>
                  <td>
                    <button class="btn-icon danger" @click="deleteField(field)">🗑️</button>
                  </td>
                </tr>
              </tbody>
            </table>
            <p v-else class="empty-text">该表下暂无字段</p>

            <div class="records-section">
              <h3>记录列表</h3>
              <table class="data-table records-table" v-if="records.length > 0">
                <thead>
                  <tr>
                    <th>ID</th>
                    <th>数据预览</th>
                    <th>版本</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="record in records"
                    :key="record.id"
                    :class="{ active: selectedRecord?.id === record.id }"
                    @click="selectRecord(record)"
                  >
                    <td>{{ record.id }}</td>
                    <td>{{ summarizeRecord(record) }}</td>
                    <td>{{ record.version }}</td>
                  </tr>
                </tbody>
              </table>
              <p v-else class="empty-text">该表下暂无记录</p>

              <div v-if="selectedRecord" class="record-detail">
                <h4>记录详情</h4>
                <pre>{{ formatRecordData(selectedRecord.data) }}</pre>
              </div>
            </div>
          </div>
        </div>

        <div v-else class="placeholder">
          <p>请从左侧选择一个数据库</p>
        </div>
      </div>
    </div>

    <!-- Modals -->
    <div v-if="showCreateDbModal" class="modal-overlay" @click.self="showCreateDbModal = false">
      <div class="modal">
        <h3>新建数据库</h3>
        <input v-model="newDb.name" placeholder="数据库名称" class="input" />
        <textarea v-model="newDb.description" placeholder="描述（可选）" class="textarea"></textarea>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showCreateDbModal = false">取消</button>
          <button class="btn btn-primary" @click="createDb">创建</button>
        </div>
      </div>
    </div>

    <div v-if="showBulkCreateModal" class="modal-overlay" @click.self="showBulkCreateModal = false">
      <div class="modal large">
        <h3>批量创建（JSON）</h3>
        <textarea v-model="bulkJson" placeholder='{"name": "my_db", "tables": [{"name": "users", "fields": [{"name": "name", "type": "string"}]}]}' class="textarea code"></textarea>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showBulkCreateModal = false">取消</button>
          <button class="btn btn-primary" @click="bulkCreate">提交</button>
        </div>
      </div>
    </div>

    <div v-if="showCreateTableModal" class="modal-overlay" @click.self="showCreateTableModal = false">
      <div class="modal">
        <h3>新建表</h3>
        <input v-model="newTable.name" placeholder="表名称" class="input" />
        <textarea v-model="newTable.description" placeholder="描述（可选）" class="textarea"></textarea>
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showCreateTableModal = false">取消</button>
          <button class="btn btn-primary" @click="createTable">创建</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { databaseAPI, tableAPI, fieldAPI, recordAPI, type Database, type Table, type Field, type RecordItem } from '@/services/api'

const databases = ref<Database[]>([])
const tables = ref<Table[]>([])
const fields = ref<Field[]>([])
const records = ref<RecordItem[]>([])
const selectedDb = ref<Database | null>(null)
const selectedTable = ref<Table | null>(null)
const selectedRecord = ref<RecordItem | null>(null)

const showCreateDbModal = ref(false)
const showBulkCreateModal = ref(false)
const showCreateTableModal = ref(false)

const newDb = ref({ name: '', description: '' })
const newTable = ref({ name: '', description: '' })
const bulkJson = ref('')

onMounted(async () => {
  await loadDatabases()
})

async function loadDatabases() {
  try {
    const res = await databaseAPI.list()
    databases.value = res.data.databases || []
  } catch (e) {
    console.error('加载数据库失败', e)
  }
}

async function selectDb(db: Database) {
  selectedDb.value = db
  selectedTable.value = null
  fields.value = []
  records.value = []
  selectedRecord.value = null
  try {
    const res = await tableAPI.list(db.id)
    tables.value = res.data.tables || []
  } catch (e) {
    console.error('加载表失败', e)
  }
}

async function selectTable(table: Table) {
  selectedTable.value = table
  selectedRecord.value = null
  try {
    const res = await fieldAPI.list(table.id)
    fields.value = res.data.items || []
  } catch (e) {
    console.error('加载字段失败', e)
  }

  try {
    const res = await recordAPI.list({ table_id: table.id, limit: 50, offset: 0 })
    records.value = res.data.items || []
  } catch (e) {
    console.error('加载记录失败', e)
    records.value = []
  }
}

async function createDb() {
  if (!newDb.value.name) return
  try {
    await databaseAPI.create(newDb.value)
    showCreateDbModal.value = false
    newDb.value = { name: '', description: '' }
    await loadDatabases()
  } catch (e) {
    console.error('创建数据库失败', e)
  }
}

async function bulkCreate() {
  try {
    const data = JSON.parse(bulkJson.value)
    await databaseAPI.createWithTables(data)
    showBulkCreateModal.value = false
    bulkJson.value = ''
    await loadDatabases()
  } catch (e) {
    console.error('批量创建失败', e)
  }
}

async function createTable() {
  if (!newTable.value.name || !selectedDb.value) return
  try {
    await tableAPI.create({ ...newTable.value, database_id: selectedDb.value.id })
    showCreateTableModal.value = false
    newTable.value = { name: '', description: '' }
    await selectDb(selectedDb.value)
  } catch (e) {
    console.error('创建表失败', e)
  }
}

async function deleteDb(db: Database) {
  if (!confirm(`确定删除数据库 "${db.name}" 吗？`)) return
  try {
    await databaseAPI.delete(db.id)
    if (selectedDb.value?.id === db.id) {
      selectedDb.value = null
      tables.value = []
      fields.value = []
      records.value = []
      selectedRecord.value = null
    }
    await loadDatabases()
  } catch (e) {
    console.error('删除数据库失败', e)
  }
}

async function deleteField(field: Field) {
  if (!confirm(`确定删除字段 "${field.name}" 吗？`)) return
  try {
    await fieldAPI.delete(field.id)
    if (selectedTable.value) await selectTable(selectedTable.value)
  } catch (e) {
    console.error('删除字段失败', e)
  }
}

function editDb(db: Database) {
  console.log('Edit DB:', db.name)
  alert('编辑功能待实现')
}

function selectRecord(record: RecordItem) {
  selectedRecord.value = record
}

function summarizeRecord(record: RecordItem) {
  const entries = Object.entries(record.data || {})
  if (entries.length === 0) {
    return '-'
  }

  return entries
    .slice(0, 3)
    .map(([key, value]) => `${key}: ${String(value)}`)
    .join(' | ')
}

function formatRecordData(data: Record<string, unknown>) {
  return JSON.stringify(data, null, 2)
}
</script>

<style scoped>
.data-browser {
  padding: 20px;
  height: 100vh;
  display: flex;
  flex-direction: column;
}
.browser-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.browser-content {
  display: flex;
  gap: 20px;
  flex: 1;
  overflow: hidden;
}
.sidebar {
  width: 250px;
  border-right: 1px solid #ddd;
  padding-right: 15px;
  overflow-y: auto;
}
.main-panel {
  flex: 1;
  overflow-y: auto;
}
.db-list, .table-list {
  list-style: none;
  padding: 0;
  margin: 0;
}
.db-list li, .table-list li {
  padding: 8px 12px;
  cursor: pointer;
  border-radius: 4px;
  margin-bottom: 4px;
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.db-list li:hover, .table-list li:hover {
  background: #f5f5f5;
}
.db-list li.active, .table-list li.active {
  background: #e6f7ff;
}
.db-actions {
  display: flex;
  gap: 4px;
}
.btn-icon {
  background: none;
  border: none;
  cursor: pointer;
  padding: 2px 4px;
  font-size: 14px;
}
.btn-icon.danger:hover {
  color: red;
}
.placeholder {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 200px;
  color: #999;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
  margin-top: 10px;
}
.data-table th, .data-table td {
  border: 1px solid #ddd;
  padding: 8px;
  text-align: left;
}
.data-table th {
  background: #f9f9f9;
}
.records-section {
  margin-top: 20px;
}
.records-table tbody tr {
  cursor: pointer;
}
.records-table tbody tr.active {
  background: #e6f7ff;
}
.record-detail {
  margin-top: 16px;
}
.record-detail pre {
  background: #f9f9f9;
  border: 1px solid #ddd;
  border-radius: 4px;
  padding: 12px;
  white-space: pre-wrap;
  word-break: break-word;
}
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0,0,0,0.5);
  display: flex;
  align-items: center;
  justify-content: center;
}
.modal {
  background: white;
  padding: 20px;
  border-radius: 8px;
  width: 400px;
  max-width: 90vw;
}
.modal.large {
  width: 600px;
}
.input, .textarea {
  width: 100%;
  padding: 8px;
  margin: 8px 0;
  border: 1px solid #ddd;
  border-radius: 4px;
  box-sizing: border-box;
}
.textarea {
  min-height: 80px;
  resize: vertical;
}
.textarea.code {
  font-family: monospace;
  min-height: 150px;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}
.btn {
  padding: 6px 12px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}
.btn-primary {
  background: #1890ff;
  color: white;
}
.btn-secondary {
  background: #f0f0f0;
}
.empty-text {
  color: #999;
  font-style: italic;
}
</style>
