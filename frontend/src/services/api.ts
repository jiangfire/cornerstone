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
  FileListResponse,
  Plugin,
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

// API 配置
const API_CONFIG = {
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 10000,
}

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
  list() {
    return request.get<OrganizationListResponse>('/organizations')
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

// 插件 API (后端直接返回数组,不包成 {items,total})
export const pluginAPI = {
  list() {
    return request.get<Plugin[]>('/plugins')
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
  downloadRecords(tableId: string, format: 'csv' | 'json' = 'csv', filter = '') {
    const params: Record<string, string> = {
      table_id: tableId,
      format,
    }
    if (filter.trim() !== '') {
      params.filter = filter.trim()
    }
    return api.get<Blob, Blob>('/records/export', { params, responseType: 'blob' })
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
}

// 默认导出 axios 实例（用于直接调用，如文件上传）
export default api
