import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from 'axios'
import type {
  ApiResponse,
  LoginResponse,
  RegisterResponse,
  UserProfile,
  UserListResponse,
  DatabaseListResponse,
  TableListResponse,
  FieldListResponse,
  RecordListResponse,
  OrganizationListResponse,
  FileListResponse,
  PluginListResponse,
  StatsSummary,
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
      config.headers = { ...config.headers, Authorization: `Bearer ${token}` }
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
  get<T = unknown>(url: string, params?: Record<string, unknown>): Promise<ApiResponse<T>> {
    return api.get(url, { params })
  },

  post<T = unknown>(url: string, data?: Record<string, unknown>): Promise<ApiResponse<T>> {
    return api.post(url, data)
  },

  put<T = unknown>(url: string, data?: Record<string, unknown>): Promise<ApiResponse<T>> {
    return api.put(url, data)
  },

  delete<T = unknown>(url: string, data?: Record<string, unknown>): Promise<ApiResponse<T>> {
    return data ? api.delete(url, { data }) : api.delete(url)
  },
}

// 认证相关 API
export const authAPI = {
  register: (data: { username: string; email: string; password: string }): Promise<ApiResponse<RegisterResponse>> =>
    request.post<RegisterResponse>('/auth/register', data),
  login: (data: { username: string; password: string }): Promise<ApiResponse<LoginResponse>> =>
    request.post<LoginResponse>('/auth/login', data),
  logout: (): Promise<ApiResponse<null>> => request.post<null>('/auth/logout'),
}

// 用户相关 API
export const userAPI = {
  getProfile: (): Promise<ApiResponse<UserProfile>> => request.get<UserProfile>('/users/me'),
  list: (params?: { org_id?: string; db_id?: string }): Promise<ApiResponse<UserListResponse>> =>
    request.get<UserListResponse>('/users', params),
  search: (query: string): Promise<ApiResponse<UserListResponse>> =>
    request.get<UserListResponse>('/users/search', { q: query }),
}

// 组织相关 API
export const organizationAPI = {
  list: (): Promise<ApiResponse<OrganizationListResponse>> =>
    request.get<OrganizationListResponse>('/organizations'),

  create: (data: { name: string; description?: string }): Promise<ApiResponse<Organization>> =>
    request.post<Organization>('/organizations', data),

  getDetail: (id: string): Promise<ApiResponse<Organization>> =>
    request.get<Organization>(`/organizations/${id}`),

  update: (
    id: string,
    data: { name: string; description?: string }
  ): Promise<ApiResponse<Organization>> =>
    request.put<Organization>(`/organizations/${id}`, data),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/organizations/${id}`),

  // 组织成员管理
  getMembers: (id: string): Promise<ApiResponse<Organization['members']>> =>
    request.get<Organization['members']>(`/organizations/${id}/members`),

  addMember: (
    orgId: string,
    data: { user_id: string; role: string }
  ): Promise<ApiResponse<null>> =>
    request.post<null>(`/organizations/${orgId}/members`, data),

  removeMember: (orgId: string, memberId: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/organizations/${orgId}/members/${memberId}`),

  updateMemberRole: (
    orgId: string,
    memberId: string,
    role: string
  ): Promise<ApiResponse<null>> =>
    request.put<null>(`/organizations/${orgId}/members/${memberId}/role`, { role }),
}

// 数据库相关 API
export const databaseAPI = {
  list: (): Promise<ApiResponse<DatabaseListResponse>> =>
    request.get<DatabaseListResponse>('/databases'),

  create: (data: {
    name: string
    description?: string
    isPublic?: boolean
    isPersonal?: boolean
  }): Promise<ApiResponse<Database>> =>
    request.post<Database>('/databases', data),

  getDetail: (id: string): Promise<ApiResponse<Database>> =>
    request.get<Database>(`/databases/${id}`),

  update: (
    id: string,
    data: { name?: string; description?: string }
  ): Promise<ApiResponse<Database>> =>
    request.put<Database>(`/databases/${id}`, data),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/databases/${id}`),

  // 数据库权限相关
  share: (
    id: string,
    data: { user_id: string; role: string }
  ): Promise<ApiResponse<null>> =>
    request.post<null>(`/databases/${id}/share`, data),

  getUsers: (id: string): Promise<ApiResponse<Database['users']>> =>
    request.get<Database['users']>(`/databases/${id}/users`),

  removeUser: (dbId: string, userId: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/databases/${dbId}/users/${userId}`),

  updateUserRole: (
    dbId: string,
    userId: string,
    role: string
  ): Promise<ApiResponse<null>> =>
    request.put<null>(`/databases/${dbId}/users/${userId}/role`, { role }),
}

// 表相关 API
export const tableAPI = {
  create: (data: { database_id: string; name: string }): Promise<ApiResponse<Table>> =>
    request.post<Table>('/tables', data),

  get: (id: string): Promise<ApiResponse<Table>> =>
    request.get<Table>(`/tables/${id}`),

  update: (
    id: string,
    data: { name: string }
  ): Promise<ApiResponse<Table>> =>
    request.put<Table>(`/tables/${id}`, data),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/tables/${id}`),
}

// 字段相关 API
export const fieldAPI = {
  create: (data: {
    table_id: string
    name: string
    type: string
    required?: boolean
    options?: string
  }): Promise<ApiResponse<Field>> =>
    request.post<Field>('/fields', data),

  get: (id: string): Promise<ApiResponse<Field>> =>
    request.get<Field>(`/fields/${id}`),

  update: (
    id: string,
    data: { name?: string; type?: string; required?: boolean; options?: string }
  ): Promise<ApiResponse<Field>> =>
    request.put<Field>(`/fields/${id}`, data),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/fields/${id}`),

  // 字段权限相关
  getPermissions: (tableId: string): Promise<ApiResponse<Field['permissions']>> =>
    request.get<Field['permissions']>(`/tables/${tableId}/field-permissions`),

  setPermission: (
    tableId: string,
    data: { field_id: string; role: string; can_read: boolean; can_write: boolean; can_delete: boolean }
  ): Promise<ApiResponse<null>> =>
    request.put<null>(`/tables/${tableId}/field-permissions`, data),

  batchSetPermissions: (
    tableId: string,
    data: { permissions: Array<{ field_id: string; role: string; can_read: boolean; can_write: boolean; can_delete: boolean }> }
  ): Promise<ApiResponse<null>> =>
    request.put<null>(`/tables/${tableId}/field-permissions/batch`, { permissions }),
}

// 记录相关 API
export const recordAPI = {
  create: (data: {
    table_id: string
    data: Record<string, unknown>
  }): Promise<ApiResponse<Record>> =>
    request.post<Record>('/records', data),

  list: (params: {
    table_id: string
    limit?: number
    offset?: number
    filter?: string
  }): Promise<ApiResponse<RecordListResponse>> =>
    request.get<RecordListResponse>('/records', {
      table_id: params.table_id,
      limit: params.limit,
      offset: params.offset,
      filter: params.filter,
    }),

  get: (id: string): Promise<ApiResponse<Record>> =>
    request.get<Record>(`/records/${id}`),

  update: (
    id: string,
    data: Record<string, unknown>,
    version?: number,
  ): Promise<ApiResponse<Record>> =>
    request.put<Record>(`/records/${id}`, { ...data, version }),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/records/${id}`),

  batchCreate: (data: {
    table_id: string
    data: Record<string, unknown>
    count: number
  }): Promise<ApiResponse<Record[]>> =>
    request.post<Record[]>('/records/batch', { ...data, count }),
}

// 文件相关 API
export const fileAPI = {
  upload: (recordId: string, file: File): Promise<ApiResponse<File>> => {
    const formData = new FormData()
    formData.append('record_id', recordId)
    formData.append('file', file)

    return request.post<File>('/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  get: (id: string): Promise<ApiResponse<File>> =>
    request.get<File>(`/files/${id}`),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/files/${id}`),

  listByRecord: (recordId: string): Promise<ApiResponse<File['files']>> =>
    request.get<File['files']>(`/records/${recordId}/files`),

  download: (id: string): string =>
    `${API_CONFIG.baseURL}/files/${id}/download`,
}

// 插件相关 API
export const pluginAPI = {
  create: (data: {
    name: string
    description?: string
    language: string
    entry_file: string
    timeout?: number
    config?: string
  }): Promise<ApiResponse<Plugin>> =>
    request.post<Plugin>('/plugins', data),

  list: (): Promise<ApiResponse<PluginListResponse>> =>
    request.get<PluginListResponse>('/plugins'),

  get: (id: string): Promise<ApiResponse<Plugin>> =>
    request.get<Plugin>(`/plugins/${id}`),

  update: (
    id: string,
    data: {
      name?: string
      description?: string
      timeout?: number
      config?: string
    }
  ): Promise<ApiResponse<Plugin>> =>
    request.put<Plugin>(`/plugins/${id}`, data),

  delete: (id: string): Promise<ApiResponse<null>> =>
    request.delete<null>(`/plugins/${id}`),

  bind: (id: string, data: { table_id: string; trigger: string }): Promise<ApiResponse<null>> =>
    request.post<null>(`/plugins/${id}/bind`, data),

  unbind: (id: string, data: { table_id: string }): Promise<ApiResponse<null>> =>
    request.delete<null>(`/plugins/${id}/unbind`, { data }),

  getBindings: (id: string): Promise<ApiResponse<Plugin['bindings']>> =>
    request.get<Plugin['bindings']>(`/plugins/${id}/bindings`),
}

// 统计相关 API
export const statsAPI = {
  getSummary: (): Promise<ApiResponse<StatsSummary>> =>
    request.get<StatsSummary>('/stats/summary'),

  getActivities: (limit?: number): Promise<ApiResponse<StatsSummary['activities']>> =>
    request.get<StatsSummary['activities']>('/stats/activities', { limit }),
}

export default api
