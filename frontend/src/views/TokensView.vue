<template>
  <div class="tokens-view">
    <div v-if="!hasApiKey" class="api-key-entry">
      <h3>输入 API Key</h3>
      <p class="hint">请输入 Master Token 或其他有效的 API Key 以访问系统。Master Token 会在服务启动时输出到日志。</p>
      <div class="key-input-row">
        <input
          v-model="inputKey"
          type="password"
          placeholder="cs_..."
          class="input key-input"
          @keydown.enter="applyKey"
        />
        <button class="btn btn-primary" @click="applyKey">确认</button>
      </div>
      <p v-if="keyError" class="error-text">{{ keyError }}</p>
    </div>

    <template v-else>
    <div class="header">
      <h1>令牌管理</h1>
      <div class="header-actions">
        <button class="btn btn-secondary" @click="clearKey">退出</button>
        <button class="btn btn-primary" @click="showCreateModal = true">新建令牌</button>
      </div>
    </div>

    <table class="data-table">
      <thead>
        <tr>
          <th>名称</th>
          <th>令牌值</th>
          <th>作用域</th>
          <th>过期时间</th>
          <th>创建时间</th>
          <th>操作</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="token in tokens" :key="token.id">
          <td>{{ token.name }}</td>
          <td>
            <code class="token-value">{{ token.is_master ? '*** MASTER ***' : '仅创建时展示一次' }}</code>
          </td>
          <td><code>{{ token.scopes || '*' }}</code></td>
          <td>{{ token.expires_at ? new Date(token.expires_at).toLocaleString() : '永久' }}</td>
          <td>{{ new Date(token.created_at).toLocaleString() }}</td>
          <td>
            <button class="btn-icon danger" @click="deleteToken(token)" title="删除">🗑️</button>
          </td>
        </tr>
      </tbody>
    </table>

    <p v-if="tokens.length === 0" class="empty-text">暂无令牌</p>

    <div v-if="showCreateModal" class="modal-overlay" @click.self="showCreateModal = false">
      <div class="modal">
        <h3>新建令牌</h3>
        <input v-model="newToken.name" placeholder="令牌名称" class="input" />
        <input v-model="newToken.scopes" placeholder="作用域（可选，逗号分隔）" class="input" />
        <input v-model="newToken.expires_at" type="datetime-local" class="input" />
        <div class="modal-actions">
          <button class="btn btn-secondary" @click="showCreateModal = false">取消</button>
          <button class="btn btn-primary" @click="createToken">创建</button>
        </div>
      </div>
    </div>

    <div v-if="createdTokenValue" class="modal-overlay" @click.self="createdTokenValue = ''">
      <div class="modal">
        <h3>令牌创建成功</h3>
        <p>请妥善保存此令牌，关闭后将无法再次查看：</p>
        <code class="token-display">{{ createdTokenValue }}</code>
        <div class="modal-actions">
          <button class="btn btn-primary" @click="copyValue(createdTokenValue); createdTokenValue = ''">复制并关闭</button>
        </div>
      </div>
    </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { tokenAPI, setApiKey, clearApiKey as clearStoredKey, getApiKey, type Token } from '@/services/api'

const tokens = ref<Token[]>([])
const showCreateModal = ref(false)
const createdTokenValue = ref('')
const newToken = ref({ name: '', scopes: '', expires_at: '' })
const hasApiKey = ref(!!getApiKey())
const inputKey = ref('')
const keyError = ref('')

onMounted(async () => {
  if (hasApiKey.value) {
    await loadTokens()
  }
})

async function applyKey() {
  if (!inputKey.value.trim()) return
  keyError.value = ''
  setApiKey(inputKey.value.trim())
  try {
    await tokenAPI.list()
    hasApiKey.value = true
    await loadTokens()
  } catch {
    clearStoredKey()
    keyError.value = 'API Key 无效或已过期，请检查后重试'
  }
}

function clearKey() {
  clearStoredKey()
  hasApiKey.value = false
  tokens.value = []
  inputKey.value = ''
}

async function loadTokens() {
  try {
    const res = await tokenAPI.list()
    tokens.value = res.data.tokens || []
  } catch (e) {
    console.error('加载令牌失败', e)
  }
}

async function createToken() {
  if (!newToken.value.name) return
  try {
    const res = await tokenAPI.create({
      name: newToken.value.name,
      scopes: newToken.value.scopes || undefined,
      expires_at: newToken.value.expires_at || undefined,
    })
    createdTokenValue.value = res.data.token || ''
    showCreateModal.value = false
    newToken.value = { name: '', scopes: '', expires_at: '' }
    await loadTokens()
  } catch (e) {
    console.error('创建令牌失败', e)
  }
}

async function deleteToken(token: Token) {
  if (token.is_master) {
    alert('不能删除 Master Token')
    return
  }
  if (!confirm(`确定删除令牌 "${token.name}" 吗？`)) return
  try {
    await tokenAPI.delete(token.id)
    await loadTokens()
  } catch (e) {
    console.error('删除令牌失败', e)
  }
}

function copyValue(value: string) {
  navigator.clipboard.writeText(value)
}
</script>

<style scoped>
.tokens-view {
  padding: 20px;
}
.api-key-entry {
  max-width: 480px;
  margin: 80px auto;
  text-align: center;
}
.api-key-entry h3 {
  margin-bottom: 8px;
}
.hint {
  color: #888;
  margin-bottom: 16px;
  font-size: 14px;
}
.key-input-row {
  display: flex;
  gap: 8px;
}
.key-input {
  flex: 1;
}
.error-text {
  color: #e53e3e;
  margin-top: 8px;
  font-size: 14px;
}
.header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;
}
.header-actions {
  display: flex;
  gap: 8px;
}
.data-table {
  width: 100%;
  border-collapse: collapse;
}
.data-table th, .data-table td {
  border: 1px solid #ddd;
  padding: 10px;
  text-align: left;
}
.data-table th {
  background: #f9f9f9;
}
.token-value {
  margin-right: 8px;
}
.btn-icon {
  background: none;
  border: none;
  cursor: pointer;
  font-size: 14px;
}
.btn-icon.danger:hover {
  color: red;
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
  text-align: center;
  margin-top: 40px;
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
}
.input {
  width: 100%;
  padding: 8px;
  margin: 8px 0;
  border: 1px solid #ddd;
  border-radius: 4px;
  box-sizing: border-box;
}
.modal-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 12px;
}
.token-display {
  display: block;
  background: #f5f5f5;
  padding: 12px;
  border-radius: 4px;
  word-break: break-all;
  margin: 10px 0;
}
</style>
