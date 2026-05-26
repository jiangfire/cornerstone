package services

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
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

func ExecuteAITool(name string, args map[string]any) (any, error) {
	db := pkgdb.DB()

	switch name {
	case "list_databases":
		var databases []DBResult
		if err := db.Table("databases").Select("id, name").Where("deleted_at IS NULL").Find(&databases).Error; err != nil {
			return nil, fmt.Errorf("query databases: %w", err)
		}
		return databases, nil

	case "list_tables":
		databaseID, ok := args["database_id"].(string)
		if !ok {
			return nil, fmt.Errorf("database_id required")
		}
		var tables []TableResult
		if err := db.Table("tables").Select("id, name").Where("database_id = ? AND deleted_at IS NULL", databaseID).Find(&tables).Error; err != nil {
			return nil, fmt.Errorf("query tables: %w", err)
		}
		return tables, nil

	case "get_schema":
		return executeGetSchema(db, args)

	case "create_database":
		return executeCreateDatabase(db, args)

	case "create_table":
		return executeCreateTable(db, args)

	case "create_field":
		return executeCreateField(db, args)

	case "execute_query":
		return executeQuery(db, args)

	case "insert_records":
		return executeInsertRecords(db, args)

	case "update_record":
		return executeUpdateRecord(db, args)

	case "delete_record":
		return executeDeleteRecord(db, args)

	case "generate_test_data":
		return executeGenerateTestData(db, args)

	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func executeGetSchema(db *gorm.DB, args map[string]any) (any, error) {
	tableID, hasTable := args["table_id"].(string)
	databaseID, hasDB := args["database_id"].(string)

	if hasTable {
		var table models.Table
		if err := db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error; err != nil {
			return nil, fmt.Errorf("table not found: %w", err)
		}
		var fields []models.Field
		if err := db.Where("table_id = ? AND deleted_at IS NULL", tableID).Find(&fields).Error; err != nil {
			return nil, fmt.Errorf("query fields: %w", err)
		}
		return map[string]any{
			"table_id":   table.ID,
			"table_name": table.Name,
			"fields":     fields,
		}, nil
	}

	if hasDB {
		var database models.Database
		if err := db.Where("id = ? AND deleted_at IS NULL", databaseID).First(&database).Error; err != nil {
			return nil, fmt.Errorf("database not found: %w", err)
		}
		var tables []models.Table
		if err := db.Where("database_id = ? AND deleted_at IS NULL", databaseID).Find(&tables).Error; err != nil {
			return nil, fmt.Errorf("query tables: %w", err)
		}
		return map[string]any{
			"database_id":   database.ID,
			"database_name": database.Name,
			"tables":        tables,
		}, nil
	}

	return nil, fmt.Errorf("either database_id or table_id required")
}

func executeCreateDatabase(db *gorm.DB, args map[string]any) (any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name required")
	}
	description, _ := args["description"].(string)

	database := models.Database{
		Name:        name,
		Description: description,
	}
	if err := db.Create(&database).Error; err != nil {
		return nil, fmt.Errorf("create database: %w", err)
	}
	return map[string]any{
		"id":   database.ID,
		"name": database.Name,
	}, nil
}

func executeCreateTable(db *gorm.DB, args map[string]any) (any, error) {
	databaseID, ok := args["database_id"].(string)
	if !ok {
		return nil, fmt.Errorf("database_id required")
	}
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name required")
	}
	description, _ := args["description"].(string)

	var database models.Database
	if err := db.Where("id = ? AND deleted_at IS NULL", databaseID).First(&database).Error; err != nil {
		return nil, fmt.Errorf("database not found: %w", err)
	}

	table := models.Table{
		DatabaseID:  databaseID,
		Name:        name,
		Description: description,
	}
	if err := db.Create(&table).Error; err != nil {
		return nil, fmt.Errorf("create table: %w", err)
	}

	fieldsRaw, hasFields := args["fields"]
	if hasFields {
		fieldsArr, ok := fieldsRaw.([]any)
		if ok {
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

				field := models.Field{
					TableID:     table.ID,
					Name:        fieldName,
					Type:        fieldType,
					Description: fieldDesc,
					Required:    fieldRequired,
				}
				_ = db.Create(&field)
			}
		}
	}

	return map[string]any{
		"id":   table.ID,
		"name": table.Name,
	}, nil
}

func executeCreateField(db *gorm.DB, args map[string]any) (any, error) {
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

	var table models.Table
	if err := db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error; err != nil {
		return nil, fmt.Errorf("table not found: %w", err)
	}

	field := models.Field{
		TableID:     tableID,
		Name:        name,
		Type:        fieldType,
		Description: description,
		Required:    required,
	}
	if err := db.Create(&field).Error; err != nil {
		return nil, fmt.Errorf("create field: %w", err)
	}
	return map[string]any{
		"id":   field.ID,
		"name": field.Name,
		"type": field.Type,
	}, nil
}

func executeQuery(db *gorm.DB, args map[string]any) (any, error) {
	from, ok := args["from"].(string)
	if !ok {
		return nil, fmt.Errorf("from required")
	}

	query := db.Table(from).Where("deleted_at IS NULL")

	if limit, ok := args["limit"].(float64); ok {
		query = query.Limit(int(limit))
	} else {
		query = query.Limit(100)
	}

	if offset, ok := args["offset"].(float64); ok {
		query = query.Offset(int(offset))
	}

	if where, ok := args["where"].(map[string]any); ok {
		for k, v := range where {
			query = query.Where(k+" = ?", v)
		}
	}

	var results []map[string]any
	if err := query.Find(&results).Error; err != nil {
		return nil, fmt.Errorf("query %s: %w", from, err)
	}
	return results, nil
}

func executeInsertRecords(db *gorm.DB, args map[string]any) (any, error) {
	tableID, ok := args["table_id"].(string)
	if !ok {
		return nil, fmt.Errorf("table_id required")
	}
	recordsRaw, ok := args["records"].([]any)
	if !ok {
		return nil, fmt.Errorf("records required as array")
	}

	var table models.Table
	if err := db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error; err != nil {
		return nil, fmt.Errorf("table not found: %w", err)
	}

	inserted := 0
	for _, r := range recordsRaw {
		dataMap, ok := r.(map[string]any)
		if !ok {
			continue
		}
		dataJSON, err := json.Marshal(dataMap)
		if err != nil {
			continue
		}
		record := models.Record{
			TableID: tableID,
			Data:    string(dataJSON),
			Version: 1,
		}
		if err := db.Create(&record).Error; err != nil {
			return nil, fmt.Errorf("insert record: %w", err)
		}
		inserted++
	}

	return map[string]any{
		"table_id": tableID,
		"inserted": inserted,
	}, nil
}

func executeUpdateRecord(db *gorm.DB, args map[string]any) (any, error) {
	recordID, ok := args["record_id"].(string)
	if !ok {
		return nil, fmt.Errorf("record_id required")
	}
	dataRaw, ok := args["data"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("data required")
	}

	var record models.Record
	if err := db.First(&record, "id = ?", recordID).Error; err != nil {
		return nil, fmt.Errorf("record not found: %w", err)
	}

	var existingData map[string]any
	_ = json.Unmarshal([]byte(record.Data), &existingData)
	for k, v := range dataRaw {
		existingData[k] = v
	}
	updatedJSON, _ := json.Marshal(existingData)

	record.Data = string(updatedJSON)
	record.Version++
	if err := db.Save(&record).Error; err != nil {
		return nil, fmt.Errorf("update record: %w", err)
	}

	return map[string]any{
		"id":      record.ID,
		"version": record.Version,
	}, nil
}

func executeDeleteRecord(db *gorm.DB, args map[string]any) (any, error) {
	recordID, ok := args["record_id"].(string)
	if !ok {
		return nil, fmt.Errorf("record_id required")
	}

	result := db.Model(&models.Record{}).Where("id = ? AND deleted_at IS NULL", recordID).Updates(map[string]any{
		"deleted_at": time.Now(),
	})
	if result.Error != nil {
		return nil, fmt.Errorf("delete record: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, fmt.Errorf("record not found")
	}

	return map[string]any{
		"id": recordID,
	}, nil
}

func executeGenerateTestData(db *gorm.DB, args map[string]any) (any, error) {
	tableID, ok := args["table_id"].(string)
	if !ok {
		return nil, fmt.Errorf("table_id required")
	}
	countRaw, ok := args["count"].(float64)
	if !ok {
		return nil, fmt.Errorf("count required")
	}
	count := int(countRaw)

	var table models.Table
	if err := db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error; err != nil {
		return nil, fmt.Errorf("table not found: %w", err)
	}

	var fields []models.Field
	if err := db.Where("table_id = ? AND deleted_at IS NULL", tableID).Find(&fields).Error; err != nil {
		return nil, fmt.Errorf("query fields: %w", err)
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	inserted := 0

	for i := 0; i < count; i++ {
		data := make(map[string]any)
		for _, f := range fields {
			data[f.Name] = generateFieldValue(rng, f.Type)
		}
		dataJSON, _ := json.Marshal(data)
		record := models.Record{
			TableID: tableID,
			Data:    string(dataJSON),
			Version: 1,
		}
		if err := db.Create(&record).Error; err != nil {
			return nil, fmt.Errorf("insert test record: %w", err)
		}
		inserted++
	}

	return map[string]any{
		"table_id": tableID,
		"inserted": inserted,
	}, nil
}

func generateFieldValue(rng *rand.Rand, fieldType string) any {
	switch fieldType {
	case "string":
		names := []string{"Alice", "Bob", "Charlie", "Diana", "Eve", "Frank", "Grace", "Henry"}
		return names[rng.Intn(len(names))]
	case "text":
		sentences := []string{"Lorem ipsum dolor sit amet.", "Quick brown fox jumps over the lazy dog.", "The early bird catches the worm."}
		return sentences[rng.Intn(len(sentences))]
	case "number":
		return float64(rng.Intn(10000)) / 100.0
	case "boolean":
		return rng.Intn(2) == 0
	case "date", "datetime":
		days := rng.Intn(365)
		t := time.Now().AddDate(0, 0, -days)
		if fieldType == "datetime" {
			return t.Format(time.RFC3339)
		}
		return t.Format("2006-01-02")
	case "select":
		options := []string{"active", "pending", "completed", "cancelled"}
		return options[rng.Intn(len(options))]
	case "list":
		items := []string{"tag1", "tag2", "tag3", "tag4", "tag5"}
		count := 1 + rng.Intn(3)
		result := make([]string, count)
		for i := 0; i < count; i++ {
			result[i] = items[rng.Intn(len(items))]
		}
		return result
	default:
		return "sample_value"
	}
}
