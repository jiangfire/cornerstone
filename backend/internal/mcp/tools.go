package mcp

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/query"
	"gorm.io/gorm"
)

// ToolService 封装 MCP 暴露的业务能力
type ToolService struct {
	db              *gorm.DB
	userID          string
	queryExecutor   *query.Executor
	databaseService *services.DatabaseService
	notifier        Notifier
}

// NewToolService 创建 MCP tool service
func NewToolService(database *gorm.DB, userID string) *ToolService {
	return NewToolServiceWithNotifier(database, userID, nil)
}

// NewToolServiceWithNotifier 创建带通知能力的 MCP tool service。
func NewToolServiceWithNotifier(database *gorm.DB, userID string, notifier Notifier) *ToolService {
	return &ToolService{
		db:              database,
		userID:          userID,
		queryExecutor:   query.NewExecutor(database),
		databaseService: services.NewDatabaseService(database),
		notifier:        notifier,
	}
}

func (s *ToolService) ListTools() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "query_data",
			Description: "Execute a permission-scoped Cornerstone query DSL request against allowed tables.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"query": map[string]interface{}{
						"type":                 "object",
						"description":          "Cornerstone Query DSL request body.",
						"additionalProperties": true,
					},
				},
				"required": []string{"query"},
			},
		},
		{
			Name:        "create_database",
			Description: "Create a new Cornerstone database for the configured MCP user.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Database name.",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Database description.",
					},
				},
				"required": []string{"name"},
			},
		},
		{
			Name:        "list_databases",
			Description: "List databases accessible to the configured MCP user.",
			InputSchema: map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			},
		},
		{
			Name:        "get_table_schema",
			Description: "Return the allowed schema fields for an accessible Query DSL table.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table": map[string]interface{}{
						"type":        "string",
						"description": "Allowed Query DSL table name.",
					},
				},
				"required": []string{"table"},
			},
		},
		{
			Name:        "create_table",
			Description: "Create a new table in a database.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"database_id": map[string]interface{}{
						"type":        "string",
						"description": "Database ID.",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Table name.",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Table description.",
					},
					"fields": map[string]interface{}{
						"type": "array",
						"description": "Field definitions.",
						"items": map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"name": map[string]interface{}{"type": "string"},
								"type": map[string]interface{}{"type": "string"},
								"description": map[string]interface{}{"type": "string"},
								"required": map[string]interface{}{"type": "boolean"},
							},
							"required": []string{"name", "type"},
						},
					},
				},
				"required": []string{"database_id", "name"},
			},
		},
		{
			Name:        "create_field",
			Description: "Create a new field in a table.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": "Table ID.",
					},
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Field name.",
					},
					"type": map[string]interface{}{
						"type":        "string",
						"description": "Field type.",
					},
					"description": map[string]interface{}{
						"type":        "string",
						"description": "Field description.",
					},
				},
				"required": []string{"table_id", "name", "type"},
			},
		},
		{
			Name:        "insert_record",
			Description: "Insert a record into a table.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": "Table ID.",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "Record data as key-value pairs.",
					},
				},
				"required": []string{"table_id", "data"},
			},
		},
		{
			Name:        "update_record",
			Description: "Update a single record.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"record_id": map[string]interface{}{
						"type":        "string",
						"description": "Record ID.",
					},
					"data": map[string]interface{}{
						"type":        "object",
						"description": "Updated field values.",
					},
				},
				"required": []string{"record_id", "data"},
			},
		},
		{
			Name:        "delete_record",
			Description: "Delete a single record.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"record_id": map[string]interface{}{
						"type":        "string",
						"description": "Record ID.",
					},
				},
				"required": []string{"record_id"},
			},
		},
		{
			Name:        "generate_test_data",
			Description: "Generate test data for a table.",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"table_id": map[string]interface{}{
						"type":        "string",
						"description": "Table ID.",
					},
					"count": map[string]interface{}{
						"type":        "integer",
						"description": "Number of records to generate.",
					},
				},
				"required": []string{"table_id", "count"},
			},
		},
	}
}

// Call 执行指定 tool
func (s *ToolService) Call(ctx context.Context, name string, args json.RawMessage) (*ToolCallResult, error) {
	switch name {
	case "query_data":
		return s.callQueryData(ctx, args)
	case "create_database":
		return s.callCreateDatabase(args)
	case "list_databases":
		return s.callListDatabases()
	case "get_table_schema":
		return s.callGetTableSchema(ctx, args)
	case "create_table":
		return s.callCreateTable(args)
	case "create_field":
		return s.callCreateField(args)
	case "insert_record":
		return s.callInsertRecord(args)
	case "update_record":
		return s.callUpdateRecord(args)
	case "delete_record":
		return s.callDeleteRecord(args)
	case "generate_test_data":
		return s.callGenerateTestData(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (s *ToolService) callQueryData(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		Query query.QueryRequest `json:"query"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid query_data arguments: %w", err)
	}

	result, err := s.queryExecutor.Execute(ctx, &req.Query, s.userID)
	if err != nil {
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Query execution failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Query succeeded with %d row(s).", len(result.Data))}},
		StructuredContent: result,
	}, nil
}

func (s *ToolService) callCreateDatabase(args json.RawMessage) (*ToolCallResult, error) {
	var req services.CreateDBRequest
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid create_database arguments: %w", err)
	}

	database, err := s.databaseService.CreateDatabase(req, s.userID)
	if err != nil {
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Database creation failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
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
		StructuredContent: database,
	}, nil
}

func (s *ToolService) callListDatabases() (*ToolCallResult, error) {
	databases, err := s.databaseService.ListDatabases(s.userID)
	if err != nil {
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Listing databases failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	payload := map[string]interface{}{
		"databases": databases,
		"total":     len(databases),
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Found %d accessible database(s).", len(databases))}},
		StructuredContent: payload,
	}, nil
}

func (s *ToolService) callGetTableSchema(ctx context.Context, args json.RawMessage) (*ToolCallResult, error) {
	var req struct {
		Table string `json:"table"`
	}
	if err := json.Unmarshal(args, &req); err != nil {
		return nil, fmt.Errorf("invalid get_table_schema arguments: %w", err)
	}

	validator := s.queryExecutor.GetValidator()
	if err := validator.CheckTableAccess(ctx, s.userID, req.Table); err != nil {
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Table schema lookup failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	payload := map[string]interface{}{
		"table":  req.Table,
		"fields": query.DefaultAllowedTables.GetAllowedFields(req.Table),
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Returned schema for table %q.", req.Table)}},
		StructuredContent: payload,
	}, nil
}

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
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Database not found."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	tableService := services.NewTableService(s.db)
	table, err := tableService.CreateTable(services.CreateTableRequest{
		DatabaseID:  req.DatabaseID,
		Name:        req.Name,
		Description: req.Description,
	}, s.userID)
	if err != nil {
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Table creation failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	fieldService := services.NewFieldService(s.db)
	for _, f := range req.Fields {
		_, _ = fieldService.CreateField(services.CreateFieldRequest{
			TableID:     table.ID,
			Name:        f.Name,
			Type:        f.Type,
			Description: f.Description,
			Required:    f.Required,
		}, s.userID)
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Table %q created in %q.", table.Name, database.Name)}},
		StructuredContent: table,
	}, nil
}

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
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Field creation failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: fmt.Sprintf("Field %q created.", field.Name)}},
		StructuredContent: field,
	}, nil
}

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
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Record insertion failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: "Record inserted."}},
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
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Record update failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content:           []TextContent{{Type: "text", Text: "Record updated."}},
		StructuredContent: record,
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
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Record deletion failed."}},
			StructuredContent: map[string]interface{}{
				"error": err.Error(),
			},
			IsError: true,
		}, nil
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: "Record deleted."}},
		StructuredContent: map[string]interface{}{
			"record_id": req.RecordID,
		},
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
		return &ToolCallResult{
			Content: []TextContent{{Type: "text", Text: "Count must be between 1 and 100."}},
			IsError: true,
		}, nil
	}

	recordService := services.NewRecordService(s.db)
	inserted := 0
	for i := 0; i < req.Count; i++ {
		_, err := recordService.CreateRecord(services.CreateRecordRequest{
			TableID: req.TableID,
			Data:    map[string]interface{}{},
		}, s.userID)
		if err != nil {
			return &ToolCallResult{
				Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Generated %d records before error.", inserted)}},
				StructuredContent: map[string]interface{}{
					"error":    err.Error(),
					"inserted": inserted,
				},
				IsError: true,
			}, nil
		}
		inserted++
	}

	return &ToolCallResult{
		Content: []TextContent{{Type: "text", Text: fmt.Sprintf("Generated %d test records.", inserted)}},
		StructuredContent: map[string]interface{}{
			"table_id": req.TableID,
			"count":    inserted,
		},
	}, nil
}
