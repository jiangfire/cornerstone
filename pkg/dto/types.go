package dto

import "time"

type DatabaseObject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type DatabaseListData struct {
	Databases []DatabaseObject `json:"databases"`
	Total     int              `json:"total"`
}

type TableObject struct {
	ID          string `json:"id"`
	DatabaseID  string `json:"database_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type TableListData struct {
	Tables []TableObject `json:"tables"`
	Total  int           `json:"total"`
}

type FieldObject struct {
	ID          string `json:"id"`
	TableID     string `json:"table_id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Options     string `json:"options"`
}

type FieldListData struct {
	Items []FieldObject `json:"items"`
	Total int           `json:"total"`
}

type TokenObject struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	IsMaster  bool       `json:"is_master"`
	Scopes    string     `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type TokenCreateData struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Scopes    string     `json:"scopes"`
	ExpiresAt *time.Time `json:"expires_at"`
	Token     string     `json:"token"`
}

type TokenListData struct {
	Tokens []TokenObject `json:"tokens"`
	Total  int           `json:"total"`
}

type TokenDeleteData struct {
	ID string `json:"id"`
}

type FileObject struct {
	ID         string `json:"id"`
	RecordID   string `json:"record_id"`
	FieldID    string `json:"field_id"`
	FileName   string `json:"file_name"`
	FileSize   int64  `json:"file_size"`
	FileType   string `json:"file_type"`
	StorageURL string `json:"storage_url"`
}

type FileListData struct {
	Items []FileObject `json:"items"`
}

type RecordObject struct {
	ID      string `json:"id"`
	TableID string `json:"table_id,omitempty"`
	Version int    `json:"version"`
	Data    any    `json:"data"`
}

type RecordBatchCreateData struct {
	Records []RecordObject `json:"records"`
	Count   int            `json:"count"`
}

type BulkCreateData struct {
	Database DatabaseObject `json:"database"`
	Tables   []TableObject  `json:"tables"`
	Fields   []FieldObject  `json:"fields"`
	Summary  struct {
		TableCount int `json:"table_count"`
		FieldCount int `json:"field_count"`
	} `json:"summary"`
}

type MessageData struct {
	Message string `json:"message"`
}

type AIChatData struct {
	Type    string         `json:"type"`
	Reply   string         `json:"reply"`
	Context map[string]any `json:"context"`
}

type QueryExplainData struct {
	SQL    string `json:"sql"`
	Params any    `json:"params"`
}

type QueryTablesData struct {
	Tables []string `json:"tables"`
}

type QuerySchemaData struct {
	Table  string   `json:"table"`
	Fields []string `json:"fields"`
}
