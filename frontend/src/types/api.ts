/**
 * API 响应类型定义
 *
 * 本文件是后端 JSON 响应的单一真相. `api.ts` 里的 `request.get<T>(...)`
 * 通过 `ApiResponse<T>` 包装,所以 T 只描述 `data:` 字段内部的形状.
 * 信封字段 (`code`/`message`) 不属于 data.
 */

// 通用 API 响应信封 (ApiResponse.data 类型由具体接口指定)
export interface ApiResponse<T = unknown> {
  code: number
  message: string
  data: T
}

// 认证相关
export interface LoginResponseData {
  token: string
}

// 用户相关
export interface UserProfile {
  id: string
  username: string
  email: string
  role?: string
  is_system_admin?: boolean
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
  users: User[]
}

// 数据库相关
export interface Database {
  id: string
  name: string
  description?: string
  owner_id: string
  is_public?: boolean
  is_personal?: boolean
  role?: string // 当前用户对该库的角色: owner/admin/editor/viewer
  created_at: string
  updated_at: string
}

export interface DatabaseListResponse {
  databases: Database[]
  total: number
}

export interface DatabaseDetail {
  id: string
  name: string
  description?: string
  owner_id: string
  is_public?: boolean
  is_personal?: boolean
  role?: string
  created_at: string
  updated_at: string
}

// 后端 `ListDatabaseUsers` 返回 `{user_id, username, email, role, joined_at}`,
// 无独立 `id` 字段(关系表 PK 是 user_id+database_id 复合,不暴露给前端).
export interface DatabaseUser {
  user_id: string
  username: string
  email: string
  role: string
  joined_at: string
}

export interface DatabaseUserListResponse {
  users: DatabaseUser[]
  total: number
}

// 表相关
export interface Table {
  id: string
  database_id: string
  name: string
  description?: string
  created_at: string
  updated_at: string
}

export interface TableListResponse {
  tables: Table[]
  total: number
}

export interface TableDetail {
  id?: string
  name: string
  database_id: string
  role?: string
}

// 字段相关
export interface Field {
  id: string
  table_id: string
  name: string
  type: string
  description?: string
  required: boolean
  options: string
  created_at: string
  updated_at: string
}

// 后端 `ListFields` 实际返回 `{items, total}`, 不是 `{fields, total}`.
export interface FieldListResponse {
  items: Field[]
  total: number
}

export interface FieldPermission {
  id?: string
  table_id?: string
  field_id: string
  role: string
  can_read: boolean
  can_write: boolean
  can_delete: boolean
}

export interface FieldPermissionListResponse {
  permissions: FieldPermission[]
}

// 记录相关
export interface RecordItem {
  id: string
  table_id: string
  data: Record<string, unknown>
  created_by: string
  updated_by: string
  version: number
  created_at: string
  updated_at: string
  _corrupted?: boolean
}

// 后端 `ListRecords` 实际返回 `{items, total, has_more}`, 不是 `{records, total}`.
export interface RecordListResponse {
  items: RecordItem[]
  total: number
  has_more: boolean
}

// 后端 `ListOrganizations` 返回 `OrgResponse`,含当前用户在该组织的 `role`.
export interface Organization {
  id: string
  name: string
  description?: string
  owner_id: string
  role: string // 当前用户在该组织的角色: owner/admin/editor/viewer
  created_at: string
  updated_at: string
}

export interface OrganizationListResponse {
  organizations: Organization[]
  total: number
  page: number
  page_size: number
}

// 后端 `ListMembers` 返回 `{id, organization_id, user_id, username, email, role, joined_at}`,
// 不附带 `created_at/updated_at`(来自 organization_members 表的 created_at 被映射成 joined_at).
export interface OrganizationMember {
  id: string
  organization_id: string
  user_id: string
  username: string
  email: string
  role: string
  joined_at: string
}

export interface OrganizationMemberListResponse {
  members: OrganizationMember[]
  total?: number
}

// 文件相关
export interface FileItem {
  id: string
  record_id: string
  field_id?: string
  file_name: string
  file_size: number
  file_type: string
  storage_url: string
  uploaded_by: string
  created_at: string
  updated_at: string
}

// 后端 `ListRecordFiles` 返回 `{items}`,无 total.
export interface FileListResponse {
  items: FileItem[]
}

// 后端 `models.Plugin` 的 `Description` 是 `string`(非指针,无 omitempty),
// 故 description 总会序列化出来.其它字段也按模型保持必选.
export interface Plugin {
  id: string
  name: string
  description: string
  language: string
  entry_file: string
  timeout: number
  config?: string
  config_values?: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface PluginListResponse {
  items: Plugin[]
  total: number
  page: number
  page_size: number
}

// 后端 `BindingDetail`(`ListBindings` 返回类型)在 `PluginBinding` 上 join 出 `table_name/database_name`,
// 并不包含 `updated_at`.前端目前只通过 `pluginAPI.getBindings` 拿到该结构.
export interface PluginBinding {
  id: string
  plugin_id?: string
  table_id: string
  table_name: string
  database_id: string
  database_name: string
  trigger: string
  created_at: string
}

export interface PluginExecution {
  id: string
  plugin_id: string
  table_id: string
  record_id?: string
  trigger: string
  status: string // running / success / failed / timeout
  output?: string
  error?: string
  duration_ms: number
  started_at: string
  finished_at?: string
  created_by: string
  created_at: string
}

// 统计相关 (后端 `services.StatsSummary`)
export interface StatsSummary {
  users: number
  organizations: number
  databases: number
  plugins: number
}

// 后端 `services.Activity` 直接返回 `{content, time, type}`,
// `time` 来自 `time.Time` 的 ISO 字符串, `type` 是活动标签 (primary/success/warning/danger/info).
export interface Activity {
  content: string
  time: string
  type: string
}

// 系统设置
export interface SystemSettings {
  system_name: string
  system_description: string
  allow_registration: boolean
  max_file_size: number
  db_type: string
  db_pool_size: number
  db_timeout: number
  plugin_timeout: number
  plugin_work_dir: string
  plugin_auto_update: boolean
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

export interface GovernanceTaskListResponse {
  tasks: GovernanceTask[]
  total: number
  page: number
  page_size: number
}

export interface GovernanceTaskDetail {
  task: GovernanceTask
  reviews: GovernanceReview[]
  evidences: GovernanceEvidence[]
  comments: GovernanceComment[]
  external_links: GovernanceExternalLink[]
}
