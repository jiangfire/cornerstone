package swagger

import "time"

// APIResponse is the standard envelope for all API responses.
type APIResponse struct {
	Code    int    `json:"code" example:"0"`
	Message string `json:"message" example:"success"`
	Data    any    `json:"data,omitempty"`
}

// ErrorResponse is returned when the request cannot be fulfilled.
type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"Validation error - invalid request body"`
}

// --- Database ---

// DatabaseCreateRequest body for POST /api/databases
type DatabaseCreateRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255" example:"My Database"`
	Description string `json:"description" binding:"max=500" example:"A test database"`
}

// DatabaseUpdateRequest body for PUT /api/databases/{id}
type DatabaseUpdateRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255" example:"Updated DB"`
	Description string `json:"description" binding:"max=500" example:"Updated description"`
}

// DatabaseObject represents a single database in responses.
type DatabaseObject struct {
	ID          string `json:"id" example:"db_abc123"`
	Name        string `json:"name" example:"My Database"`
	Description string `json:"description" example:"A test database"`
	CreatedAt   string `json:"created_at" example:"2026-01-01 00:00:00"`
	UpdatedAt   string `json:"updated_at" example:"2026-01-01 00:00:00"`
}

// DatabaseListResponse is the data payload for GET /api/databases.
type DatabaseListResponse struct {
	Databases []DatabaseObject `json:"databases"`
	Total     int              `json:"total" example:"3"`
}

// BulkCreateTableField is a nested field definition inside DatabaseBulkCreateRequest.
type BulkCreateTableField struct {
	Name        string `json:"name" binding:"required" example:"title"`
	Type        string `json:"type" binding:"required" example:"string"`
	Description string `json:"description" example:"The record title"`
	Required    bool   `json:"required" example:"false"`
}

// BulkCreateTable is a nested table definition inside DatabaseBulkCreateRequest.
type BulkCreateTable struct {
	Name        string                 `json:"name" binding:"required" example:"users"`
	Description string                 `json:"description" example:"User accounts"`
	Fields      []BulkCreateTableField `json:"fields"`
}

// DatabaseBulkCreateRequest body for POST /api/databases/with-tables
type DatabaseBulkCreateRequest struct {
	Name        string            `json:"name" binding:"required,min=2,max=255" example:"My App"`
	Description string            `json:"description" example:"App database"`
	Tables      []BulkCreateTable `json:"tables"`
}

// DatabaseBulkCreateResponse is the data payload for the bulk create endpoint.
type DatabaseBulkCreateResponse struct {
	Database DatabaseObject `json:"database"`
	Tables   []TableObject  `json:"tables"`
	Fields   []FieldObject  `json:"fields"`
	Summary  struct {
		TableCount int `json:"table_count" example:"2"`
		FieldCount int `json:"field_count" example:"5"`
	} `json:"summary"`
}

// --- Table ---

// TableCreateRequest body for POST /api/tables
type TableCreateRequest struct {
	DatabaseID  string `json:"database_id" binding:"required" example:"db_abc123"`
	Name        string `json:"name" binding:"required,min=2,max=255" example:"orders"`
	Description string `json:"description" binding:"max=500" example:"Order records"`
}

// TableUpdateRequest body for PUT /api/tables/{id}
type TableUpdateRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=255" example:"orders_v2"`
	Description string `json:"description" binding:"max=500" example:"Updated orders"`
}

// TableObject represents a single table in responses.
type TableObject struct {
	ID          string `json:"id" example:"tbl_xyz789"`
	DatabaseID  string `json:"database_id" example:"db_abc123"`
	Name        string `json:"name" example:"orders"`
	Description string `json:"description" example:"Order records"`
	CreatedAt   string `json:"created_at" example:"2026-01-01 00:00:00"`
	UpdatedAt   string `json:"updated_at" example:"2026-01-01 00:00:00"`
}

// TableListResponse is the data payload for GET /api/databases/{id}/tables.
type TableListResponse struct {
	Tables []TableObject `json:"tables"`
	Total  int           `json:"total" example:"5"`
}

// --- Field ---

// FieldConfig describes the configuration for list, number, file and other typed fields.
type FieldConfig struct {
	Options       []string `json:"options,omitempty" example:"option1,option2"`
	Required      bool     `json:"required,omitempty" example:"false"`
	Min           *float64 `json:"min,omitempty" example:"0"`
	Max           *float64 `json:"max,omitempty" example:"100"`
	Format        string   `json:"format,omitempty" example:"2006-01-02"`
	MaxLength     *int     `json:"max_length,omitempty" example:"255"`
	Validation    string   `json:"validation,omitempty" example:"^[a-z]+$"`
	AllowedTypes  []string `json:"allowed_types,omitempty" example:"image/*,.pdf"`
	MaxFileSizeMB int      `json:"max_file_size_mb,omitempty" example:"10"`
	Multiple      bool     `json:"multiple,omitempty" example:"false"`
}

// FieldCreateRequest body for POST /api/fields
type FieldCreateRequest struct {
	TableID     string      `json:"table_id" binding:"required" example:"tbl_xyz789"`
	Name        string      `json:"name" binding:"required,min=1,max=255" example:"status"`
	Type        string      `json:"type" binding:"required" example:"string"`
	Description string      `json:"description" binding:"max=1000" example:"Current status"`
	Required    bool        `json:"required" example:"true"`
	Options     string      `json:"options" example:"active,inactive"`
	Config      FieldConfig `json:"config"`
}

// FieldUpdateRequest body for PUT /api/fields/{id}
type FieldUpdateRequest struct {
	Name        string      `json:"name" binding:"required,min=1,max=255" example:"status"`
	Type        string      `json:"type" binding:"required" example:"string"`
	Description string      `json:"description" binding:"max=1000" example:"Current status"`
	Required    bool        `json:"required" example:"true"`
	Options     string      `json:"options" example:"active,inactive"`
	Config      FieldConfig `json:"config"`
}

// FieldObject represents a single field in responses.
type FieldObject struct {
	ID          string      `json:"id" example:"fld_def456"`
	TableID     string      `json:"table_id" example:"tbl_xyz789"`
	Name        string      `json:"name" example:"status"`
	Type        string      `json:"type" example:"string"`
	Description string      `json:"description" example:"Current status"`
	Required    bool        `json:"required" example:"true"`
	Options     string      `json:"options,omitempty" example:"active,inactive"`
	Config      FieldConfig `json:"config"`
	CreatedAt   string      `json:"created_at" example:"2026-01-01 00:00:00"`
	UpdatedAt   string      `json:"updated_at" example:"2026-01-01 00:00:00"`
}

// FieldListResponse is the data payload for GET /api/tables/{id}/fields.
type FieldListResponse struct {
	Items []FieldObject `json:"items"`
	Total int           `json:"total" example:"8"`
}

// --- Record ---

// RecordCreateRequest body for POST /api/records
type RecordCreateRequest struct {
	TableID string                 `json:"table_id" binding:"required" example:"tbl_xyz789"`
	Data    map[string]interface{} `json:"data" binding:"required"`
}

// RecordUpdateRequest body for PUT /api/records/{id}
type RecordUpdateRequest struct {
	Data    map[string]interface{} `json:"data" binding:"required"`
	Version int                    `json:"version" example:"3"`
}

// RecordObject represents a single record in responses.
type RecordObject struct {
	ID        string                 `json:"id" example:"rec_ghi012"`
	TableID   string                 `json:"table_id" example:"tbl_xyz789"`
	Data      map[string]interface{} `json:"data"`
	Version   int                    `json:"version" example:"1"`
	CreatedAt string                 `json:"created_at" example:"2026-01-01 00:00:00"`
	UpdatedAt string                 `json:"updated_at" example:"2026-01-01 00:00:00"`
}

// RecordListResponse is the data payload for GET /api/records.
type RecordListResponse struct {
	Items   []RecordObject `json:"items"`
	Total   int64          `json:"total" example:"42"`
	HasMore bool           `json:"has_more" example:"true"`
}

// RecordBatchCreateRequest body for POST /api/records/batch
type RecordBatchCreateRequest struct {
	TableID string                 `json:"table_id" binding:"required" example:"tbl_xyz789"`
	Data    map[string]interface{} `json:"data" binding:"required"`
}

// --- Token ---

// TokenCreateRequest body for POST /api/tokens
type TokenCreateRequest struct {
	Name      string     `json:"name" binding:"required,min=1,max=255" example:"my-app-token"`
	Scopes    string     `json:"scopes" example:"read,write"`
	ExpiresAt *time.Time `json:"expires_at" example:"2027-01-01T00:00:00Z"`
}

// TokenUpdateRequest body for PUT /api/tokens/{id}
type TokenUpdateRequest struct {
	Scopes    string     `json:"scopes" example:"read"`
	ExpiresAt *time.Time `json:"expires_at" example:"2027-06-01T00:00:00Z"`
}

// TokenObject represents a token in list/update responses (without the secret value).
type TokenObject struct {
	ID        string     `json:"id" example:"tok_jkl345"`
	Name      string     `json:"name" example:"my-app-token"`
	IsMaster  bool       `json:"is_master" example:"false"`
	Scopes    string     `json:"scopes" example:"read,write"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" example:"2027-01-01T00:00:00Z"`
	CreatedAt time.Time  `json:"created_at" example:"2026-01-01T00:00:00Z"`
}

// TokenListResponse is the data payload for GET /api/tokens.
type TokenListResponse struct {
	Tokens []TokenObject `json:"tokens"`
	Total  int           `json:"total" example:"2"`
}

// TokenCreateResponse is returned once after creating a token (includes the secret).
type TokenCreateResponse struct {
	ID        string     `json:"id" example:"tok_jkl345"`
	Name      string     `json:"name" example:"my-app-token"`
	Scopes    string     `json:"scopes" example:"read,write"`
	ExpiresAt *time.Time `json:"expires_at,omitempty" example:"2027-01-01T00:00:00Z"`
	CreatedAt time.Time  `json:"created_at" example:"2026-01-01T00:00:00Z"`
	Token     string     `json:"token" example:"cs_a1b2c3d4e5f6..."`
}

// --- File ---

// FileObject represents file metadata in responses.
type FileObject struct {
	ID         string `json:"id" example:"fil_mno678"`
	RecordID   string `json:"record_id" example:"rec_ghi012"`
	FieldID    string `json:"field_id" example:"fld_def456"`
	FileName   string `json:"file_name" example:"report.pdf"`
	FileSize   int64  `json:"file_size" example:"204800"`
	FileType   string `json:"file_type" example:"application/pdf"`
	StorageURL string `json:"storage_url" example:"./uploads/file_report.pdf"`
	CreatedAt  string `json:"created_at" example:"2026-01-01 00:00:00"`
	UpdatedAt  string `json:"updated_at" example:"2026-01-01 00:00:00"`
}

// FileListResponse is the data payload for GET /api/records/{id}/files.
type FileListResponse struct {
	Items []FileObject `json:"items"`
}

// --- Query DSL ---

// QueryDSLRequest body for POST /api/query
type QueryDSLRequest struct {
	From      string            `json:"from" example:"records"`
	Select    []string          `json:"select" example:"id,name,data"`
	Where     *WhereClause      `json:"where"`
	Having    *WhereClause      `json:"having"`
	Join      []JoinClause      `json:"join"`
	GroupBy   []string          `json:"groupBy" example:"table_id"`
	Aggregate []AggregateFunc   `json:"aggregate"`
	OrderBy   []OrderByClause   `json:"orderBy"`
	Page      int               `json:"page" example:"1"`
	Size      int               `json:"size" example:"20"`
	Union     []QueryDSLRequest `json:"union,omitempty"`
	Table     string            `json:"table" example:"records"`
	Filter    map[string]any    `json:"filter"`
	Sort      string            `json:"sort" example:"-created_at"`
}

// WhereClause represents a WHERE or HAVING condition tree.
type WhereClause struct {
	And []Condition `json:"and,omitempty"`
	Or  []Condition `json:"or,omitempty"`
}

// Condition is a single predicate inside a WhereClause.
type Condition struct {
	Field string      `json:"field" example:"name"`
	Op    string      `json:"op,omitempty" example:"eq"`
	Value interface{} `json:"value" example:"Alice"`
	Not   bool        `json:"not,omitempty"`
	And   []Condition `json:"and,omitempty"`
	Or    []Condition `json:"or,omitempty"`
}

// JoinClause describes a single JOIN in a query.
type JoinClause struct {
	Type   string        `json:"type" example:"left"`
	Table  string        `json:"table" example:"fields"`
	As     string        `json:"as,omitempty" example:"f"`
	On     JoinCondition `json:"on"`
	Select []string      `json:"select,omitempty" example:"name,type"`
}

// JoinCondition is the ON predicate for a JOIN.
type JoinCondition struct {
	Left  string `json:"left" example:"tables.id"`
	Op    string `json:"op" example:"="`
	Right string `json:"right" example:"f.table_id"`
}

// AggregateFunc describes an aggregate expression.
type AggregateFunc struct {
	Func  string `json:"func" example:"count"`
	Field string `json:"field,omitempty" example:"id"`
	As    string `json:"as" example:"total"`
}

// OrderByClause describes a sort column.
type OrderByClause struct {
	Field string `json:"field" example:"created_at"`
	Dir   string `json:"dir,omitempty" example:"desc"`
}

// QueryResult is returned from query execution.
type QueryResult struct {
	Data    []map[string]interface{} `json:"data"`
	Total   int64                    `json:"total" example:"100"`
	Page    int                      `json:"page" example:"1"`
	Size    int                      `json:"size" example:"20"`
	HasMore bool                     `json:"has_more" example:"true"`
}

// BatchQueryRequest body for POST /api/query/batch
type BatchQueryRequest struct {
	Queries map[string]QueryDSLRequest `json:"queries"`
}

// --- AI ---

// AIChatRequest body for POST /api/ai/chat
type AIChatRequest struct {
	Message string         `json:"message" binding:"required" example:"Show me all databases"`
	Context map[string]any `json:"context"`
}

// AIChatResponse is returned from the AI chat endpoint.
type AIChatResponse struct {
	Type    string         `json:"type" example:"result"`
	Reply   string         `json:"reply" example:"You have 3 databases: ..."`
	Context map[string]any `json:"context"`
}
