import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from 'axios'
import type { ApiResponse } from '@/types/api'

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

/* eslint-disable @typescript-eslint/no-explicit-any */

// 通用请求方法
export const request = {
  get<T = any>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    return api.get(url, { params })
  },

  post<T = any>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    return api.post(url, data)
  },

  put<T = any>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    return api.put(url, data)
  },

  delete<T = any>(url: string, data?: unknown): Promise<ApiResponse<T>> {
    return data ? api.delete(url, { data }) : api.delete(url)
  },
}

// 认证 API
export const authAPI = {
  login(data: { username: string; password: string }) {
    return request.post<{ token: string }>('/auth/login', data)
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
    return request.get<{
      id: string
      username: string
      email: string
      role?: string
      phone?: string
      bio?: string
      avatar?: string
    }>('/user/profile')
  },
  list(params?: Record<string, unknown>) {
    return request.get<{ users: Array<{ id: string; username: string; email: string }> }>(
      '/users',
      params,
    )
  },
}

// 数据库 API
export const databaseAPI = {
  list() {
    return request.get<{ databases: any[] }>('/databases')
  },
  create(data: Record<string, unknown>) {
    return request.post('/databases', data)
  },
  getDetail(id: string) {
    return request.get<{ name: string; role: string }>(`/databases/${id}`)
  },
  getTables(databaseId: string) {
    return request.get<{ tables: any[] }>(`/databases/${databaseId}/tables`)
  },
}

// 表 API
export const tableAPI = {
  get(id: string) {
    return request.get<{ name: string; database_id: string }>(`/tables/${id}`)
  },
  create(data: Record<string, unknown>) {
    return request.post('/tables', data)
  },
  delete(id: string) {
    return request.delete(`/tables/${id}`)
  },
  getFields(tableId: string) {
    return request.get<{ fields: any[] }>(`/tables/${tableId}/fields`)
  },
}

// 字段 API
export const fieldAPI = {
  create(data: Record<string, unknown>) {
    return request.post('/fields', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put(`/fields/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/fields/${id}`)
  },
  getPermissions(tableId: string) {
    return request.get<{ permissions: any[] }>(`/tables/${tableId}/field-permissions`)
  },
  setPermission(tableId: string, permission: unknown) {
    return request.post(`/tables/${tableId}/field-permissions`, permission)
  },
  batchSetPermissions(tableId: string, permissions: unknown[]) {
    return request.post(`/tables/${tableId}/field-permissions/batch`, { permissions })
  },
}

// 记录 API
export const recordAPI = {
  list(params: Record<string, unknown>) {
    return request.get<{ records: any[]; total: number }>('/records', params)
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
    return request.get<any[]>('/files', { record_id: recordId })
  },
  download(fileId: string): string {
    return `${API_CONFIG.baseURL}/files/${fileId}/download`
  },
  delete(fileId: string) {
    return request.delete(`/files/${fileId}`)
  },
}

// 组织 API
export const organizationAPI = {
  list() {
    return request.get<{ organizations: any[] }>('/organizations')
  },
  create(data: Record<string, unknown>) {
    return request.post('/organizations', data)
  },
  update(id: string, data: Record<string, unknown>) {
    return request.put(`/organizations/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/organizations/${id}`)
  },
  getMembers(orgId: string) {
    return request.get<{ members: any[] }>(`/organizations/${orgId}/members`)
  },
  addMember(orgId: string, data: unknown) {
    return request.post(`/organizations/${orgId}/members`, data)
  },
  updateMemberRole(orgId: string, memberId: string, role: string) {
    return request.put(`/organizations/${orgId}/members/${memberId}`, { role })
  },
  removeMember(orgId: string, memberId: string) {
    return request.delete(`/organizations/${orgId}/members/${memberId}`)
  },
}

// 插件 API
export const pluginAPI = {
  list() {
    return request.get<any[]>('/plugins')
  },
  create(data: Record<string, unknown>) {
    return request.post('/plugins', data)
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
    return request.get<any[]>(`/plugins/${pluginId}/bindings`)
  },
  unbind(pluginId: string, data: Record<string, unknown>) {
    return request.post(`/plugins/${pluginId}/unbind`, data)
  },
}

// 统计 API
export const statsAPI = {
  getSummary() {
    return request.get('/stats/summary')
  },
  getActivities(limit: number) {
    return request.get<any[]>('/stats/activities', { limit })
  },
}

/* eslint-enable @typescript-eslint/no-explicit-any */

// 默认导出 axios 实例（用于直接调用，如文件上传）
export default api
