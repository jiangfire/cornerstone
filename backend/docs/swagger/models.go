package swagger

// ─── Common ───

type APIResponse struct {
	Code    int    `json:"code" example:"0"`
	Message string `json:"message" example:""`
	Data    any    `json:"data,omitempty"`
}

type ErrorResponse struct {
	Code    int    `json:"code" example:"400"`
	Message string `json:"message" example:"参数错误"`
}

// ─── Token ───

type TokenCreateRequest struct {
	Name      string  `json:"name" binding:"required" example:"my-client"`
	Scopes    string  `json:"scopes" example:""`
	ExpiresAt *string `json:"expires_at" example:"2027-01-01T00:00:00Z"`
}

type TokenObject struct {
	ID        string  `json:"id" example:"tok_abc123"`
	Name      string  `json:"name" example:"my-client"`
	IsMaster  bool    `json:"is_master" example:"false"`
	Scopes    string  `json:"scopes" example:""`
	ExpiresAt *string `json:"expires_at,omitempty"`
	CreatedAt string  `json:"created_at" example:"2026-05-26T14:00:00Z"`
}

type TokenCreateResponse struct {
	ID        string  `json:"id" example:"tok_abc123"`
	Name      string  `json:"name" example:"my-client"`
	IsMaster  bool    `json:"is_master" example:"false"`
	Scopes    string  `json:"scopes" example:""`
	ExpiresAt *string `json:"expires_at,omitempty"`
	CreatedAt string  `json:"created_at" example:"2026-05-26T14:00:00Z"`
	Token     string  `json:"token" example:"cs_xxxxxxxxxxxx"`
}

type TokenUpdateRequest struct {
	Scopes    string  `json:"scopes" example:""`
	ExpiresAt *string `json:"expires_at" example:"2027-01-01T00:00:00Z"`
}

type TokenListResponse struct {
	Tokens []TokenObject `json:"tokens"`
	Total  int           `json:"total" example:"3"`
}

// ─── Database ───

type DatabaseCreateRequest struct {
	Name        string `json:"name" binding:"required" example:"ecommerce"`
	Description string `json:"description" example:"E-commerce database"`
}

type DatabaseUpdateRequest struct {
	Name        string `json:"name" binding:"required" example:"ecommerce-v2"`
	Description string `json:"description" example:"Updated description"`
}

type DatabaseObject struct {
	ID          string `json:"id" example:"db_abc123"`
	Name        string `json:"name" example:"ecommerce"`
	Description string `json:"description" example:"E-commerce database"`
	CreatedAt   string `json:"created_at" example:"2026-05-26 14:00:00"`
	UpdatedAt   string `json:"updated_at" example:"2026-05-26 14:00:00"`
}

type DatabaseListResponse struct {
	Databases []DatabaseObject `json:"databases"`
	Total     int              `json:"total" example:"1"`
}

type BulkCreateFieldDef struct {
	Name        string `json:"name" binding:"required" example:"order_no"`
	Type        string `json:"type" binding:"required" example:"string"`
	Description string `json:"description" example:"Order number"`
	Required    bool   `json:"required" example:"true"`
}

type BulkCreateTableDef struct {
	Name        string              `json:"name" binding:"required" example:"orders"`
	Description string              `json:"description" example:"Order table"`
	Fields      []BulkCreateFieldDef `json:"fields"`
}

type DatabaseBulkCreateRequest struct {
	Name        string               `json:"name" binding:"required" example:"ecommerce"`
	Description string               `json:"description" example:"E-commerce database"`
	Tables      []BulkCreateTableDef `json:"tables"`
}

type DatabaseBulkCreateResponse struct {
	Database *DatabaseObject `json:"database"`
	Tables   []TableObject   `json:"tables"`
	Fields   []FieldObject   `json:"fields"`
	Summary  struct {
		TableCount int `json:"table_count" example:"2"`
		FieldCount int `json:"field_count" example:"5"`
	} `json:"summary"`
}

// ─── Table ───

type TableCreateRequest struct {
	DatabaseID  string `json:"database_id" binding:"required" example:"db_abc123"`
	Name        string `json:"name" binding:"required" example:"users"`
	Description string `json:"description" example:"User table"`
}

type TableUpdateRequest struct {
	Name        string `json:"name" binding:"required" example:"users"`
	Description string `json:"description" example:"Updated description"`
}

type TableObject struct {
	ID          string `json:"id" example:"tbl_abc123"`
	DatabaseID  string `json:"database_id" example:"db_abc123"`
	Name        string `json:"name" example:"users"`
	Description string `json:"description" example:"User table"`
	CreatedAt   string `json:"created_at" example:"2026-05-26 14:00:00"`
	UpdatedAt   string `json:"updated_at" example:"2026-05-26 14:00:00"`
}

type TableListResponse struct {
	Tables []TableObject `json:"tables"`
	Total  int           `json:"total" example:"3"`
}

// ─── Field ───

type FieldConfigSwagger struct {
	Options       []string `json:"options,omitempty" example:"red,green,blue"`
	Min           *float64 `json:"min,omitempty" example:"0"`
	Max           *float64 `json:"max,omitempty" example:"100"`
	Format        string   `json:"format,omitempty" example:"YYYY-MM-DD"`
	MaxLength     *int     `json:"max_length,omitempty" example:"255"`
	Validation    string   `json:"validation,omitempty" example:"^[a-z]+$"`
	AllowedTypes  []string `json:"allowed_types,omitempty" example:"image/*,.pdf"`
	MaxFileSizeMB int      `json:"max_file_size_mb,omitempty" example:"10"`
	Multiple      bool     `json:"multiple,omitempty" example:"false"`
}

type FieldCreateRequest struct {
	TableID     string              `json:"table_id" binding:"required" example:"tbl_abc123"`
	Name        string              `json:"name" binding:"required" example:"email"`
	Type        string              `json:"type" binding:"required" example:"string"`
	Description string              `json:"description" example:"User email address"`
	Required    bool                `json:"required" example:"true"`
	Options     string              `json:"options" example:"red,green,blue"`
	Config      FieldConfigSwagger  `json:"config"`
}

type FieldUpdateRequest struct {
	Name        string             `json:"name" binding:"required" example:"email"`
	Type        string             `json:"type" binding:"required" example:"string"`
	Description string             `json:"description" example:"Updated description"`
	Required    bool               `json:"required" example:"true"`
	Options     string             `json:"options" example:"red,green,blue"`
	Config      FieldConfigSwagger `json:"config"`
}

type FieldObject struct {
	ID          string    `json:"id" example:"fld_abc123"`
	TableID     string    `json:"table_id" example:"tbl_abc123"`
	Name        string    `json:"name" example:"email"`
	Type        string    `json:"type" example:"string"`
	Description string    `json:"description" example:"User email"`
	Required    bool      `json:"required" example:"true"`
	Options     string    `json:"options" example:""`
	Config      any       `json:"config"`
	CreatedAt   string    `json:"created_at" example:"2026-05-26 14:00:00"`
	UpdatedAt   string    `json:"updated_at" example:"2026-05-26 14:00:00"`
}

type FieldListResponse struct {
	Items []FieldObject `json:"items"`
	Total int           `json:"total" example:"5"`
}

// ─── Record ───

type RecordCreateRequest struct {
	TableID string                 `json:"table_id" binding:"required" example:"tbl_abc123"`
	Data    map[string]any         `json:"data"`
}

type RecordUpdateRequest struct {
	Data    map[string]any `json:"data"`
	Version int            `json:"version,omitempty" example:"1"`
}

type RecordObject struct {
	ID        string         `json:"id" example:"rec_abc123"`
	TableID   string         `json:"table_id" example:"tbl_abc123"`
	Data      map[string]any `json:"data"`
	Version   int            `json:"version" example:"1"`
	CreatedAt string         `json:"created_at" example:"2026-05-26 14:00:00"`
	UpdatedAt string         `json:"updated_at" example:"2026-05-26 14:00:00"`
}

type RecordListResponse struct {
	Records []RecordObject `json:"records"`
	Total   int            `json:"total" example:"100"`
	HasMore bool           `json:"has_more" example:"true"`
}

type RecordBatchCreateRequest struct {
	TableID string         `json:"table_id" binding:"required" example:"tbl_abc123"`
	Data    map[string]any `json:"data"`
	Count   int            `json:"count" example:"10"`
}

// ─── Query ───

type QueryDSLRequest struct {
	From       string         `json:"from" example:"users"`
	Select     []string       `json:"select,omitempty" example:"name,email"`
	Where      map[string]any `json:"where,omitempty"`
	OrderBy    string         `json:"order_by,omitempty" example:"name ASC"`
	Limit      int            `json:"limit,omitempty" example:"20"`
	Offset     int            `json:"offset,omitempty" example:"0"`
	GroupBy    []string       `json:"group_by,omitempty" example:"status"`
	Having     map[string]any `json:"having,omitempty"`
	Aggregates []string       `json:"aggregates,omitempty" example:"count:*,avg:age"`
	Join       map[string]any `json:"join,omitempty"`
	Union      map[string]any `json:"union,omitempty"`
}

type QueryResult struct {
	Data    []map[string]any `json:"data"`
	Total   int              `json:"total" example:"42"`
	Page    int              `json:"page" example:"1"`
	Size    int              `json:"size" example:"20"`
	HasMore bool             `json:"has_more" example:"false"`
}

type BatchQueryRequest struct {
	Queries []QueryDSLRequest `json:"queries"`
}

// ─── AI ───

type AIChatRequest struct {
	Message string         `json:"message" binding:"required" example:"创建一个用户表，包含姓名和邮箱"`
	Context map[string]any `json:"context,omitempty"`
}

type AIChatResponse struct {
	Type    string         `json:"type" example:"result"`
	Reply   string         `json:"reply" example:"已创建表 users，包含 2 个字段"`
	Context map[string]any `json:"context,omitempty"`
}

// ─── File ───

type FileObject struct {
	ID        string `json:"id" example:"file_abc123"`
	RecordID  string `json:"record_id" example:"rec_abc123"`
	FieldID   string `json:"field_id" example:"fld_abc123"`
	FileName  string `json:"file_name" example:"report.pdf"`
	FileSize  int64  `json:"file_size" example:"102400"`
	FileType  string `json:"file_type" example:"application/pdf"`
	StorageURL string `json:"storage_url" example:"/uploads/2026/05/report.pdf"`
	CreatedAt string `json:"created_at" example:"2026-05-26 14:00:00"`
}

type FileListResponse struct {
	Items []FileObject `json:"items"`
}
