/**
 * API 响应类型定义
 */

// 通用API响应结构
export interface ApiResponse<T = unknown> {
  success: boolean
  message: string
  data: T
}

// 认证相关
export interface AuthResponse {
  success: boolean
  message?: string
  token?: string
}

export interface LoginResponse {
  success: boolean
  message?: string
  token?: string
  user?: User
}

export interface RegisterResponse {
  success: boolean
  message?: string
  token?: string
  user?: User
}

// 用户相关
export interface UserProfile {
  id: string
  username: string
  email: string
  role?: string
  phone?: string
  bio?: string
  avatar?: string
}

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
  success: boolean
  message?: string
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
  success: boolean
  message?: string
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
  success: boolean
  message?: string
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
  success: boolean
  message?: string
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
  success: boolean
  message?: string
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
  success: boolean
  message?: string
  organizations: Organization[]
  total: number
  page: number
  limit: number
}

export interface OrganizationMembers {
  success: boolean
  message?: string
  members: OrganizationMember[]
}

export interface OrganizationMember {
  id: string
  user_id: string
  organization_id: string
  role: string
  created_at: string
  updated_at: string
}

export interface OrganizationAdded {
  success: boolean
  message?: string
  member: OrganizationMember
}

export interface OrganizationRemoved {
  success: boolean
  message?: string
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
  updated_at: string
}

export interface FileListResponse {
  success: boolean
  message?: string
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
  success: boolean
  message?: string
  plugins: Plugin[]
  total: number
  page: number
  limit: number
}

export interface PluginBinding {
  table_id: string
  plugin_id: string
  created_at: string
}

export interface PluginBindings {
  success: boolean
  message?: string
  bindings: PluginBinding[]
}

// 统计相关
export interface StatsSummary {
  success: boolean
  message?: string
  total_databases: number
  total_tables: number
  total_fields: number
  total_records: number
}

export interface Activity {
  id: string
  type?: string
  content: string
  time: string
  created_at?: string
}

export interface ActivitiesResponse {
  success: boolean
  message?: string
  activities: Activity[]
}

// 数据库用户相关
export interface DatabaseUsers {
  success: boolean
  message?: string
  users: DatabaseUser[]
}

export interface DatabaseUser {
  id: string
  username: string
  email: string
  role: string
}

export interface DatabaseUserAdded {
  success: boolean
  message?: string
  user: DatabaseUser
}

export interface DatabaseUserRemoved {
  success: boolean
  message?: string
}

// 治理域相关
export interface GovernanceTask {
  id: string
  title: string
  description: string
  task_type: string
  status: string
  priority: string
  source_system?: string
  resource_type?: string
  resource_id?: string
  assignee_id?: string
  created_by: string
  due_at?: string
  completed_at?: string
  last_comment_at?: string
  created_at: string
  updated_at: string
}

export interface GovernanceReview {
  id: string
  task_id: string
  review_type: string
  status: string
  proposal_source?: string
  proposal_payload: string
  decision_payload?: string
  apply_status: string
  apply_error?: string
  apply_result?: string
  apply_target?: string
  reviewer_id: string
  created_by: string
  reviewed_at?: string
  applied_at?: string
  created_at: string
  updated_at: string
}

export interface GovernanceEvidence {
  id: string
  task_id: string
  evidence_type: string
  content: string
  file_id?: string
  created_by: string
  created_at: string
}

export interface GovernanceComment {
  id: string
  task_id: string
  content: string
  created_by: string
  created_at: string
}

export interface GovernanceExternalLink {
  id: string
  task_id: string
  source_system: string
  resource_type: string
  resource_id: string
  display_name?: string
  target_url?: string
  created_at: string
}

export interface GovernanceTaskDetail {
  task: GovernanceTask
  reviews: GovernanceReview[]
  evidences: GovernanceEvidence[]
  comments: GovernanceComment[]
  external_links: GovernanceExternalLink[]
}
