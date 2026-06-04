package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// JSONField 是跨数据库兼容的 JSON 字段类型。
// GORM AutoMigrate 会根据实际数据库选择对应类型：
// PostgreSQL → JSONB，MySQL → JSON，SQLite → TEXT
// 定义为 string 的派生类型，用法上与 string 基本相同（显式转换即可）。
type JSONField string

// GormDataType 返回通用数据类型标识。
func (JSONField) GormDataType() string {
	return "text"
}

// GormDBDataType 返回特定数据库的列类型。
func (JSONField) GormDBDataType(db *gorm.DB, field *schema.Field) string {
	switch db.Name() {
	case "postgres":
		return "jsonb"
	case "mysql":
		return "json"
	default:
		return "text"
	}
}

// Token API Token 表 (tok_前缀)
type Token struct {
	ID        string     `gorm:"type:varchar(50);primaryKey" json:"id"`
	Token     string     `gorm:"type:varchar(255);uniqueIndex;not null" json:"-"`
	Name      string     `gorm:"type:varchar(255);not null" json:"name"`
	IsMaster  bool       `gorm:"type:boolean;not null;default:false" json:"is_master"`
	Scopes    string     `gorm:"type:text" json:"scopes"`
	ExpiresAt *time.Time `gorm:"type:timestamp" json:"expires_at,omitempty"`
	CreatedAt time.Time  `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
}

func (Token) TableName() string {
	return "tokens"
}

func (t *Token) BeforeCreate(tx *gorm.DB) (err error) {
	if t.ID == "" {
		t.ID = GenerateID("tok")
	}
	if t.Token == "" {
		t.Token = "cs_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	}
	return nil
}

// Database 数据库表 (db_前缀)
type Database struct {
	ID          string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	Name        string         `gorm:"type:varchar(255);not null;uniqueIndex:idx_db_name" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
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

// Table 表定义 (tbl_前缀)
type Table struct {
	ID          string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	DatabaseID  string         `gorm:"type:varchar(50);not null;uniqueIndex:uk_table_db_name" json:"database_id"`
	Name        string         `gorm:"type:varchar(255);not null;uniqueIndex:uk_table_db_name" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
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
	ID          string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID     string         `gorm:"type:varchar(50);not null;uniqueIndex:uk_field_table_name" json:"table_id"`
	Name        string         `gorm:"type:varchar(255);not null;uniqueIndex:uk_field_table_name" json:"name"`
	Type        string         `gorm:"type:varchar(50);not null" json:"type"`
	Description string         `gorm:"type:text" json:"description"`
	Required    bool           `gorm:"type:boolean;default:false" json:"required"`
	Options     string         `gorm:"type:text" json:"options"`
	CreatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
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
	ID        string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID   string         `gorm:"type:varchar(50);not null" json:"table_id"`
	Data      JSONField      `gorm:"not null" json:"data"`
	Version   int            `gorm:"type:integer;default:1" json:"version"`
	CreatedAt time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
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
	ID         string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	RecordID   string         `gorm:"type:varchar(50);default:'';index" json:"record_id"`
	FieldID    string         `gorm:"type:varchar(50);default:'';index" json:"field_id"`
	FileName   string         `gorm:"type:varchar(255);not null" json:"file_name"`
	FileSize   int64          `gorm:"type:bigint;not null" json:"file_size"`
	FileType   string         `gorm:"type:varchar(100)" json:"file_type"`
	StorageURL string         `gorm:"type:text" json:"storage_url"`
	CreatedAt  time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt  time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
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

// GenerateID 生成带前缀的唯一ID
func GenerateID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
