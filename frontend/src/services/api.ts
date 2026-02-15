import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from 'axios'

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

// 通用请求方法
export const request = {
  get<T = unknown>(url: string, params?: Record<string, unknown>): Promise<T> {
    return api.get(url, { params })
  },

  post<T = unknown>(url: string, data?: Record<string, unknown>): Promise<T> {
    return api.post(url, data)
  },

  put<T = unknown>(url: string, data?: Record<string, unknown>): Promise<T> {
    return api.put(url, data)
  },

  delete<T = unknown>(url: string, data?: Record<string, unknown>): Promise<T> {
    return data ? api.delete(url, { data }) : api.delete(url)
  },
}

// 认证相关 API
export const authAPI = {
  register: (data: { username: string; email: string; password: string }) =>
    request.post('/auth/register', data),

  login: (data: { username: string; password: string }) => request.post('/auth/login', data),

  logout: () => request.post('/auth/logout'),
}

// 用户相关 API
export const userAPI = {
  getProfile: () => request.get('/users/me'),

  updateProfile: (data: Record<string, unknown>) => request.put('/users/me', data),

  // 获取用户列表（用于选择成员/共享用户）
  list: (params?: { org_id?: string; db_id?: string }) => request.get('/users', params),

  // 搜索用户
  search: (query: string) => request.get('/users/search', { q: query }),
}

// 组织相关 API
export const organizationAPI = {
  list: () => request.get('/organizations'),

  create: (data: { name: string; description?: string }) => request.post('/organizations', data),

  getDetail: (id: string) => request.get(`/organizations/${id}`),

  update: (id: string, data: Record<string, unknown>) => request.put(`/organizations/${id}`, data),

  delete: (id: string) => request.delete(`/organizations/${id}`),

  // 组织成员管理
  getMembers: (id: string) => request.get(`/organizations/${id}/members`),

  addMember: (id: string, data: { user_id: string; role: string }) =>
    request.post(`/organizations/${id}/members`, data),

  removeMember: (orgId: string, memberId: string) =>
    request.delete(`/organizations/${orgId}/members/${memberId}`),

  updateMemberRole: (orgId: string, memberId: string, role: string) =>
    request.put(`/organizations/${orgId}/members/${memberId}/role`, { role }),
}

// 数据库相关 API
export const databaseAPI = {
  list: () => request.get('/databases'),

  create: (data: {
    name: string
    description?: string
    isPublic?: boolean
    isPersonal?: boolean
  }) => request.post('/databases', data),

  getDetail: (id: string) => request.get(`/databases/${id}`),

  update: (id: string, data: { name: string; description?: string; isPublic?: boolean }) =>
    request.put(`/databases/${id}`, data),

  delete: (id: string) => request.delete(`/databases/${id}`),

  getTables: (dbId: string) => request.get(`/databases/${dbId}/tables`),

  share: (id: string, data: { user_id: string; role: string }) =>
    request.post(`/databases/${id}/share`, data),

  getUsers: (id: string) => request.get(`/databases/${id}/users`),

  removeUser: (dbId: string, userId: string) =>
    request.delete(`/databases/${dbId}/users/${userId}`),

  updateUserRole: (dbId: string, userId: string, role: string) =>
    request.put(`/databases/${dbId}/users/${userId}/role`, { role }),
}

// 表相关 API
export const tableAPI = {
  create: (data: { database_id: string; name: string; description?: string }) =>
    request.post('/tables', data),

  get: (id: string) => request.get(`/tables/${id}`),

  update: (id: string, data: { name: string; description?: string }) =>
    request.put(`/tables/${id}`, data),

  delete: (id: string) => request.delete(`/tables/${id}`),

  getFields: (tableId: string) => request.get(`/tables/${tableId}/fields`),
}

// 字段相关 API
export const fieldAPI = {
  create: (data: {
    table_id: string
    name: string
    type: string
    required?: boolean
    options?: string
  }) => request.post('/fields', data),

  get: (id: string) => request.get(`/fields/${id}`),

  update: (
    id: string,
    data: { name: string; type: string; required?: boolean; options?: string },
  ) => request.put(`/fields/${id}`, data),

  delete: (id: string) => request.delete(`/fields/${id}`),

  // 字段权限相关
  getPermissions: (tableId: string) => request.get(`/tables/${tableId}/field-permissions`),

  setPermission: (
    tableId: string,
    data: {
      field_id: string
      role: string
      can_read: boolean
      can_write: boolean
      can_delete: boolean
    },
  ) => request.put(`/tables/${tableId}/field-permissions`, data),

  batchSetPermissions: (
    tableId: string,
    permissions: Array<{
      field_id: string
      role: string
      can_read: boolean
      can_write: boolean
      can_delete: boolean
    }>,
  ) => request.put(`/tables/${tableId}/field-permissions/batch`, { permissions }),
}

// 记录相关 API
export const recordAPI = {
  create: (data: { table_id: string; data: Record<string, unknown> }) =>
    request.post('/records', data),

  list: (params: { table_id: string; limit?: number; offset?: number; filter?: string }) =>
    request.get('/records', {
      table_id: params.table_id,
      limit: params.limit,
      offset: params.offset,
      filter: params.filter,
    }),

  get: (id: string) => request.get(`/records/${id}`),

  update: (id: string, data: { data: Record<string, unknown>; version?: number }) =>
    request.put(`/records/${id}`, data),

  delete: (id: string) => request.delete(`/records/${id}`),

  batchCreate: (data: { table_id: string; records: Record<string, unknown>[] }) =>
    request.post('/records/batch', data),
}

// 文件相关 API
export const fileAPI = {
  upload: (recordId: string, file: File) => {
    const formData = new FormData()
    formData.append('record_id', recordId)
    formData.append('file', file)
    return api.post('/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  get: (id: string) => request.get(`/files/${id}`),

  download: (id: string) => `${API_CONFIG.baseURL}/files/${id}/download`,

  delete: (id: string) => request.delete(`/files/${id}`),

  listByRecord: (recordId: string) => request.get(`/records/${recordId}/files`),
}

// 插件相关 API
export const pluginAPI = {
  create: (data: {
    name: string
    description: string
    language: string
    entry_file: string
    timeout: number
    config?: string
    config_values?: string
  }) => request.post('/plugins', data),

  list: () => request.get('/plugins'),

  get: (id: string) => request.get(`/plugins/${id}`),

  update: (
    id: string,
    data: {
      name: string
      description: string
      timeout: number
      config?: string
      config_values?: string
    },
  ) => request.put(`/plugins/${id}`, data),

  delete: (id: string) => request.delete(`/plugins/${id}`),

  bind: (id: string, data: { table_id: string; trigger: string }) =>
    request.post(`/plugins/${id}/bind`, data),

  unbind: (id: string, data: { table_id: string }) => request.delete(`/plugins/${id}/unbind`, data),

  getBindings: (id: string) => request.get(`/plugins/${id}/bindings`),
}

// 统计相关 API
export const statsAPI = {
  getSummary: () => request.get('/stats/summary'),

  getActivities: (limit?: number) => request.get('/stats/activities', { limit }),
}

export default api
