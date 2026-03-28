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
					"is_public": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the database is public.",
					},
					"is_personal": map[string]interface{}{
						"type":        "boolean",
						"description": "Whether the database is personal.",
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
				"is_public":   database.IsPublic,
				"is_personal": database.IsPersonal,
				"owner_id":    database.OwnerID,
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
