package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/query"
	"gorm.io/gorm"
)

// ToolService encapsulates the business capabilities exposed by MCP.
type ToolService struct {
	db              *gorm.DB
	userID          string
	queryExecutor   *query.Executor
	databaseService *services.DatabaseService
	notifier        Notifier
}

// NewToolService creates an MCP tool service.
func NewToolService(database *gorm.DB, userID string) *ToolService {
	return NewToolServiceWithNotifier(database, userID, nil)
}

// NewToolServiceWithNotifier creates an MCP tool service with notification support.
func NewToolServiceWithNotifier(database *gorm.DB, userID string, notifier Notifier) *ToolService {
	return &ToolService{
		db:              database,
		userID:          userID,
		queryExecutor:   query.NewExecutor(database),
		databaseService: services.NewDatabaseService(database),
		notifier:        notifier,
	}
}

// --- Response shaping helpers ---

func shapeDatabase(d *models.Database) map[string]interface{} {
	return map[string]interface{}{
		"id":          d.ID,
		"name":        d.Name,
		"description": d.Description,
		"created_at":  d.CreatedAt.Format(time.RFC3339),
		"updated_at":  d.UpdatedAt.Format(time.RFC3339),
	}
}

func shapeTable(t *models.Table) map[string]interface{} {
	return map[string]interface{}{
		"id":          t.ID,
		"database_id": t.DatabaseID,
		"name":        t.Name,
		"description": t.Description,
		"created_at":  t.CreatedAt.Format(time.RFC3339),
		"updated_at":  t.UpdatedAt.Format(time.RFC3339),
	}
}

func shapeField(f *models.Field) map[string]interface{} {
	return map[string]interface{}{
		"id":          f.ID,
		"table_id":    f.TableID,
		"name":        f.Name,
		"type":        f.Type,
		"description": f.Description,
		"required":    f.Required,
		"created_at":  f.CreatedAt.Format(time.RFC3339),
		"updated_at":  f.UpdatedAt.Format(time.RFC3339),
	}
}

func shapeRecord(r *models.Record) map[string]interface{} {
	return map[string]interface{}{
		"id":         r.ID,
		"table_id":   r.TableID,
		"data":       json.RawMessage(r.Data),
		"version":    r.Version,
		"created_at": r.CreatedAt.Format(time.RFC3339),
		"updated_at": r.UpdatedAt.Format(time.RFC3339),
	}
}

// --- Tool definitions ---

const (
	fieldTypeDescription = "Field data type. Determines how values are stored and validated. Supported types: string (short text), text (long text), number (numeric), boolean (true/false), date (YYYY-MM-DD), datetime (ISO 8601), file (attachment reference), json (nested object/array), list (array of strings)."
	allowedDSLTables     = `["records", "tables", "databases", "fields", "files", "tokens"]`
	fieldTypeEnum        = `["string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"]`
)

func (s *ToolService) ListTools() []ToolDefinition {
	return []ToolDefinition{
		// --- Query ---
		{
			Name: "query_data",
			Description: `Execute a permission-scoped Cornerstone Query DSL request against allowed tables.

The query body uses the Cornerstone Query DSL with these top-level fields (same as the REST API /api/v1/query endpoint):
- "from" (required): The table to query. Allowed values: ` + allowedDSLTables + `. Example: "records".
- "select": Array of field names to return. Omit to return all allowed fields.
  Note: When using JOIN, use qualified names like "records.id" to avoid ambiguous column errors.
- "where": Filter conditions. Use {"and": [...]} or {"or": [...]} with condition objects {"field": "<name>", "op": "<operator>", "value": <val>}.
  Supported operators: eq, ne, gt, gte, lt, lte, in, not_in, like, not_like, is_null, is_not_null, between.
  For user record data, use "data.<field_name>" as the field path (e.g. "data.email").
- "orderBy": Array of {"field": "<name>", "direction": "asc"|"desc"}.
- "page": Page number (1-based). Default: 1.
- "size": Page size. Default: 20, max: 100.
- "table": (simplified) A table ID like "tbl_xxx" to filter records by table. Shorthand for filtering by table_id.
- "filter": (simplified) A JSON object of field-value pairs for equality filtering on record data fields.

Example: List records in a user table with pagination:
{"from": "records", "table": "tbl_abc123", "page": 1, "size": 10}

Example: Query with conditions:
{"from": "records", "table": "tbl_abc123", "where": {"and": [{"field": "data.status", "op": "eq", "value": "active"}]}, "orderBy": [{"field": "created_at", "direction": "desc"}]}

Example: JOIN query (note the qualified select fields):
{"from": "records", "select": ["records.id", "records.data"], "join": [{"type": "left", "table": "tables", "as": "t", "on": {"left": "records.table_id", "op": "=", "right": "t.id"}, "select": ["t.name"]}]}`,
			InputSchema: map[string]interface{}{
				"type":                 "object",
				"description":          "Cornerstone Query DSL request body. Pass Query DSL fields directly (same as REST API).",
				"additionalProperties": true,
			},
		},

		// --- Database CRUD ---
		{
			Name:        "create_database",
			Description: `Create a new structured database. A database is a top-level container for tables. Returns the created database with its generated ID (prefixed with "db_"). Use create_database_with_tables to create a database with tables in a single call.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Database name (2-255 characters).",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Optional database description (max 500 characters).",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "list_databases",
			Description: `List all databases accessible to the current user. Returns database IDs, names, and descriptions. Use list_tables to explore tables within a database.`,
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_database",
			Description: `Get details of a specific database by its ID. Returns the database name, description, and timestamps.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_id": map[string]interface{}{
						"type":        "string",
						"description": `Database ID (prefixed with "db_").`,
					},
				},
				"required": []string{"database_id"},
			},
		},
		{
			Name:        "update_database",
			Description: `Update a database's name and/or description. Provide at least one field to update.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_id": map[string]interface{}{
						"type":        "string",
						"description": `Database ID (prefixed with "db_").`,
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New database name (2-255 characters).",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New database description (max 500 characters).",
					},
				},
				"required": []string{"database_id", "name"},
			},
		},
		{
			Name:        "delete_database",
			Description: `Delete a database and all its tables, fields, and records. This action is irreversible. Use with caution.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_id": map[string]interface{}{
						"type":        "string",
						"description": `Database ID (prefixed with "db_").`,
					},
				},
				"required": []string{"database_id"},
			},
		},
		{
			Name:        "create_database_with_tables",
			Description: `Create a database with nested tables and fields in a single atomic operation. This is the recommended way to set up a new project structure. Each table can include field definitions. Returns the created database, tables, and fields.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Database name (2-255 characters).",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Database description.",
					},
					"tables": map[string]interface{}{
						"type":        "array",
						"description": "Tables to create inside the database.",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "Table name.",
								},
								"description": map[string]interface{}{
									"type":        "string",
									"description": "Table description.",
								},
								"fields": map[string]interface{}{
									"type":        "array",
									"description": "Field definitions for the table.",
									"items": map[string]interface{}{
										"type": "object",
										"properties": map[string]interface{}{
											"name": map[string]interface{}{
												"type":        "string",
												"description": "Field name.",
											},
											"type": map[string]interface{}{
												"type":        "string",
												"description": fieldTypeDescription,
												"enum":        []string{"string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"},
											},
											"description": map[string]interface{}{
												"type":        "string",
												"description": "Field description.",
											},
											"required": map[string]interface{}{
												"type":        "boolean",
												"description": "Whether this field is required for record creation.",
											},
										},
										"required": []string{"name", "type"},
									},
								},
							},
							"required": []string{"name"},
						},
					},
				},
				"required": []string{"name", "tables"},
			},
		},

		// --- Table CRUD ---
		{
			Name:        "create_table",
			Description: `Create a new table in a database. Optionally include field definitions. Returns the created table with its generated ID (prefixed with "tbl_") and any created fields. Use list_tables to discover existing tables in a database.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_id": map[string]interface{}{
						"type":        "string",
						"description": `Database ID (prefixed with "db_") to create the table in.`,
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Table name (2-255 characters).",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Table description (max 500 characters).",
					},
					"fields": map[string]interface{}{
						"type":        "array",
						"description": "Optional field definitions to create alongside the table.",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{
									"type":        "string",
									"description": "Field name.",
								},
								"type": map[string]interface{}{
									"type":        "string",
									"description": fieldTypeDescription,
									"enum":        []string{"string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"},
								},
								"description": map[string]interface{}{
									"type":        "string",
									"description": "Field description.",
								},
								"required": map[string]interface{}{
									"type":        "boolean",
									"description": "Whether the field is required.",
								},
							},
							"required": []string{"name", "type"},
						},
					},
				},
				"required": []string{"database_id", "name"},
			},
		},
		{
			Name:        "list_tables",
			Description: `List all tables in a database. Returns table IDs, names, and descriptions. Use list_fields to see field definitions for a specific table.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_id": map[string]interface{}{
						"type":        "string",
						"description": `Database ID (prefixed with "db_").`,
					},
				},
				"required": []string{"database_id"},
			},
		},
		{
			Name:        "get_table",
			Description: `Get details of a specific table by its ID. Returns the table name, description, database ID, and timestamps.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
				},
				"required": []string{"table_id"},
			},
		},
		{
			Name:        "update_table",
			Description: `Update a table's name and/or description.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New table name (2-255 characters).",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New table description (max 500 characters).",
					},
				},
				"required": []string{"table_id", "name"},
			},
		},
		{
			Name:        "delete_table",
			Description: `Delete a table and all its fields and records. This action is irreversible.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
				},
				"required": []string{"table_id"},
			},
		},

		// --- Field CRUD ---
		{
			Name:        "create_field",
			Description: `Add a new field to an existing table. The field type determines how values are stored and validated when records are created or updated. Use list_fields to see existing fields on a table.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_") to add the field to.`,
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Field name (1-255 characters). Must be unique within the table.",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": fieldTypeDescription,
						"enum":        []string{"string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"},
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Field description (max 1000 characters).",
					},
					"required": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether this field must have a value when creating records.",
					},
				},
				"required": []string{"table_id", "name", "type"},
			},
		},
		{
			Name:        "list_fields",
			Description: `List all field definitions for a table. Returns field names, types, descriptions, and required flags. Essential for understanding what keys and value types to use when inserting or updating records.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
				},
				"required": []string{"table_id"},
			},
		},
		{
			Name:        "update_field",
			Description: `Update a field's name, type, description, or required status. Changing the type may affect existing record data validation.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field_id": map[string]interface{}{
						"type":        "string",
						"description": `Field ID (prefixed with "fld_").`,
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "New field name (1-255 characters).",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": fieldTypeDescription,
						"enum":        []string{"string", "text", "number", "boolean", "date", "datetime", "file", "json", "list"},
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "New field description.",
					},
					"required": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the field is required.",
					},
				},
				"required": []string{"field_id", "name", "type"},
			},
		},
		{
			Name:        "delete_field",
			Description: `Delete a field from a table. Existing record data for this field is preserved but the field definition is removed.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"field_id": map[string]interface{}{
						"type":        "string",
						"description": `Field ID (prefixed with "fld_").`,
					},
				},
				"required": []string{"field_id"},
			},
		},

		// --- Record CRUD ---
		{
			Name:        "insert_record",
			Description: `Insert a single record into a table. The data object keys must match field names defined on the table, and values must conform to the field's type. Use list_fields to discover available field names and types. Returns the created record with its generated ID (prefixed with "rec_") and version number.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
					"data": map[string]interface{}{
						"type":                 "object",
						"description":          "Record data as key-value pairs. Keys must match field names on the table. Use list_fields to discover available fields and their expected types.",
						"additionalProperties": true,
					},
				},
				"required": []string{"table_id", "data"},
			},
		},
		{
			Name:        "list_records",
			Description: `List records in a table with pagination. A simplified alternative to query_data for basic record browsing. Returns records with their data, version numbers, and timestamps.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
					"limit": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of records to return (1-100). Default: 20.",
						"minimum":     1,
						"maximum":     100,
					},
					"offset": map[string]interface{}{
						"type":        "integer",
						"description": "Number of records to skip. Default: 0.",
						"minimum":     0,
					},
					"filter": map[string]interface{}{
						"type":        "string",
						"description": "Optional JSON filter object for equality matching on record data fields. Example: {\"status\":\"active\",\"priority\":\"high\"}",
					},
				},
				"required": []string{"table_id"},
			},
		},
		{
			Name:        "get_record",
			Description: `Get a single record by its ID. Returns the record's data, version, and timestamps.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"record_id": map[string]interface{}{
						"type":        "string",
						"description": `Record ID (prefixed with "rec_").`,
					},
				},
				"required": []string{"record_id"},
			},
		},
		{
			Name:        "update_record",
			Description: `Update a single record's data. Only the provided keys are updated; omitted keys remain unchanged. Returns the updated record with an incremented version number.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"record_id": map[string]interface{}{
						"type":        "string",
						"description": `Record ID (prefixed with "rec_").`,
					},
					"data": map[string]interface{}{
						"type":                 "object",
						"description":          "Updated field values. Only provided keys are changed. Keys must match field names on the table.",
						"additionalProperties": true,
					},
				},
				"required": []string{"record_id", "data"},
			},
		},
		{
			Name:        "delete_record",
			Description: `Delete a single record by its ID. This action is irreversible.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"record_id": map[string]interface{}{
						"type":        "string",
						"description": `Record ID (prefixed with "rec_").`,
					},
				},
				"required": []string{"record_id"},
			},
		},
		{
			Name:        "batch_insert_records",
			Description: `Insert multiple records into a table at once. Each record is an independent data object. Returns the created records with their generated IDs.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
					"records": map[string]interface{}{
						"type":        "array",
						"description": "Array of record data objects. Each object's keys must match field names on the table.",
						"items": map[string]interface{}{
							"type":                 "object",
							"additionalProperties": true,
						},
					},
				},
				"required": []string{"table_id", "records"},
			},
		},
		{
			Name:        "generate_test_data",
			Description: `Generate realistic test data for a table. Automatically creates records with random values matching each field's type. Useful for prototyping and testing. Returns the generated records.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": `Table ID (prefixed with "tbl_").`,
					},
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "Number of test records to generate (1-100).",
						"minimum":     1,
						"maximum":     100,
					},
				},
				"required": []string{"table_id", "count"},
			},
		},

		// --- Schema introspection ---
		{
			Name:        "get_table_schema",
			Description: `Return the allowed schema fields for a system Query DSL table. This returns field names available for the "from" table in query_data, NOT user-defined table schemas. To inspect user table fields, use list_fields instead. Allowed table names: ` + allowedDSLTables + `.`,
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query_table_name": map[string]interface{}{
						"type":        "string",
						"description": "System Query DSL table name (preferred). This is a logical table name used in the \"from\" field of query_data, NOT a user table ID.",
						"enum":        []string{"records", "tables", "databases", "fields", "files", "tokens"},
					},
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Legacy alias for query_table_name. Prefer query_table_name for clarity.",
						"enum":        []string{"records", "tables", "databases", "fields", "files", "tokens"},
					},
				},
				"required": []string{},
			},
		},
	}
}

// --- Tool dispatch ---

func (s *ToolService) Call(ctx context.Context, name string, args json.RawMessage) (*ToolCallResult, error) {
	switch name {
	case "query_data":
		return s.callQueryData(ctx, args)
	case "create_database":
		return s.callCreateDatabase(args)
	case "list_databases":
		return s.callListDatabases()
	case "get_database":
		return s.callGetDatabase(args)
	case "update_database":
		return s.callUpdateDatabase(args)
	case "delete_database":
		return s.callDeleteDatabase(args)
	case "create_database_with_tables":
		return s.callCreateDatabaseWithTables(args)
	case "create_table":
		return s.callCreateTable(args)
	case "list_tables":
		return s.callListTables(args)
	case "get_table":
		return s.callGetTable(args)
	case "update_table":
		return s.callUpdateTable(args)
	case "delete_table":
		return s.callDeleteTable(args)
	case "create_field":
		return s.callCreateField(args)
	case "list_fields":
		return s.callListFields(args)
	case "update_field":
		return s.callUpdateField(args)
	case "delete_field":
		return s.callDeleteField(args)
	case "insert_record":
		return s.callInsertRecord(args)
	case "list_records":
		return s.callListRecords(args)
	case "get_record":
		return s.callGetRecord(args)
	case "update_record":
		return s.callUpdateRecord(args)
	case "delete_record":
		return s.callDeleteRecord(args)
	case "batch_insert_records":
		return s.callBatchInsertRecords(args)
	case "generate_test_data":
		return s.callGenerateTestData(args)
	case "get_table_schema":
		return s.callGetTableSchema(ctx, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

// --- Error helper ---

func errorResult(summary, code, message string) *ToolCallResult {
	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: summary}},
		StructuredContent: map[string]interface{}{
			"error": message,
			"code":  code,
		},
		IsError: true,
	}
}

// --- Query tools ---

func (s *ToolService) callQueryData(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var req query.QueryRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid query_data arguments: %w", err)
	}

	result, err := s.queryExecutor.Execute(ctx, &req, s.userID)
	if err != nil {
		return errorResult("Query execution failed.", "QUERY_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Query succeeded with %d row(s).", len(result.Data))}},
		StructuredContent: result,
	}, nil
}

func (s *ToolService) callGetTableSchema(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		QueryTableName string `json:"query_table_name"`
		Table          string `json:"table"` // legacy alias, kept for backward compatibility
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid get_table_schema arguments: %w", err)
	}

	tableName := req.QueryTableName
	if tableName == "" {
		tableName = req.Table
	}
	if tableName == "" {
		return errorResult("Missing table name.", "VALIDATION_ERROR", "Provide query_table_name (or table) parameter."), nil
	}

	validator := s.queryExecutor.GetValidator()
	if err := validator.CheckTableAccess(ctx, s.userID, tableName); err != nil {
		return errorResult("Table schema lookup failed.", "ACCESS_DENIED", err.Error()), nil
	}

	payload := map[string]interface{}{
		"table":  tableName,
		"fields": query.DefaultAllowedTables.GetAllowedFields(tableName),
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Returned schema for table %q.", tableName)}},
		StructuredContent: payload,
	}, nil
}

// --- Database tools ---

func (s *ToolService) callCreateDatabase(args json.RawMessage) (*ToolCallResult, error) {
	var req services.CreateDBRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid create_database arguments: %w", err)
	}

	database, err := s.databaseService.CreateDatabase(req, s.userID)
	if err != nil {
		return errorResult("Database creation failed.", "CREATE_ERROR", err.Error()), nil
	}

	if s.notifier != nil {
		s.notifier.PublishToUser(s.userID, "notifications/databases/changed", map[string]interface{}{
			"action": "created",
			"database": map[string]interface{}{
				"id":          database.ID,
				"name":        database.Name,
				"description": database.Description,
			},
		})
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Database %q created.", database.Name)}},
		StructuredContent: shapeDatabase(database),
	}, nil
}

func (s *ToolService) callListDatabases() (*ToolCallResult, error) {
	databases, err := s.databaseService.ListDatabases(s.userID)
	if err != nil {
		return errorResult("Listing databases failed.", "QUERY_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Found %d accessible database(s).", len(databases))}},
		StructuredContent: map[string]interface{}{"databases": databases, "total": len(databases)},
	}, nil
}

func (s *ToolService) callGetDatabase(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		DatabaseID string `json:"database_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid get_database arguments: %w", err)
	}

	db, err := s.databaseService.GetDatabase(req.DatabaseID, s.userID)
	if err != nil {
		return errorResult("Database not found.", "NOT_FOUND", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Database %q details.", db.Name)}},
		StructuredContent: db,
	}, nil
}

func (s *ToolService) callUpdateDatabase(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		DatabaseID  string `json:"database_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid update_database arguments: %w", err)
	}

	database, err := s.databaseService.UpdateDatabase(req.DatabaseID, services.UpdateDBRequest{
		Name:        req.Name,
		Description: req.Description,
	}, s.userID)
	if err != nil {
		return errorResult("Database update failed.", "UPDATE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Database %q updated.", database.Name)}},
		StructuredContent: shapeDatabase(database),
	}, nil
}

func (s *ToolService) callDeleteDatabase(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		DatabaseID string `json:"database_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid delete_database arguments: %w", err)
	}

	if err := s.databaseService.DeleteDatabase(req.DatabaseID, s.userID); err != nil {
		return errorResult("Database deletion failed.", "DELETE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Database %q deleted.", req.DatabaseID)}},
		StructuredContent: map[string]interface{}{
			"database_id": req.DatabaseID,
			"deleted":     true,
		},
	}, nil
}

func (s *ToolService) callCreateDatabaseWithTables(args json.RawMessage) (*ToolCallResult, error) {
	var req services.CreateDBWithTablesRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid create_database_with_tables arguments: %w", err)
	}

	result, err := s.databaseService.CreateDatabaseWithTables(req, s.userID)
	if err != nil {
		return errorResult("Database creation with tables failed.", "CREATE_ERROR", err.Error()), nil
	}

	tables := make([]map[string]interface{}, 0, len(result.Tables))
	for _, t := range result.Tables {
		tables = append(tables, shapeTable(t))
	}
	fields := make([]map[string]interface{}, 0, len(result.Fields))
	for _, f := range result.Fields {
		fields = append(fields, shapeField(f))
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Database %q created with %d table(s) and %d field(s).", result.Database.Name, len(result.Tables), len(result.Fields))}},
		StructuredContent: map[string]interface{}{
			"database": shapeDatabase(result.Database),
			"tables":   tables,
			"fields":   fields,
		},
	}, nil
}

// --- Table tools ---

func (s *ToolService) callCreateTable(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		DatabaseID  string `json:"database_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
		Fields      []struct {
			Name        string `json:"name"`
			Type        string `json:"type"`
			Description string `json:"description"`
			Required    bool   `json:"required"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid create_table arguments: %w", err)
	}

	database, err := s.databaseService.GetDatabase(req.DatabaseID, s.userID)
	if err != nil {
		return errorResult("Database not found.", "NOT_FOUND", err.Error()), nil
	}

	tableService := services.NewTableService(s.db)
	table, err := tableService.CreateTable(services.CreateTableRequest{
		DatabaseID:  req.DatabaseID,
		Name:        req.Name,
		Description: req.Description,
	}, s.userID)
	if err != nil {
		return errorResult("Table creation failed.", "CREATE_ERROR", err.Error()), nil
	}

	fieldService := services.NewFieldService(s.db)
	var createdFields []map[string]interface{}
	var fieldErrors []map[string]interface{}
	for _, f := range req.Fields {
		field, err := fieldService.CreateField(services.CreateFieldRequest{
			TableID:     table.ID,
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Required:    f.Required,
		}, s.userID)
		if err != nil {
			fieldErrors = append(fieldErrors, map[string]interface{}{
				"name":  f.Name,
				"type":  f.Type,
				"error": err.Error(),
			})
		} else {
			createdFields = append(createdFields, shapeField(field))
		}
	}

	result := map[string]interface{}{
		"table":  shapeTable(table),
		"fields": createdFields,
	}
	if len(fieldErrors) > 0 {
		result["field_errors"] = fieldErrors
	}

	summary := fmt.Sprintf("Table %q created in %q with %d field(s).", table.Name, database.Name, len(createdFields))
	if len(fieldErrors) > 0 {
		summary += fmt.Sprintf(" %d field(s) failed.", len(fieldErrors))
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: summary}},
		StructuredContent: result,
	}, nil
}

func (s *ToolService) callListTables(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		DatabaseID string `json:"database_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid list_tables arguments: %w", err)
	}

	tableService := services.NewTableService(s.db)
	tables, err := tableService.ListTables(req.DatabaseID, s.userID)
	if err != nil {
		return errorResult("Listing tables failed.", "QUERY_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Found %d table(s).", len(tables))}},
		StructuredContent: map[string]interface{}{"tables": tables, "total": len(tables), "database_id": req.DatabaseID},
	}, nil
}

func (s *ToolService) callGetTable(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string `json:"table_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid get_table arguments: %w", err)
	}

	tableService := services.NewTableService(s.db)
	table, err := tableService.GetTable(req.TableID, s.userID)
	if err != nil {
		return errorResult("Table not found.", "NOT_FOUND", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Table %q details.", table.Name)}},
		StructuredContent: table,
	}, nil
}

func (s *ToolService) callUpdateTable(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID     string `json:"table_id"`
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid update_table arguments: %w", err)
	}

	tableService := services.NewTableService(s.db)
	table, err := tableService.UpdateTable(req.TableID, services.UpdateTableRequest{
		Name:        req.Name,
		Description: req.Description,
	}, s.userID)
	if err != nil {
		return errorResult("Table update failed.", "UPDATE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Table %q updated.", table.Name)}},
		StructuredContent: shapeTable(table),
	}, nil
}

func (s *ToolService) callDeleteTable(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string `json:"table_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid delete_table arguments: %w", err)
	}

	tableService := services.NewTableService(s.db)
	if err := tableService.DeleteTable(req.TableID, s.userID); err != nil {
		return errorResult("Table deletion failed.", "DELETE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Table %q deleted.", req.TableID)}},
		StructuredContent: map[string]interface{}{
			"table_id": req.TableID,
			"deleted":  true,
		},
	}, nil
}

// --- Field tools ---

func (s *ToolService) callCreateField(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID     string `json:"table_id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid create_field arguments: %w", err)
	}

	fieldService := services.NewFieldService(s.db)
	field, err := fieldService.CreateField(services.CreateFieldRequest{
		TableID:     req.TableID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Required:    req.Required,
	}, s.userID)
	if err != nil {
		return errorResult("Field creation failed.", "CREATE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Field %q (%s) created.", field.Name, field.Type)}},
		StructuredContent: shapeField(field),
	}, nil
}

func (s *ToolService) callListFields(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string `json:"table_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid list_fields arguments: %w", err)
	}

	fieldService := services.NewFieldService(s.db)
	fields, err := fieldService.ListFields(req.TableID, s.userID)
	if err != nil {
		return errorResult("Listing fields failed.", "QUERY_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Found %d field(s).", len(fields))}},
		StructuredContent: map[string]interface{}{"fields": fields, "total": len(fields), "table_id": req.TableID},
	}, nil
}

func (s *ToolService) callUpdateField(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		FieldID     string `json:"field_id"`
		Name        string `json:"name"`
		Type        string `json:"type"`
		Description string `json:"description"`
		Required    bool   `json:"required"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid update_field arguments: %w", err)
	}

	fieldService := services.NewFieldService(s.db)
	field, err := fieldService.UpdateField(req.FieldID, services.UpdateFieldRequest{
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Required:    req.Required,
	}, s.userID)
	if err != nil {
		return errorResult("Field update failed.", "UPDATE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Field %q updated.", field.Name)}},
		StructuredContent: shapeField(field),
	}, nil
}

func (s *ToolService) callDeleteField(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		FieldID string `json:"field_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid delete_field arguments: %w", err)
	}

	fieldService := services.NewFieldService(s.db)
	if err := fieldService.DeleteField(req.FieldID, s.userID); err != nil {
		return errorResult("Field deletion failed.", "DELETE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Field %q deleted.", req.FieldID)}},
		StructuredContent: map[string]interface{}{
			"field_id": req.FieldID,
			"deleted":  true,
		},
	}, nil
}

// --- Record tools ---

func (s *ToolService) callInsertRecord(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string                 `json:"table_id"`
		Data    map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid insert_record arguments: %w", err)
	}

	recordService := services.NewRecordService(s.db)
	record, err := recordService.CreateRecord(services.CreateRecordRequest{
		TableID: req.TableID,
		Data:    req.Data,
	}, s.userID)
	if err != nil {
		return errorResult("Record insertion failed.", "CREATE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Record %s inserted.", record.ID)}},
		StructuredContent: shapeRecord(record),
	}, nil
}

func (s *ToolService) callListRecords(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string `json:"table_id"`
		Limit   int    `json:"limit"`
		Offset  int    `json:"offset"`
		Filter  string `json:"filter"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid list_records arguments: %w", err)
	}

	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 100 {
		req.Limit = 100
	}

	recordService := services.NewRecordService(s.db)
	result, err := recordService.ListRecords(services.QueryRequest{
		TableID: req.TableID,
		Limit:   req.Limit,
		Offset:  req.Offset,
		Filter:  req.Filter,
	}, s.userID)
	if err != nil {
		return errorResult("Listing records failed.", "QUERY_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Found %d record(s), total %d.", len(result.Records), result.Total)}},
		StructuredContent: result,
	}, nil
}

func (s *ToolService) callGetRecord(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		RecordID string `json:"record_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid get_record arguments: %w", err)
	}

	recordService := services.NewRecordService(s.db)
	record, err := recordService.GetRecord(req.RecordID, s.userID, "")
	if err != nil {
		return errorResult("Record not found.", "NOT_FOUND", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Record %s details.", record.ID)}},
		StructuredContent: record,
	}, nil
}

func (s *ToolService) callUpdateRecord(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		RecordID string                 `json:"record_id"`
		Data     map[string]interface{} `json:"data"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid update_record arguments: %w", err)
	}

	recordService := services.NewRecordService(s.db)
	record, err := recordService.UpdateRecord(req.RecordID, services.UpdateRecordRequest{
		Data: req.Data,
	}, s.userID)
	if err != nil {
		return errorResult("Record update failed.", "UPDATE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Record %s updated to version %d.", record.ID, record.Version)}},
		StructuredContent: shapeRecord(record),
	}, nil
}

func (s *ToolService) callDeleteRecord(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		RecordID string `json:"record_id"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid delete_record arguments: %w", err)
	}

	recordService := services.NewRecordService(s.db)
	if err := recordService.DeleteRecord(req.RecordID, s.userID); err != nil {
		return errorResult("Record deletion failed.", "DELETE_ERROR", err.Error()), nil
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Record %s deleted.", req.RecordID)}},
		StructuredContent: map[string]interface{}{
			"record_id": req.RecordID,
			"deleted":   true,
		},
	}, nil
}

func (s *ToolService) callBatchInsertRecords(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string                   `json:"table_id"`
		Records []map[string]interface{} `json:"records"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid batch_insert_records arguments: %w", err)
	}

	if len(req.Records) == 0 {
		return errorResult("No records provided.", "VALIDATION_ERROR", "The records array must contain at least one record."), nil
	}

	recordService := services.NewRecordService(s.db)
	var created []map[string]interface{}
	var errors []map[string]interface{}

	for i, data := range req.Records {
		record, err := recordService.CreateRecord(services.CreateRecordRequest{
			TableID: req.TableID,
			Data:    data,
		}, s.userID)
		if err != nil {
			errors = append(errors, map[string]interface{}{
				"index": i,
				"error": err.Error(),
			})
		} else {
			created = append(created, shapeRecord(record))
		}
	}

	result := map[string]interface{}{
		"records":  created,
		"created":  len(created),
		"table_id": req.TableID,
	}
	if len(errors) > 0 {
		result["errors"] = errors
	}

	summary := fmt.Sprintf("Inserted %d record(s).", len(created))
	if len(errors) > 0 {
		summary += fmt.Sprintf(" %d record(s) failed.", len(errors))
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: summary}},
		StructuredContent: result,
	}, nil
}

func (s *ToolService) callGenerateTestData(args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		TableID string `json:"table_id"`
		Count   int    `json:"count"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid generate_test_data arguments: %w", err)
	}

	if req.Count <= 0 || req.Count > 100 {
		return errorResult("Count must be between 1 and 100.", "VALIDATION_ERROR", fmt.Sprintf("count=%d is out of range [1, 100].", req.Count)), nil
	}

	recordService := services.NewRecordService(s.db)
	records, err := recordService.GenerateTestData(req.TableID, s.userID, req.Count)
	if err != nil {
		return errorResult("Test data generation failed.", "CREATE_ERROR", err.Error()), nil
	}

	shaped := make([]map[string]interface{}, 0, len(records))
	for _, r := range records {
		shaped = append(shaped, shapeRecord(r))
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Generated %d test record(s).", len(records))}},
		StructuredContent: map[string]interface{}{
			"table_id": req.TableID,
			"count":    len(records),
			"records":  shaped,
		},
	}, nil
}
