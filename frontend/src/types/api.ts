/**
 * API 响应类型定义
 */

// 认证相关
export interface LoginResponse {
  token: string
  user: User
}

export interface RegisterResponse {
  token: string
  user: User
}

export interface UserProfile {
  id: string
  username: string
  email: string
  role?: string
  phone?: string
  bio?: string
  avatar?: string
}

// 用户相关
export interface User {
  id: string
  username: string
  email: string
  role?: string
  phone?: string
  bio?: string
  avatar?: string
}

export interface UserListResponse {
  users: User[]
  total: number
  page: number
  limit: number
}

// 数据库相关
export interface Database {
  id: string
  name: string
  description?: string
  owner_id: string
  created_at: string
  updated_at: string
}

export interface DatabaseListResponse {
  databases: Database[]
  total: number
  page: number
  limit: number
}

// 表相关
export interface Table {
  id: string
  database_id: string
  name: string
  created_at: string
  updated_at: string
}

export interface TableListResponse {
  tables: Table[]
  total: number
  page: number
  limit: number
}

// 字段相关
export interface Field {
  id: string
  table_id: string
  name: string
  type: string
  required: boolean
  options: string
  created_at: string
  updated_at: string
}

export interface FieldListResponse {
  fields: Field[]
  total: number
  page: number
  limit: number
}

// 记录相关
export interface Record {
  id: string
  table_id: string
  data: unknown
  created_by: string
  updated_by: string
  version: number
  created_at: string
  updated_at: string
}

export interface RecordListResponse {
  records: Record[]
  total: number
  has_more: boolean
}

// 组织相关
export interface Organization {
  id: string
  name: string
  description?: string
  owner_id: string
  created_at: string
  updated_at: string
}

export interface OrganizationListResponse {
  organizations: Organization[]
  total: number
  page: number
  limit: number
}

// 文件相关
export interface File {
  id: string
  record_id: string
  file_name: string
  file_size: number
  file_type: string
  storage_url: string
  uploaded_by: string
  created_at: string
}

export interface FileListResponse {
  files: File[]
  total: number
  page: number
  limit: number
}

// 插件相关
export interface Plugin {
  id: string
  name: string
  description?: string
  owner_id: string
  created_at: string
  updated_at: string
}

export interface PluginListResponse {
  plugins: Plugin[]
  total: number
  page: number
  limit: number
}

// 统计相关
export interface StatsSummary {
  total_databases: number
  total_tables: number
  total_fields: number
  total_records: number
}

export interface Activity {
  id: string
  type: string
  description: string
  user_id: string
  created_at: string
}
