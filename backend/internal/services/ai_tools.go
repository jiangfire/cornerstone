package services

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/jiangfire/cornerstone/backend/pkg/query"
	"gorm.io/gorm"
)

type DBResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type TableResult struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func ExecuteAIToolForToken(db *gorm.DB, tokenID, name string, args map[string]any) (any, error) {
	switch name {
	case "list_databases":
		databases, err := NewDatabaseService(db).ListDatabases(tokenID)
		if err != nil {
			return nil, err
		}
		result := make([]DBResult, len(databases))
		for i, database := range databases {
			result[i] = DBResult{ID: database.ID, Name: database.Name}
		}
		return result, nil

	case "list_tables":
		databaseID, ok := args["database_id"].(string)
		if !ok {
			return nil, fmt.Errorf("database_id required")
		}
		tables, err := NewTableService(db).ListTables(databaseID, tokenID)
		if err != nil {
			return nil, err
		}
		result := make([]TableResult, len(tables))
		for i, table := range tables {
			result[i] = TableResult{ID: table.ID, Name: table.Name}
		}
		return result, nil

	case "get_schema":
		return executeGetSchema(db, tokenID, args)

	case "create_database":
		return executeCreateDatabase(db, tokenID, args)

	case "create_table":
		return executeCreateTable(db, tokenID, args)

	case "create_field":
		return executeCreateField(db, tokenID, args)

	case "execute_query":
		return executeQuery(db, tokenID, args)

	case "insert_records":
		return executeInsertRecords(db, tokenID, args)

	case "update_record":
		return executeUpdateRecord(db, tokenID, args)

	case "delete_record":
		return executeDeleteRecord(db, tokenID, args)

	case "generate_test_data":
		return executeGenerateTestData(db, tokenID, args)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func executeGetSchema(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	tableID, hasTable := args["table_id"].(string)
	databaseID, hasDB := args["database_id"].(string)

	if hasTable {
		table, err := NewTableService(db).GetTable(tableID, tokenID)
		if err != nil {
			return nil, err
		}
		fields, err := NewFieldService(db).ListFields(tableID, tokenID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"table_id":   table.ID,
			"table_name": table.Name,
			"fields":     fields,
		}, nil
	}

	if hasDB {
		database, err := NewDatabaseService(db).GetDatabase(databaseID, tokenID)
		if err != nil {
			return nil, err
		}
		tables, err := NewTableService(db).ListTables(databaseID, tokenID)
		if err != nil {
			return nil, err
		}
		return map[string]any{
			"database_id":   database.ID,
			"database_name": database.Name,
			"tables":        tables,
		}, nil
	}

	return nil, fmt.Errorf("either database_id or table_id required")
}

func executeCreateDatabase(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name required")
	}
	description, _ := args["description"].(string)

	database, err := NewDatabaseService(db).CreateDatabase(CreateDBRequest{
		Name:        name,
		Description: description,
	}, tokenID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":   database.ID,
		"name": database.Name,
	}, nil
}

func executeCreateTable(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	databaseID, ok := args["database_id"].(string)
	if !ok {
		return nil, fmt.Errorf("database_id required")
	}
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name required")
	}
	description, _ := args["description"].(string)

	table, err := NewTableService(db).CreateTable(CreateTableRequest{
		DatabaseID:  databaseID,
		Name:        name,
		Description: description,
	}, tokenID)
	if err != nil {
		return nil, err
	}

	fieldsRaw, hasFields := args["fields"]
	if hasFields {
		fieldsArr, ok := fieldsRaw.([]any)
		if ok {
			fieldService := NewFieldService(db)
			for _, f := range fieldsArr {
				fieldMap, ok := f.(map[string]any)
				if !ok {
					continue
				}
				fieldName, _ := fieldMap["name"].(string)
				fieldType, _ := fieldMap["type"].(string)
				fieldDesc, _ := fieldMap["description"].(string)
				fieldRequired, _ := fieldMap["required"].(bool)

				if fieldName == "" || fieldType == "" {
					continue
				}

				if _, err := fieldService.CreateField(CreateFieldRequest{
					TableID:     table.ID,
					Name:        fieldName,
					Type:        fieldType,
					Description: fieldDesc,
					Required:    fieldRequired,
				}, tokenID); err != nil {
					return nil, err
				}
			}
		}
	}

	return map[string]any{
		"id":   table.ID,
		"name": table.Name,
	}, nil
}

func executeCreateField(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	tableID, ok := args["table_id"].(string)
	if !ok {
		return nil, fmt.Errorf("table_id required")
	}
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name required")
	}
	fieldType, ok := args["type"].(string)
	if !ok {
		return nil, fmt.Errorf("type required")
	}
	description, _ := args["description"].(string)
	required, _ := args["required"].(bool)

	field, err := NewFieldService(db).CreateField(CreateFieldRequest{
		TableID:     tableID,
		Name:        name,
		Type:        fieldType,
		Description: description,
		Required:    required,
	}, tokenID)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"id":   field.ID,
		"name": field.Name,
		"type": field.Type,
	}, nil
}

func executeQuery(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	from, ok := args["from"].(string)
	if !ok {
		return nil, fmt.Errorf("from required")
	}

	req := &query.QueryRequest{
		From: from,
		Page: 1,
		Size: 100,
	}
	if selectFields, ok := args["select"].([]any); ok {
		req.Select = make([]string, 0, len(selectFields))
		for _, field := range selectFields {
			if fieldName, ok := field.(string); ok {
				req.Select = append(req.Select, fieldName)
			}
		}
	}
	if where, ok := args["where"].(map[string]any); ok {
		req.Where = &query.WhereClause{And: make([]query.Condition, 0, len(where))}
		for key, value := range where {
			req.Where.And = append(req.Where.And, query.Condition{
				Field: key,
				Op:    "eq",
				Value: value,
			})
		}
	}
	if limit, ok := args["limit"].(float64); ok && int(limit) > 0 {
		req.Size = int(limit)
	}
	if offset, ok := args["offset"].(float64); ok && req.Size > 0 {
		req.Page = int(offset)/req.Size + 1
	}

	result, err := query.NewExecutor(db).Execute(context.Background(), req, tokenID)
	if err != nil {
		return nil, err
	}
	return result.Data, nil
}

func executeInsertRecords(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	tableID, ok := args["table_id"].(string)
	if !ok {
		return nil, fmt.Errorf("table_id required")
	}
	recordsRaw, ok := args["records"].([]any)
	if !ok {
		return nil, fmt.Errorf("records required as array")
	}

	inserted := 0
	recordService := NewRecordService(db)
	for _, r := range recordsRaw {
		dataMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		if _, err := recordService.CreateRecord(CreateRecordRequest{
			TableID: tableID,
			Data:    dataMap,
		}, tokenID); err != nil {
			return nil, fmt.Errorf("insert record: %w", err)
		}
		inserted++
	}

	return map[string]any{
		"table_id": tableID,
		"inserted": inserted,
	}, nil
}

func executeUpdateRecord(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	recordID, ok := args["record_id"].(string)
	if !ok {
		return nil, fmt.Errorf("record_id required")
	}
	dataRaw, ok := args["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("data required")
	}

	record, err := NewRecordService(db).UpdateRecord(recordID, UpdateRecordRequest{
		Data: dataRaw,
	}, tokenID)
	if err != nil {
		return nil, fmt.Errorf("update record: %w", err)
	}

	return map[string]any{
		"id":      record.ID,
		"version": record.Version,
	}, nil
}

func executeDeleteRecord(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	recordID, ok := args["record_id"].(string)
	if !ok {
		return nil, fmt.Errorf("record_id required")
	}

	if err := NewRecordService(db).DeleteRecord(recordID, tokenID); err != nil {
		return nil, fmt.Errorf("delete record: %w", err)
	}

	return map[string]any{
		"id": recordID,
	}, nil
}

func executeGenerateTestData(db *gorm.DB, tokenID string, args map[string]any) (any, error) {
	tableID, ok := args["table_id"].(string)
	if !ok {
		return nil, fmt.Errorf("table_id required")
	}
	countRaw, ok := args["count"].(float64)
	if !ok {
		return nil, fmt.Errorf("count required")
	}

	records, err := NewRecordService(db).GenerateTestData(tableID, tokenID, int(countRaw))
	if err != nil {
		return nil, fmt.Errorf("insert test record: %w", err)
	}

	return map[string]any{
		"table_id": tableID,
		"inserted": len(records),
	}, nil
}

func generateFieldValue(rng *rand.Rand, fieldType string) any {
	switch normalizeFieldType(fieldType) {
	case "string", "link":
		names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
		return names[rng.Intn(len(names))]
	case "text", "json":
		sentences := []string{"Lorem ipsum dolor sit amet.", "Quick brown fox jumps over the lazy dog.", "The early bird catches the worm."}
		if normalizeFieldType(fieldType) == "json" {
			return map[string]any{"sample": sentences[rng.Intn(len(sentences))]}
		}
		return sentences[rng.Intn(len(sentences))]
	case "number":
		return float64(rng.Intn(10000)) / 100.0
	case "boolean":
		return rng.Intn(2) == 0
	case "date", "datetime":
		days := rng.Intn(365)
		t := time.Now().AddDate(0, 0, -days)
		if normalizeFieldType(fieldType) == "datetime" {
			return t.Format(time.RFC3339)
		}
		return t.Format("2006-01-02")
	case "select", "email", "url", "color":
		switch normalizeFieldType(fieldType) {
		case "email":
			return fmt.Sprintf("user%d@example.com", rng.Intn(1000))
		case "url":
			return fmt.Sprintf("https://example.com/item/%d", rng.Intn(1000))
		case "color":
			colors := []string{"#ff0000", "#00ff00", "#0000ff", "#ff9900"}
			return colors[rng.Intn(len(colors))]
		default:
			options := []string{"active", "pending", "completed", "cancelled"}
			return options[rng.Intn(len(options))]
		}
	case "multiselect", "list", "file":
		items := []string{"tag1", "tag2", "tag3", "tag4", "tag5"}
		count := 1 + rng.Intn(3)
		result := make([]string, count)
		for i := 0; i < count; i++ {
			result[i] = items[rng.Intn(len(items))]
		}
		return result
	case "rating":
		return 1 + rng.Intn(5)
	default:
		return "sample_value"
	}
}
