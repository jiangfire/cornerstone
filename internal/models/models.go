package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

// JSONField is a cross-database compatible JSON field type.
// GORM AutoMigrate selects the appropriate type based on the actual database:
// PostgreSQL → JSONB, MySQL → JSON, SQLite → TEXT
// Defined as a derived type of string, it behaves similarly to string (explicit conversion required).
type JSONField string

// GormDataType returns the generic data type identifier.
func (JSONField) GormDataType() string {
	return "text"
}

// GormDBDataType returns the column type for a specific database.
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

// Token API Token table (tok_ prefix)
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

// Database database table (db_ prefix)
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

// Table table definition (tbl_ prefix)
type Table struct {
	ID          string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	DatabaseID  string         `gorm:"type:varchar(50);not null;uniqueIndex:uk_table_db_name" json:"database_id"`
	Name        string         `gorm:"type:varchar(255);not null;uniqueIndex:uk_table_db_name" json:"name"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
	Database    Database       `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:DatabaseID" json:"-"`
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

// Field field definition (fld_ prefix)
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
	Table       Table          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:TableID" json:"-"`
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

// Record data record (rec_ prefix)
type Record struct {
	ID        string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID   string         `gorm:"type:varchar(50);not null" json:"table_id"`
	Data      JSONField      `gorm:"not null" json:"data"`
	Version   int            `gorm:"type:integer;default:1" json:"version"`
	CreatedAt time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
	Table     Table          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:TableID" json:"-"`
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

// RecordFieldIndex derived index table for record fields, used to optimize equality filtering on dynamic fields in MySQL.
type RecordFieldIndex struct {
	ID          string         `gorm:"type:varchar(50);primaryKey" json:"id"`
	TableID     string         `gorm:"type:varchar(50);not null" json:"table_id"`
	RecordID    string         `gorm:"type:varchar(50);not null" json:"record_id"`
	FieldID     string         `gorm:"type:varchar(50);not null" json:"field_id"`
	FieldName   string         `gorm:"type:varchar(255);not null" json:"field_name"`
	ValueType   string         `gorm:"type:varchar(20);not null" json:"value_type"`
	ValueText   string         `gorm:"type:varchar(512)" json:"value_text"`
	ValueNumber *float64       `gorm:"type:double precision" json:"value_number"`
	ValueBool   *bool          `gorm:"type:boolean" json:"value_bool"`
	CreatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"created_at"`
	UpdatedAt   time.Time      `gorm:"type:timestamp;default:CURRENT_TIMESTAMP" json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp;index" json:"deleted_at"`
	Record      Record         `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:RecordID" json:"-"`
	Field       Field          `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:FieldID" json:"-"`
}

func (RecordFieldIndex) TableName() string {
	return "record_field_indexes"
}

func (r *RecordFieldIndex) BeforeCreate(tx *gorm.DB) (err error) {
	if r.ID == "" {
		r.ID = GenerateID("rfi")
	}
	return nil
}

// File file attachment table
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

// GenerateID generates a unique ID with the given prefix
func GenerateID(prefix string) string {
	return prefix + "_" + strings.ReplaceAll(uuid.NewString(), "-", "")
}
