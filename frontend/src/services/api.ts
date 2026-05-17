import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from 'axios'
import type {
  ApiResponse,
  LoginResponseData,
  UserProfile,
  UserListResponse,
  Database,
  DatabaseListResponse,
  DatabaseDetail,
  DatabaseUserListResponse,
  Table,
  TableListResponse,
  TableDetail,
  Field,
  FieldListResponse,
  FieldPermissionListResponse,
  RecordListResponse,
  Organization,
  OrganizationListResponse,
  OrganizationMemberListResponse,
  FileItem,
  FileListResponse,
  Plugin,
  PluginListResponse,
  PluginBinding,
  PluginExecution,
  StatsSummary,
  Activity,
  SystemSettings,
  GovernanceTask,
  GovernanceTaskDetail,
  GovernanceTaskListResponse,
  GovernanceReview,
} from '@/types/api'

// API 配置：全局默认 10s 超时；耗时类调用（上传/导出/AI）在各自方法里覆盖为 60s。
const API_CONFIG = {
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 10000,
}

// 长耗时操作的统一超时（上传 / 导出 / AI 推理）
const LONG_TIMEOUT_MS = 60000

// 创建 Axios 实例
const api: AxiosInstance = axios.create(API_CONFIG)

// 请求拦截器
api.interceptors.request.use(
  (config) => {
    // 从 localStorage 获取 token
    const token = localStorage.getItem('auth_token')
    if (token) {
      config.headers.Authorization = `Bearer ${token}`
    }

    // 添加请求时间戳
    config.params = {
      ...config.params,
      _t: Date.now(),
    }

    return config
  },
  (error) => {
    return Promise.reject(error)
  },
)

// 响应拦截器
api.interceptors.response.use(
  (response: AxiosResponse) => {
    return response.data
  },
  (error: AxiosError) => {
    const { response } = error
    if (response?.status === 401) {
      // Token 过期或无效，清除本地存储并跳转到登录页
      localStorage.removeItem('auth_token')
      localStorage.removeItem('user_info')
      window.location.href = '/login'
    }

    return Promise.reject(error)
  },
)

// 通用请求方法 (T 描述 ApiResponse.data 的内部形状)
export const request = {
  get<T = unknown>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    return api.get(url, { params })
  },

  post<T = unknown>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    return api.post(url, data)
  },

  put<T = unknown>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    return api.put(url, data)
  },

  delete<T = unknown>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    return data ? api.delete(url, { data }) : api.delete(url)
  },
}

// 认证 API
export const authAPI = {
  login(data: { username: string; password: string }) {
    return request.post<LoginResponseData>('/auth/login', data)
  },
  register(data: { username: string; email: string; password: string }) {
    return request.post('/auth/register', data)
  },
  logout() {
    return request.post('/auth/logout')
  },
}

// 用户 API
export const userAPI = {
  getProfile() {
    return request.get<UserProfile>('/users/me')
  },
  updateProfile(data: {
    username: string
    email: string
    phone?: string
    bio?: string
    avatar?: string
  }) {
    return request.put('/users/me', data)
  },
  changePassword(data: { current_password: string; new_password: string }) {
    return request.put('/users/me/password', data)
  },
  deleteAccount(data: { password: string }) {
    return request.delete('/users/me', data)
  },
  list(params?: Record<string, unknown>) {
    return request.get<UserListResponse>('/users', params)
  },
}

// 头像 API
export const avatarAPI = {
  upload(file: File): Promise<ApiResponse<{ avatar_url: string }>> {
    const formData = new FormData()
    formData.append('file', file)
    return api.post('/users/me/avatar', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: LONG_TIMEOUT_MS,
    })
  },
}

// 头像公开访问地址（处理 dev 代理场景：baseURL 含 origin 时去掉 /api 后缀拼接）
export function avatarURL(path: string): string {
  if (!path) return ''
  if (path.startsWith('http') || path.startsWith('data:')) return path
  const base = API_CONFIG.baseURL.replace(/\/?api\/?$/, '')
  return (base || '') + path
}

// 数据库 API
export const databaseAPI = {
  list() {
    return request.get<DatabaseListResponse>('/databases')
  },
  create(data: Record<string, unknown>) {
    return request.post<Database>('/databases', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put<Database>(`/databases/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/databases/${id}`)
  },
  getDetail(id: string) {
    return request.get<DatabaseDetail>(`/databases/${id}`)
  },
  getTables(databaseId: string) {
    return request.get<TableListResponse>(`/databases/${databaseId}/tables`)
  },
  share(id: string, data: { user_id: string; role: 'admin' | 'editor' | 'viewer' }) {
    return request.post(`/databases/${id}/share`, data)
  },
  listUsers(id: string) {
    return request.get<DatabaseUserListResponse>(`/databases/${id}/users`)
  },
  updateUserRole(id: string, userId: string, role: 'admin' | 'editor' | 'viewer') {
    return request.put(`/databases/${id}/users/${userId}/role`, { role })
  },
  removeUser(id: string, userId: string) {
    return request.delete(`/databases/${id}/users/${userId}`)
  },
}

// 表 API
export const tableAPI = {
  get(id: string) {
    return request.get<TableDetail>(`/tables/${id}`)
  },
  create(data: Record<string, unknown>) {
    return request.post<Table>('/tables', data)
  },
  delete(id: string) {
    return request.delete(`/tables/${id}`)
  },
  getFields(tableId: string) {
    return request.get<FieldListResponse>(`/tables/${tableId}/fields`)
  },
}

// 字段 API
export const fieldAPI = {
  create(data: Record<string, unknown>) {
    return request.post<Field>('/fields', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put<Field>(`/fields/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/fields/${id}`)
  },
  getPermissions(tableId: string) {
    return request.get<FieldPermissionListResponse>(`/tables/${tableId}/field-permissions`)
  },
  setPermission(tableId: string, permission: unknown) {
    return request.put(`/tables/${tableId}/field-permissions`, permission)
  },
  batchSetPermissions(tableId: string, permissions: unknown[]) {
    return request.put(`/tables/${tableId}/field-permissions/batch`, { permissions })
  },
}

// 记录 API
export const recordAPI = {
  list(params: Record<string, unknown>) {
    return request.get<RecordListResponse>('/records', params)
  },
  create(data: Record<string, unknown>) {
    return request.post('/records', data)
  },
  update(id: string, data: unknown) {
    return request.put(`/records/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/records/${id}`)
  },
}

// 文件 API
export const fileAPI = {
  // 上传文件：60s 超时（覆盖全局 10s），可选 onUploadProgress 回调追踪进度
  upload(
    params: { recordId?: string; fieldId?: string; file: File },
    onUploadProgress?: (progressEvent: { loaded: number; total?: number }) => void,
  ): Promise<ApiResponse<FileItem>> {
    const formData = new FormData()
    if (params.recordId) {
      formData.append('record_id', params.recordId)
    }
    if (params.fieldId) {
      formData.append('field_id', params.fieldId)
    }
    formData.append('file', params.file)
    return api.post('/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: LONG_TIMEOUT_MS,
      onUploadProgress,
    })
  },
  listByRecord(recordId: string) {
    return request.get<FileListResponse>(`/records/${recordId}/files`)
  },
  download(fileId: string): string {
    return `${API_CONFIG.baseURL}/files/${fileId}/download`
  },
  downloadBlob(fileId: string) {
    return api.get<Blob, Blob>(`/files/${fileId}/download`, { responseType: 'blob' })
  },
  delete(fileId: string) {
    return request.delete(`/files/${fileId}`)
  },
}

// 组织 API
export const organizationAPI = {
  list(params?: { page?: number; page_size?: number }) {
    return request.get<OrganizationListResponse>('/organizations', params)
  },
  create(data: Record<string, unknown>) {
    return request.post<Organization>('/organizations', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put<Organization>(`/organizations/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/organizations/${id}`)
  },
  getMembers(orgId: string) {
    return request.get<OrganizationMemberListResponse>(`/organizations/${orgId}/members`)
  },
  addMember(orgId: string, data: unknown) {
    return request.post(`/organizations/${orgId}/members`, data)
  },
  updateMemberRole(orgId: string, memberId: string, role: string) {
    return request.put(`/organizations/${orgId}/members/${memberId}/role`, { role })
  },
  removeMember(orgId: string, memberId: string) {
    return request.delete(`/organizations/${orgId}/members/${memberId}`)
  },
}

// 插件 API
export const pluginAPI = {
  list(params?: { page?: number; page_size?: number }) {
    return request.get<PluginListResponse>('/plugins', params)
  },
  get(id: string) {
    return request.get<Plugin>(`/plugins/${id}`)
  },
  create(data: Record<string, unknown>) {
    return request.post<Plugin>('/plugins', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put(`/plugins/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/plugins/${id}`)
  },
  bind(pluginId: string, data: Record<string, unknown>) {
    return request.post(`/plugins/${pluginId}/bind`, data)
  },
  getBindings(pluginId: string) {
    return request.get<PluginBinding[]>(`/plugins/${pluginId}/bindings`)
  },
  unbind(pluginId: string, data: Record<string, unknown>) {
    return request.delete(`/plugins/${pluginId}/unbind`, data)
  },
  execute(pluginId: string, data: Record<string, unknown>) {
    return request.post<PluginExecution>(`/plugins/${pluginId}/execute`, data)
  },
  listExecutions(pluginId: string, limit = 50) {
    return request.get<PluginExecution[]>(`/plugins/${pluginId}/executions`, { limit })
  },
}

// 系统设置 API
export const settingsAPI = {
  get() {
    return request.get<SystemSettings>('/settings')
  },
  update(data: Record<string, unknown>) {
    return request.put('/settings', data)
  },
}

// 导出 API
export const exportAPI = {
  // 导出可能扫表/打包：60s 超时（覆盖全局 10s）
  downloadRecords(tableId: string, format: 'csv' | 'json' = 'csv', filter = '') {
    const params: Record<string, string> = {
      table_id: tableId,
      format,
    }
    if (filter.trim() !== '') {
      params.filter = filter.trim()
    }
    return api.get<Blob, Blob>('/records/export', {
      params,
      responseType: 'blob',
      timeout: LONG_TIMEOUT_MS,
    })
  },
}

// 统计 API
export const statsAPI = {
  getSummary() {
    return request.get<StatsSummary>('/stats/summary')
  },
  getActivities(limit: number) {
    return request.get<Activity[]>('/stats/activities', { limit })
  },
}

// 治理域 API
export const governanceAPI = {
  list(params?: Record<string, unknown>) {
    return request.get<GovernanceTaskListResponse>('/governance/tasks', params)
  },
  getDetail(id: string) {
    return request.get<GovernanceTaskDetail>(`/governance/tasks/${id}`)
  },
  create(data: Record<string, unknown>) {
    return request.post<GovernanceTask>('/governance/tasks', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put<GovernanceTask>(`/governance/tasks/${id}`, data)
  },
  addEvidence(taskId: string, data: Record<string, unknown>) {
    return request.post(`/governance/tasks/${taskId}/evidences`, data)
  },
  addComment(taskId: string, data: Record<string, unknown>) {
    return request.post(`/governance/tasks/${taskId}/comments`, data)
  },
  createReview(data: Record<string, unknown>) {
    return request.post<GovernanceReview>('/governance/reviews', data)
  },
  getReview(id: string) {
    return request.get<GovernanceReview>(`/governance/reviews/${id}`)
  },
  approveReview(id: string, data: { decision_payload: string }) {
    return request.post<GovernanceReview>(`/governance/reviews/${id}/approve`, data)
  },
  rejectReview(id: string, data: { decision_payload: string }) {
    return request.post<GovernanceReview>(`/governance/reviews/${id}/reject`, data)
  },
  applyReview(id: string) {
    return request.post(`/governance/reviews/${id}/apply`)
  },
  // AI 推荐生成：调用上游 LLM Governor，60s 超时（覆盖全局 10s）
  generateAIRecommendations(data: {
    task_id: string
    recommendation_type: 'term_binding' | 'classification' | 'dq_rule' | 'impact_summary'
    resource_type?: string
    resource_id?: string
    context?: Record<string, unknown>
  }): Promise<ApiResponse<GovernanceReview>> {
    return api.post('/governance/ai/recommendations', data, { timeout: LONG_TIMEOUT_MS })
  },
}

// 默认导出 axios 实例（用于直接调用，如文件上传）
export default api
