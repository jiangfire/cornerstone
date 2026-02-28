package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User 用户表 (usr_前缀)
type User struct {
	ID        string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	Username  string     `gorm:"type:varchar(100);uniqueIndex;not null" json:"username"`
	Email     string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"email"`
	Phone     string     `gorm:"type:varchar(50)" json:"phone,omitempty"`
	Bio       string     `gorm:"type:text" json:"bio,omitempty"`
	Avatar    string     `gorm:"type:text" json:"avatar,omitempty"`
	Password  string     `gorm:"type:varchar(255);not null" json:"-"` // 密码哈希，不序列化
	CreatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

// TableName 表名前缀
func (User) TableName() string {
	return "users"
}

// BeforeCreate 创建前生成ID
func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == "" {
		u.ID = GenerateID("usr")
	}
	return nil
}

// Organization 组织表 (org_前缀)
type Organization struct {
	ID          string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	Name        string     `gorm:"type:varchar(255);not null;uniqueIndex:uk_org_owner_name" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	OwnerID     string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_org_owner_name" json:"owner_id"`
	CreatedAt   time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

func (Organization) TableName() string {
	return "organizations"
}

func (o *Organization) BeforeCreate(tx *gorm.DB) (err error) {
	if o.ID == "" {
		o.ID = GenerateID("org")
	}
	return nil
}

// OrganizationMember 组织成员表 (mem_前缀)
type OrganizationMember struct {
	ID             string    `gorm:"type:varchar(50);primaryKey" json:"id"`
	OrganizationID string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_org_user" json:"organization_id"`
	UserID         string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_org_user" json:"user_id"`
	Role           string    `gorm:"type:varchar(50);not null;default:'member'" json:"role"` // owner, admin, member
	JoinedAt       time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"joined_at"`
}

func (OrganizationMember) TableName() string {
	return "organization_members"
}

func (om *OrganizationMember) BeforeCreate(tx *gorm.DB) (err error) {
	if om.ID == "" {
		om.ID = GenerateID("mem")
	}
	return nil
}

// Database 数据库表 (db_前缀)
type Database struct {
	ID          string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	Name        string     `gorm:"type:varchar(255);not null;uniqueIndex:uk_db_owner_name" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	OwnerID     string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_db_owner_name" json:"owner_id"`
	IsPublic    bool       `gorm:"type:boolean;default:false" json:"is_public"`
	IsPersonal  bool       `gorm:"type:boolean;default:true" json:"is_personal"` // 个人数据库还是组织数据库
	CreatedAt   time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

func (Database) TableName() string {
	return "databases"
}

func (d *Database) BeforeCreate(tx *gorm.DB) (err error) {
	if d.ID == "" {
		d.ID = GenerateID("db")
	}
	return nil
}

// DatabaseAccess 数据库权限表 (acc_前缀)
type DatabaseAccess struct {
	ID         string    `gorm:"type:varchar(50);primaryKey" json:"id"`
	UserID     string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_db_user" json:"user_id"`
	DatabaseID string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_db_user" json:"database_id"`
	Role       string    `gorm:"type:varchar(50);not null" json:"role"` // owner, admin, editor, viewer
	CreatedAt  time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (DatabaseAccess) TableName() string {
	return "database_access"
}

func (da *DatabaseAccess) BeforeCreate(tx *gorm.DB) (err error) {
	if da.ID == "" {
		da.ID = GenerateID("acc")
	}
	return nil
}

// Table 表定义 (tbl_前缀)
type Table struct {
	ID          string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	DatabaseID  string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_table_db_name" json:"database_id"`
	Name        string     `gorm:"type:varchar(255);not null;uniqueIndex:uk_table_db_name" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	CreatedAt   time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

func (Table) TableName() string {
	return "tables"
}

func (t *Table) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = GenerateID("tbl")
	}
	return nil
}

// Field 字段定义 (fld_前缀)
type Field struct {
	ID        string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID   string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_field_table_name" json:"table_id"`
	Name      string     `gorm:"type:varchar(255);not null;uniqueIndex:uk_field_table_name" json:"name"`
	Type      string     `gorm:"type:varchar(50);not null" json:"type"` // string, number, boolean, date, etc.
	Required  bool       `gorm:"type:boolean;default:false" json:"required"`
	Options   string     `gorm:"type:text" json:"options"` // JSON string for dropdown options, validation rules
	CreatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

func (Field) TableName() string {
	return "fields"
}

func (f *Field) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = GenerateID("fld")
	}
	return nil
}

// Record 数据记录 (rec_前缀)
type Record struct {
	ID        string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID   string     `gorm:"type:varchar(50);not null" json:"table_id"`
	Data      string     `gorm:"type:jsonb;not null" json:"data"` // JSONB存储动态字段
	CreatedBy string     `gorm:"type:varchar(50);not null" json:"created_by"`
	UpdatedBy string     `gorm:"type:varchar(50)" json:"updated_by"`
	Version   int        `gorm:"type:integer;default:1" json:"version"` // 乐观锁版本号
	CreatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

func (Record) TableName() string {
	return "records"
}

func (r *Record) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = GenerateID("rec")
	}
	return nil
}

// File 文件附件表
type File struct {
	ID         string    `gorm:"type:varchar(50);primaryKey" json:"id"`
	RecordID   string    `gorm:"type:varchar(50);not null" json:"record_id"`
	FileName   string    `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize   int64     `gorm:"type:bigint;not null" json:"file_size"`
	FileType   string    `gorm:"type:varchar(100)" json:"file_type"`
	StorageURL string    `gorm:"type:text" json:"storage_url"`
	UploadedBy string    `gorm:"type:varchar(50);not null" json:"uploaded_by"`
	CreatedAt  time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (File) TableName() string {
	return "files"
}

func (f *File) BeforeCreate(tx *gorm.DB) (err error) {
	if f.ID == "" {
		f.ID = GenerateID("fil")
	}
	return nil
}

// Plugin 插件定义 (plg_前缀)
type Plugin struct {
	ID           string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	Name         string     `gorm:"type:varchar(255);not null;uniqueIndex:uk_plugin_creator_name" json:"name"`
	Description  string     `gorm:"type:text" json:"description"`
	Language     string     `gorm:"type:varchar(50);not null" json:"language"` // go, python, bash
	EntryFile    string     `gorm:"type:varchar(255);not null" json:"entry_file"`
	Timeout      int        `gorm:"type:integer;default:5" json:"timeout"` // 超时秒数
	Config       string     `gorm:"type:text" json:"config"`               // JSON config schema
	ConfigValues string     `gorm:"type:text" json:"config_values"`        // JSON config values
	CreatedBy    string     `gorm:"type:varchar(50);not null;uniqueIndex:uk_plugin_creator_name" json:"created_by"`
	CreatedAt    time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt    *time.Time `gorm:"type:timestamp" json:"deleted_at,omitempty"`
}

func (Plugin) TableName() string {
	return "plugins"
}

func (p *Plugin) BeforeCreate(tx *gorm.DB) (err error) {
	if p.ID == "" {
		p.ID = GenerateID("plg")
	}
	return nil
}

// PluginBinding 插件绑定表 (pbd_前缀)
type PluginBinding struct {
	ID        string    `gorm:"type:varchar(50);primaryKey" json:"id"`
	PluginID  string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_plugin_table_trigger" json:"plugin_id"`
	TableID   string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_plugin_table_trigger" json:"table_id"`
	Trigger   string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_plugin_table_trigger" json:"trigger"` // create, update, delete, manual
	CreatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (PluginBinding) TableName() string {
	return "plugin_bindings"
}

func (pb *PluginBinding) BeforeCreate(tx *gorm.DB) (err error) {
	if pb.ID == "" {
		pb.ID = GenerateID("pbd")
	}
	return nil
}

// PluginExecution 插件执行记录 (pex_前缀)
type PluginExecution struct {
	ID         string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	PluginID   string     `gorm:"type:varchar(50);not null;index" json:"plugin_id"`
	TableID    string     `gorm:"type:varchar(50);not null;index" json:"table_id"`
	RecordID   string     `gorm:"type:varchar(50);index" json:"record_id,omitempty"`
	Trigger    string     `gorm:"type:varchar(50);not null" json:"trigger"`
	Status     string     `gorm:"type:varchar(20);not null;index" json:"status"` // running, success, failed, timeout
	Output     string     `gorm:"type:text" json:"output,omitempty"`
	Error      string     `gorm:"type:text" json:"error,omitempty"`
	DurationMS int64      `gorm:"type:bigint;default:0" json:"duration_ms"`
	StartedAt  time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"started_at"`
	FinishedAt *time.Time `gorm:"type:timestamp" json:"finished_at,omitempty"`
	CreatedBy  string     `gorm:"type:varchar(50);not null;index" json:"created_by"`
	CreatedAt  time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (PluginExecution) TableName() string {
	return "plugin_executions"
}

func (pe *PluginExecution) BeforeCreate(tx *gorm.DB) (err error) {
	if pe.ID == "" {
		pe.ID = GenerateID("pex")
	}
	return nil
}

// TokenBlacklist JWT黑名单表
type TokenBlacklist struct {
	TokenHash string    `gorm:"type:varchar(64);primaryKey" json:"token_hash"`
	ExpiredAt time.Time `gorm:"type:timestamptz;not null" json:"expired_at"`
	CreatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (TokenBlacklist) TableName() string {
	return "token_blacklist"
}

// FieldPermission 字段级权限表 (flp_前缀)
type FieldPermission struct {
	ID        string    `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID   string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_table_field_role" json:"table_id"`
	FieldID   string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_table_field_role" json:"field_id"`
	Role      string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_table_field_role" json:"role"` // owner, admin, editor, viewer
	CanRead   bool      `gorm:"type:boolean;default:true" json:"can_read"`
	CanWrite  bool      `gorm:"type:boolean;default:false" json:"can_write"`
	CanDelete bool      `gorm:"type:boolean;default:false" json:"can_delete"`
	CreatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (FieldPermission) TableName() string {
	return "field_permissions"
}

func (fp *FieldPermission) BeforeCreate(tx *gorm.DB) (err error) {
	if fp.ID == "" {
		fp.ID = GenerateID("flp")
	}
	return nil
}

// ActivityLog 活动日志表 (act_前缀)
type ActivityLog struct {
	ID           string    `gorm:"type:varchar(50);primaryKey" json:"id"`
	UserID       string    `gorm:"type:varchar(50);not null" json:"user_id"`
	Action       string    `gorm:"type:varchar(100);not null" json:"action"` // create, update, delete, etc.
	ResourceType string    `gorm:"type:varchar(50)" json:"resource_type"`    // database, table, record, plugin
	ResourceID   string    `gorm:"type:varchar(50)" json:"resource_id"`
	Description  string    `gorm:"type:text" json:"description"`
	CreatedAt    time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (ActivityLog) TableName() string {
	return "activity_logs"
}

func (a *ActivityLog) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == "" {
		a.ID = GenerateID("act")
	}
	return nil
}

// AppSettings 系统设置（单例配置）
type AppSettings struct {
	ID                int       `gorm:"primaryKey;autoIncrement:false" json:"id"`
	SystemName        string    `gorm:"type:varchar(255);not null;default:'Cornerstone'" json:"system_name"`
	SystemDescription string    `gorm:"type:text" json:"system_description"`
	AllowRegistration bool      `gorm:"type:boolean;default:true" json:"allow_registration"`
	MaxFileSize       int       `gorm:"type:integer;default:50" json:"max_file_size"` // MB
	DBType            string    `gorm:"type:varchar(50);default:'postgresql'" json:"db_type"`
	DBPoolSize        int       `gorm:"type:integer;default:10" json:"db_pool_size"`
	DBTimeout         int       `gorm:"type:integer;default:30" json:"db_timeout"` // sec
	PluginTimeout     int       `gorm:"type:integer;default:300" json:"plugin_timeout"`
	PluginWorkDir     string    `gorm:"type:text;default:'./plugins'" json:"plugin_work_dir"`
	PluginAutoUpdate  bool      `gorm:"type:boolean;default:false" json:"plugin_auto_update"`
	UpdatedBy         string    `gorm:"type:varchar(50)" json:"updated_by,omitempty"`
	CreatedAt         time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt         time.Time `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
}

func (AppSettings) TableName() string {
	return "app_settings"
}

func (a *AppSettings) BeforeCreate(tx *gorm.DB) (err error) {
	if a.ID == 0 {
		a.ID = 1
	}
	return nil
}

// GenerateID 生成带前缀的唯一ID
func GenerateID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
