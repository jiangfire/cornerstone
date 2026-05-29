import axios, { type AxiosInstance, type AxiosResponse, type AxiosError } from 'axios'

const API_CONFIG = {
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: 10000,
}

const LONG_TIMEOUT_MS = 60000

const api: AxiosInstance = axios.create(API_CONFIG)

export interface ApiResponse<T> {
  code: number
  message: string
  data: T
}

api.interceptors.request.use(
  (config) => {
    const apiKey = localStorage.getItem('api_key')
    if (apiKey) {
      config.headers['X-API-Key'] = apiKey
    }
    return config
  },
  (error) => Promise.reject(error),
)

api.interceptors.response.use(
  (response: AxiosResponse) => response.data,
  (error: AxiosError) => {
    if (error.response?.status === 401) {
      localStorage.removeItem('api_key')
      if (!window.location.pathname.startsWith('/tokens')) {
        window.location.href = '/tokens'
      }
    }
    return Promise.reject(error)
  },
)

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
  delete<T = unknown>(url: string): Promise<ApiResponse<T>> {
    return api.delete(url)
  },
}

export interface Token {
  id: string
  name: string
  is_master: boolean
  scopes: string
  expires_at?: string
  created_at: string
  token?: string
}

export interface Database {
  id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface Table {
  id: string
  database_id: string
  name: string
  description: string
  created_at: string
  updated_at: string
}

export interface Field {
  id: string
  table_id: string
  name: string
  type: string
  description: string
  required: boolean
  options: string
  created_at: string
  updated_at: string
}

export interface RecordItem {
  id: string
  table_id: string
  data: Record<string, unknown>
  version: number
  created_at: string
  updated_at: string
}

export interface RecordListResponse {
  items: RecordItem[]
  total: number
  has_more: boolean
}

export interface FileItem {
  id: string
  record_id: string
  field_id: string
  file_name: string
  file_size: number
  file_type: string
  storage_url: string
  created_at: string
}

export const tokenAPI = {
  list() {
    return request.get<{ tokens: Token[]; total: number }>('/tokens')
  },
  create(data: { name: string; scopes?: string; expires_at?: string }) {
    return request.post<Token>('/tokens', data)
  },
  update(id: string, data: { scopes?: string; expires_at?: string }) {
    return request.put<Token>(`/tokens/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/tokens/${id}`)
  },
}

export const databaseAPI = {
  list() {
    return request.get<{ databases: Database[]; total: number }>('/databases')
  },
  create(data: { name: string; description?: string }) {
    return request.post<Database>('/databases', data)
  },
  createWithTables(data: { name: string; description?: string; tables?: Array<{ name: string; description?: string; fields?: Array<{ name: string; type: string; description?: string; required?: boolean }> }> }) {
    return request.post<{ database: Database; tables: Table[]; fields: Field[]; summary: { table_count: number; field_count: number } }>('/databases/with-tables', data)
  },
  update(id: string, data: { name: string; description?: string }) {
    return request.put<Database>(`/databases/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/databases/${id}`)
  },
  getDetail(id: string) {
    return request.get<Database>(`/databases/${id}`)
  },
}

export const tableAPI = {
  list(databaseId: string) {
    return request.get<{ tables: Table[]; total: number }>(`/databases/${databaseId}/tables`)
  },
  create(data: { database_id: string; name: string; description?: string }) {
    return request.post<Table>('/tables', data)
  },
  get(id: string) {
    return request.get<Table>(`/tables/${id}`)
  },
  update(id: string, data: { name: string; description?: string }) {
    return request.put<Table>(`/tables/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/tables/${id}`)
  },
}

export const fieldAPI = {
  list(tableId: string) {
    return request.get<{ items: Field[]; total: number }>(`/tables/${tableId}/fields`)
  },
  create(data: { table_id: string; name: string; type: string; description?: string; required?: boolean }) {
    return request.post<Field>('/fields', data)
  },
  update(id: string, data: { name: string; type: string; description?: string; required?: boolean }) {
    return request.put<Field>(`/fields/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/fields/${id}`)
  },
}

export const recordAPI = {
  list(params: { table_id: string; limit?: number; offset?: number; filter?: string }) {
    return request.get<RecordListResponse>('/records', params)
  },
  create(data: { table_id: string; data: Record<string, unknown> }) {
    return request.post<RecordItem>('/records', data)
  },
  update(id: string, data: { data: Record<string, unknown>; version?: number }) {
    return request.put<RecordItem>(`/records/${id}`, data)
  },
  delete(id: string) {
    return request.delete(`/records/${id}`)
  },
  get(id: string) {
    return request.get<RecordItem>(`/records/${id}`)
  },
}

export const fileAPI = {
  upload(params: { recordId?: string; fieldId?: string; file: File }, onUploadProgress?: (progressEvent: { loaded: number; total?: number }) => void): Promise<ApiResponse<FileItem>> {
    const formData = new FormData()
    if (params.recordId) formData.append('record_id', params.recordId)
    if (params.fieldId) formData.append('field_id', params.fieldId)
    formData.append('file', params.file)
    return api.post('/files/upload', formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: LONG_TIMEOUT_MS,
      onUploadProgress,
    })
  },
  listByRecord(recordId: string) {
    return request.get<{ items: FileItem[] }>(`/records/${recordId}/files`)
  },
  download(fileId: string): string {
    return `${API_CONFIG.baseURL}/files/${fileId}/download`
  },
  delete(fileId: string) {
    return request.delete(`/files/${fileId}`)
  },
}

export const queryAPI = {
  execute(data: { from: string; select?: string[]; where?: Record<string, unknown>; limit?: number; offset?: number }) {
    return request.post<{ data: Record<string, unknown>[]; total: number; page: number; size: number; has_more: boolean }>('/query', data)
  },
}

export const aiAPI = {
  chat(message: string, context?: Record<string, unknown>) {
    return api.post('/ai/chat', { message, context }, { timeout: LONG_TIMEOUT_MS }) as Promise<ApiResponse<{ type?: string; reply: string; context?: Record<string, unknown> }>>
  },
}

export function setApiKey(key: string) {
  localStorage.setItem('api_key', key)
}

export function getApiKey(): string | null {
  return localStorage.getItem('api_key')
}

export function clearApiKey() {
  localStorage.removeItem('api_key')
}

export default api
