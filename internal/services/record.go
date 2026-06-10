package services

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"math/rand"
	"regexp"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	json "github.com/jiangfire/cornerstone/pkg/jsonx"
	"gorm.io/gorm"
)

func parseStringListValue(value interface{}) ([]string, error) {
	switch values := value.(type) {
	case []string:
		return values, nil
	case []interface{}:
		items := make([]string, 0, len(values))
		for _, item := range values {
			str, ok := item.(string)
			if !ok {
				return nil, errors.New("list field items must be strings, e.g. [\"admin\"]")
			}
			items = append(items, str)
		}
		return items, nil
	default:
		return nil, errors.New("list field requires an array of strings, e.g. [\"admin\"] or [\"option1\", \"option2\"]")
	}
}

func parseAttachmentValue(value interface{}) ([]string, error) {
	switch values := value.(type) {
	case string:
		if strings.TrimSpace(values) == "" {
			return []string{}, nil
		}
		return []string{values}, nil
	case []string:
		items := make([]string, 0, len(values))
		for _, item := range values {
			trimmed := strings.TrimSpace(item)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		return items, nil
	case []interface{}:
		items := make([]string, 0, len(values))
		for _, item := range values {
			str, ok := item.(string)
			if !ok {
				return nil, errors.New("attachment value must be a file ID or array of file IDs")
			}
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		return items, nil
	default:
		return nil, errors.New("attachment value must be a file ID or array of file IDs")
	}
}

// RecordService manages record operations
type RecordService struct {
	db *gorm.DB
}

// NewRecordService creates a new RecordService instance
func NewRecordService(db *gorm.DB) *RecordService {
	return &RecordService{
		db: db,
	}
}

// CreateRecordRequest is the request to create a record
type CreateRecordRequest struct {
	TableID string                 `json:"table_id" binding:"required"`
	Data    map[string]interface{} `json:"data" binding:"required"`
}

// UpdateRecordRequest is the request to update a record
type UpdateRecordRequest struct {
	Data    map[string]interface{} `json:"data" binding:"required"`
	Version int                    `json:"version"` // optimistic lock version
}

// RecordResponse is the record API response
type RecordResponse struct {
	ID      string      `json:"id"`
	TableID string      `json:"table_id"`
	Data    interface{} `json:"data"`
	Version int         `json:"version"`
}

// QueryRequest is the query request
type QueryRequest struct {
	TableID string `form:"table_id" binding:"required"`
	Limit   int    `form:"limit" binding:"min=1,max=100"`
	Offset  int    `form:"offset" binding:"min=0"`
	Filter  string `form:"filter"` // Supports JSON filter or keyword search
	Fields  string `form:"fields"` // Comma-separated field names to return in data
}

// QueryResponse is the query response
type QueryResponse struct {
	Records []RecordResponse `json:"records"`
	Total   int64            `json:"total"`
	HasMore bool             `json:"has_more"`
}

// checkTableAccess verifies table exists and user has access
func (s *RecordService) checkTableAccess(tableID, userID string, requiredRoles []string) error {
	var table models.Table
	err := s.db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error
	if err != nil {
		return errors.New("table not found")
	}

	var db models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", table.DatabaseID).First(&db).Error
	if err != nil {
		return errors.New("database not found")
	}

	authorizer, err := authz.NewAuthorizer(s.db, userID)
	if err != nil {
		return err
	}
	action := requiredActionForRoles(requiredRoles)
	if !authorizer.CanAccessTable(tableID, action) {
		return errors.New("permission denied: cannot access this table")
	}

	return nil
}

// validateRecordData validates record data against field definitions
func (s *RecordService) validateRecordData(tableID string, data map[string]interface{}, currentRecordID, userID string) error {
	// Get all field definitions for the table (cached)
	fields, err := s.getTableFields(tableID)
	if err != nil {
		return fmt.Errorf("failed to get field definitions: %w", err)
	}

	// Validate each field
	for _, field := range fields {
		// Support lookup by field ID or field name
		value, existsByID := data[field.ID]
		valueByName, existsByName := data[field.Name]

		// If not found by ID or name but field is required, report error
		if field.Required && !existsByID && !existsByName {
			return fmt.Errorf("field '%s' is required", field.Name)
		}

		// If field not present, skip validation
		if !existsByID && !existsByName {
			continue
		}

		// Prefer value found by name (if exists)
		if existsByName {
			value = valueByName
		}

		// For optional fields, skip validation if value is empty or nil
		if !field.Required && (value == nil || value == "") {
			continue
		}

		config := parseStoredFieldConfig(field.Options)

		if isAttachmentFieldType(field.Type) {
			if err := s.validateAttachmentFieldValue(field, config, value, currentRecordID, userID); err != nil {
				return fmt.Errorf("field '%s' validation failed: %w", field.Name, err)
			}
			continue
		}

		// Validate data based on field type
		if err := s.validateFieldValueWithConfig(field, config, value); err != nil {
			return fmt.Errorf("field '%s' validation failed: %w", field.Name, err)
		}
	}

	return nil
}

func (s *RecordService) validateAttachmentFieldValue(field models.Field, config FieldConfig, value interface{}, currentRecordID, userID string) error {
	fileIDs, err := parseAttachmentValue(value)
	if err != nil {
		return err
	}

	if !config.Multiple && len(fileIDs) > 1 {
		return errors.New("this attachment field only allows a single file")
	}

	seen := make(map[string]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		if _, exists := seen[fileID]; exists {
			return fmt.Errorf("duplicate attachment ID: %s", fileID)
		}
		seen[fileID] = struct{}{}

		file, _, err := NewFileService(s.db).getAccessibleFile(fileID, userID, []string{"owner", "admin", "editor"})
		if err != nil {
			return err
		}
		if file.FieldID != field.ID {
			return errors.New("attachment does not belong to this field")
		}
		if currentRecordID == "" {
			if file.RecordID != "" {
				return errors.New("can only reference unbound attachments when creating a record")
			}
		} else if file.RecordID != "" && file.RecordID != currentRecordID {
			return errors.New("attachment is already bound to another record")
		}
		if config.MaxFileSizeMB > 0 && file.FileSize > int64(config.MaxFileSizeMB)*1024*1024 {
			return fmt.Errorf("attachment exceeds field size limit (max %dMB)", config.MaxFileSizeMB)
		}
		if !fileMatchesAllowedTypes(file.FileName, file.FileType, config.AllowedTypes) {
			return errors.New("attachment type does not match field restrictions")
		}
	}

	return nil
}

func attachmentFieldIDsFromData(field models.Field, data map[string]interface{}) ([]string, error) {
	value, exists := data[field.Name]
	if !exists {
		return []string{}, nil
	}
	return parseAttachmentValue(value)
}

func (s *RecordService) syncAttachmentBindings(tx *gorm.DB, recordID string, fields []models.Field, data map[string]interface{}) error {
	for _, field := range fields {
		if !isAttachmentFieldType(field.Type) {
			continue
		}

		fileIDs, err := attachmentFieldIDsFromData(field, data)
		if err != nil {
			return fmt.Errorf("failed to sync attachments for field %s: %w", field.Name, err)
		}

		referenced := make(map[string]struct{}, len(fileIDs))
		for _, fileID := range fileIDs {
			referenced[fileID] = struct{}{}
		}

		var existingFiles []models.File
		if err := tx.Where("record_id = ? AND field_id = ?", recordID, field.ID).Find(&existingFiles).Error; err != nil {
			return fmt.Errorf("failed to query attachment bindings: %w", err)
		}

		for _, existingFile := range existingFiles {
			if _, ok := referenced[existingFile.ID]; ok {
				continue
			}
			if err := tx.Model(&models.File{}).Where("id = ?", existingFile.ID).Update("record_id", "").Error; err != nil {
				return fmt.Errorf("failed to unbind attachment: %w", err)
			}
		}

		if len(fileIDs) == 0 {
			continue
		}

		if err := tx.Model(&models.File{}).
			Where("id IN ?", fileIDs).
			Updates(map[string]interface{}{
				"record_id": recordID,
				"field_id":  field.ID,
			}).Error; err != nil {
			return fmt.Errorf("failed to bind attachment: %w", err)
		}
	}

	return nil
}

// validateFieldValue validates a field value (compatibility wrapper, re-parses config internally)
func (s *RecordService) validateFieldValue(field models.Field, value interface{}) error {
	config := parseStoredFieldConfig(field.Options)
	return s.validateFieldValueWithConfig(field, config, value)
}

// validateFieldValueWithConfig validates a field value with pre-parsed config, avoiding repeated Unmarshal
func (s *RecordService) validateFieldValueWithConfig(field models.Field, config FieldConfig, value interface{}) error {
	// Handle nil values - these should be handled by the caller, but we'll be defensive
	if value == nil {
		return nil
	}

	switch normalizeFieldType(field.Type) {
	case "string", "text":
		if _, ok := value.(string); !ok {
			return errors.New("expected string type")
		}

		strValue := value.(string)

		// Skip further validation for empty strings (length and regex)
		if strValue == "" {
			return nil
		}

		// Use pre-parsed config
		if config.MaxLength != nil && len(strValue) > *config.MaxLength {
			return fmt.Errorf("length must not exceed %d characters", *config.MaxLength)
		}
		if config.Validation != "" {
			matched, err := regexp.MatchString(config.Validation, strValue)
			if err != nil {
				return fmt.Errorf("invalid regex: %w", err)
			}
			if !matched {
				return fmt.Errorf("format mismatch, expected: %s", config.Validation)
			}
		}

	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64, json.Number:
			// OK
		default:
			return errors.New("expected number type")
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("expected boolean type")
		}

	case "date", "datetime":
		strValue, ok := value.(string)
		if !ok {
			return errors.New("expected string type (date format)")
		}
		var layouts []string
		if field.Type == "date" {
			layouts = []string{"2006-01-02", time.RFC3339}
		} else {
			layouts = []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05"}
		}
		parsed := false
		for _, layout := range layouts {
			if _, err := time.Parse(layout, strValue); err == nil {
				parsed = true
				break
			}
		}
		if !parsed {
			return fmt.Errorf("invalid date format: %s", strValue)
		}

	case "list":
		_, err := parseStringListValue(value)
		if err != nil {
			return err
		}
	case "json":
		if strValue, ok := value.(string); ok {
			var dummy interface{}
			if err := json.Unmarshal([]byte(strValue), &dummy); err != nil {
				return fmt.Errorf("invalid JSON string: %w", err)
			}
			return nil
		}
		if _, err := json.Marshal(value); err != nil {
			return fmt.Errorf("invalid JSON value: %w", err)
		}
		return nil
	}

	return nil
}

func (s *RecordService) getTableFields(tableID string) ([]models.Field, error) {
	if fields, ok := SharedFieldCache.Get(tableID); ok {
		return fields, nil
	}
	var fields []models.Field
	if err := s.db.Where("table_id = ? AND deleted_at IS NULL", tableID).
		Order("created_at ASC").
		Find(&fields).Error; err != nil {
		return nil, fmt.Errorf("failed to get field definitions: %w", err)
	}
	SharedFieldCache.Set(tableID, fields)
	return fields, nil
}

func (s *RecordService) extractKnownRecordData(fields []models.Field, data map[string]interface{}) (map[string]interface{}, map[string]struct{}) {
	normalized := make(map[string]interface{}, len(data))
	matchedKeys := make(map[string]struct{}, len(data))

	for _, field := range fields {
		if value, exists := data[field.Name]; exists {
			normalized[field.Name] = value
			matchedKeys[field.Name] = struct{}{}
			continue
		}
		if value, exists := data[field.ID]; exists {
			normalized[field.Name] = value
			matchedKeys[field.ID] = struct{}{}
		}
	}

	return normalized, matchedKeys
}

func (s *RecordService) normalizeRecordData(fields []models.Field, data map[string]interface{}) (map[string]interface{}, error) {
	normalized, matchedKeys := s.extractKnownRecordData(fields, data)
	if len(matchedKeys) != len(data) {
		for key := range data {
			if _, ok := matchedKeys[key]; !ok {
				return nil, fmt.Errorf("field '%s' does not exist", key)
			}
		}
	}
	return normalized, nil
}

func (s *RecordService) getFieldAccessMaps(fields []models.Field, userID string) (map[string]models.Field, map[string]models.Field, error) {
	readableFields := make(map[string]models.Field, len(fields))
	writableFields := make(map[string]models.Field, len(fields))

	fieldIDs := make([]string, len(fields))
	for i, f := range fields {
		fieldIDs[i] = f.ID
	}

	fieldService := NewFieldService(s.db)
	readResults, err := fieldService.CheckFieldPermissions(userID, fieldIDs, "read")
	if err != nil {
		return nil, nil, err
	}
	writeResults, err := fieldService.CheckFieldPermissions(userID, fieldIDs, "write")
	if err != nil {
		return nil, nil, err
	}

	for _, field := range fields {
		if readResults[field.ID] {
			readableFields[field.Name] = field
		}
		if writeResults[field.ID] {
			writableFields[field.Name] = field
		}
	}

	return readableFields, writableFields, nil
}

func (s *RecordService) ensureWritableFields(data map[string]interface{}, writableFields map[string]models.Field) error {
	for fieldName := range data {
		if _, ok := writableFields[fieldName]; !ok {
			return fmt.Errorf("write permission denied for field '%s'", fieldName)
		}
	}
	return nil
}

func parseRecordPayload(raw models.JSONField) map[string]interface{} {
	payload := make(map[string]interface{})
	if raw == "" {
		return payload
	}
	_ = json.UnmarshalString(string(raw), &payload)
	return payload
}

func filterDataFields(data map[string]interface{}, fields string) map[string]interface{} {
	if fields == "" {
		return data
	}
	keep := make(map[string]struct{})
	for _, f := range splitAndTrim(fields, ",") {
		keep[f] = struct{}{}
	}
	filtered := make(map[string]interface{}, len(keep))
	for k, v := range data {
		if _, ok := keep[k]; ok {
			filtered[k] = v
		}
	}
	return filtered
}

func splitAndTrim(s, sep string) []string {
	parts := strings.Split(s, sep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

func (s *RecordService) filterReadableData(fields []models.Field, readableFields map[string]models.Field, payload map[string]interface{}) map[string]interface{} {
	filtered := make(map[string]interface{})
	for _, field := range fields {
		if _, ok := readableFields[field.Name]; !ok {
			continue
		}
		if value, exists := payload[field.Name]; exists {
			filtered[field.Name] = value
			continue
		}
		if value, exists := payload[field.ID]; exists {
			filtered[field.Name] = value
		}
	}
	return filtered
}

func marshalRecordPayload(payload map[string]interface{}) (models.JSONField, error) {
	dataJSON, err := json.MarshalString(payload)
	if err != nil {
		return "", fmt.Errorf("data serialization failed: %w", err)
	}
	return models.JSONField(dataJSON), nil
}

// recordFilterClause is a reusable WHERE fragment used for both paginated queries and COUNT.
type recordFilterClause struct {
	sql         string
	args        []interface{}
	indexFilter *recordFieldIndexFilter
}

type recordFieldIndexFilter struct {
	fieldID   string
	valueType string
	value     interface{}
}

const maxRecordFieldIndexTextLength = 512

func buildRecordFieldIndexRows(tableID, recordID string, fields []models.Field, data map[string]interface{}) ([]models.RecordFieldIndex, error) {
	rows := make([]models.RecordFieldIndex, 0, len(fields))
	for _, field := range fields {
		value, exists := data[field.Name]
		if !exists || value == nil {
			continue
		}
		row, ok, err := buildRecordFieldIndexRow(tableID, recordID, field, value)
		if err != nil {
			return nil, err
		}
		if ok {
			rows = append(rows, row)
		}
	}
	return rows, nil
}

func buildRecordFieldIndexRow(tableID, recordID string, field models.Field, value interface{}) (models.RecordFieldIndex, bool, error) {
	row := models.RecordFieldIndex{
		TableID:   tableID,
		RecordID:  recordID,
		FieldID:   field.ID,
		FieldName: field.Name,
	}

	switch normalizeFieldType(field.Type) {
	case "string", "text", "date", "datetime":
		text, ok := value.(string)
		if !ok || len(text) > maxRecordFieldIndexTextLength {
			return models.RecordFieldIndex{}, false, nil
		}
		row.ValueType = "text"
		row.ValueText = text
		return row, true, nil
	case "number":
		number, ok := recordFieldIndexNumber(value)
		if !ok {
			return models.RecordFieldIndex{}, false, nil
		}
		row.ValueType = "number"
		row.ValueNumber = &number
		return row, true, nil
	case "boolean":
		boolean, ok := value.(bool)
		if !ok {
			return models.RecordFieldIndex{}, false, nil
		}
		row.ValueType = "bool"
		row.ValueBool = &boolean
		return row, true, nil
	case "json":
		text, ok, err := recordFieldIndexJSONText(value)
		if err != nil || !ok {
			return models.RecordFieldIndex{}, ok, err
		}
		row.ValueType = "text"
		row.ValueText = text
		return row, true, nil
	default:
		return models.RecordFieldIndex{}, false, nil
	}
}

func recordFieldIndexNumber(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int32:
		return float64(v), true
	case int64:
		return float64(v), true
	case json.Number:
		n, err := v.Float64()
		return n, err == nil
	default:
		return 0, false
	}
}

func recordFieldIndexJSONText(value interface{}) (string, bool, error) {
	if str, ok := value.(string); ok {
		if len(str) > maxRecordFieldIndexTextLength {
			return "", false, nil
		}
		return str, true, nil
	}
	encoded, err := json.MarshalString(value)
	if err != nil {
		return "", false, fmt.Errorf("field index serialization failed: %w", err)
	}
	if len(encoded) > maxRecordFieldIndexTextLength {
		return "", false, nil
	}
	return encoded, true, nil
}

func mysqlRecordFieldIndexClause(field models.Field, value interface{}) (recordFilterClause, bool, error) {
	if normalizeFieldType(field.Type) == "json" {
		return recordFilterClause{}, false, nil
	}

	row, ok, err := buildRecordFieldIndexRow("", "", field, value)
	if err != nil || !ok {
		return recordFilterClause{}, ok, err
	}

	switch row.ValueType {
	case "text":
		return recordFilterClause{
			sql: "EXISTS (SELECT 1 FROM record_field_indexes rfi WHERE rfi.record_id = records.id AND rfi.table_id = records.table_id AND rfi.deleted_at IS NULL AND rfi.field_id = ? AND rfi.value_text = ?)",
			args: []interface{}{
				field.ID,
				row.ValueText,
			},
			indexFilter: &recordFieldIndexFilter{
				fieldID:   field.ID,
				valueType: row.ValueType,
				value:     row.ValueText,
			},
		}, true, nil
	case "number":
		return recordFilterClause{
			sql: "EXISTS (SELECT 1 FROM record_field_indexes rfi WHERE rfi.record_id = records.id AND rfi.table_id = records.table_id AND rfi.deleted_at IS NULL AND rfi.field_id = ? AND rfi.value_number = ?)",
			args: []interface{}{
				field.ID,
				*row.ValueNumber,
			},
			indexFilter: &recordFieldIndexFilter{
				fieldID:   field.ID,
				valueType: row.ValueType,
				value:     *row.ValueNumber,
			},
		}, true, nil
	case "bool":
		return recordFilterClause{
			sql: "EXISTS (SELECT 1 FROM record_field_indexes rfi WHERE rfi.record_id = records.id AND rfi.table_id = records.table_id AND rfi.deleted_at IS NULL AND rfi.field_id = ? AND rfi.value_bool = ?)",
			args: []interface{}{
				field.ID,
				*row.ValueBool,
			},
			indexFilter: &recordFieldIndexFilter{
				fieldID:   field.ID,
				valueType: row.ValueType,
				value:     *row.ValueBool,
			},
		}, true, nil
	default:
		return recordFilterClause{}, false, nil
	}
}

func (s *RecordService) syncRecordFieldIndexes(tx *gorm.DB, recordID, tableID string, fields []models.Field, data map[string]interface{}) error {
	if err := tx.Model(&models.RecordFieldIndex{}).
		Where("record_id = ? AND deleted_at IS NULL", recordID).
		Update("deleted_at", time.Now()).Error; err != nil {
		return fmt.Errorf("failed to clear record field indexes: %w", err)
	}

	rows, err := buildRecordFieldIndexRows(tableID, recordID, fields, data)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("failed to write record field indexes: %w", err)
	}
	return nil
}

func buildMySQLRecordListSQL(req QueryRequest, clauses []recordFilterClause) (string, []interface{}) {
	if filters, ok := collectMySQLRecordFieldIndexFilters(clauses); ok {
		return buildMySQLRecordFieldIndexListSQL(req, filters)
	}

	var b strings.Builder
	b.WriteString("SELECT id, table_id, data, version, created_at, updated_at FROM records FORCE INDEX (idx_records_table_deleted_created) ")
	b.WriteString("WHERE table_id = ? AND deleted_at IS NULL")

	args := make([]interface{}, 0, 3+len(clauses)*2)
	args = append(args, req.TableID)
	for _, clause := range clauses {
		b.WriteString(" AND ")
		b.WriteString(clause.sql)
		args = append(args, clause.args...)
	}

	b.WriteString(" ORDER BY created_at DESC LIMIT ? OFFSET ?")
	args = append(args, req.Limit, req.Offset)
	return b.String(), args
}

func buildMySQLRecordCountSQL(tableID string, clauses []recordFilterClause) (string, []interface{}) {
	if filters, ok := collectMySQLRecordFieldIndexFilters(clauses); ok {
		return buildMySQLRecordFieldIndexCountSQL(tableID, filters)
	}

	var b strings.Builder
	b.WriteString("SELECT COUNT(*) FROM records FORCE INDEX (idx_records_table_deleted_created) ")
	b.WriteString("WHERE table_id = ? AND deleted_at IS NULL")

	args := make([]interface{}, 0, 1+len(clauses)*2)
	args = append(args, tableID)
	for _, clause := range clauses {
		b.WriteString(" AND ")
		b.WriteString(clause.sql)
		args = append(args, clause.args...)
	}

	return b.String(), args
}

func collectMySQLRecordFieldIndexFilters(clauses []recordFilterClause) ([]recordFieldIndexFilter, bool) {
	if len(clauses) == 0 {
		return nil, false
	}
	filters := make([]recordFieldIndexFilter, 0, len(clauses))
	for _, clause := range clauses {
		if clause.indexFilter == nil {
			return nil, false
		}
		filters = append(filters, *clause.indexFilter)
	}
	return filters, true
}

func buildMySQLRecordFieldIndexListSQL(req QueryRequest, filters []recordFieldIndexFilter) (string, []interface{}) {
	subquery, args := buildMySQLRecordFieldIndexMatchedSubquery(req.TableID, filters)

	var b strings.Builder
	b.WriteString("SELECT records.id, records.table_id, records.data, records.version, records.created_at, records.updated_at ")
	b.WriteString("FROM (")
	b.WriteString(subquery)
	b.WriteString(") matched JOIN records FORCE INDEX (PRIMARY) ON records.id = matched.record_id ")
	b.WriteString("WHERE records.table_id = ? AND records.deleted_at IS NULL ")
	b.WriteString("ORDER BY records.created_at DESC LIMIT ? OFFSET ?")

	args = append(args, req.TableID, req.Limit, req.Offset)
	return b.String(), args
}

func buildMySQLRecordFieldIndexCountSQL(tableID string, filters []recordFieldIndexFilter) (string, []interface{}) {
	subquery, args := buildMySQLRecordFieldIndexMatchedSubquery(tableID, filters)

	var b strings.Builder
	b.WriteString("SELECT COUNT(*) FROM (")
	b.WriteString(subquery)
	b.WriteString(") matched JOIN records FORCE INDEX (PRIMARY) ON records.id = matched.record_id ")
	b.WriteString("WHERE records.table_id = ? AND records.deleted_at IS NULL")

	args = append(args, tableID)
	return b.String(), args
}

func buildMySQLRecordFieldIndexMatchedSubquery(tableID string, filters []recordFieldIndexFilter) (string, []interface{}) {
	var b strings.Builder
	b.WriteString("SELECT record_id FROM (")

	args := make([]interface{}, 0, 1+len(filters)*3)
	for i, filter := range filters {
		if i > 0 {
			b.WriteString(" UNION ALL ")
		}
		b.WriteString("SELECT record_id, field_id FROM record_field_indexes ")
		b.WriteString("WHERE table_id = ? AND deleted_at IS NULL AND field_id = ? AND ")
		switch filter.valueType {
		case "text":
			b.WriteString("value_text = ?")
		case "number":
			b.WriteString("value_number = ?")
		case "bool":
			b.WriteString("value_bool = ?")
		default:
			b.WriteString("1 = 0")
		}
		args = append(args, tableID, filter.fieldID, filter.value)
	}
	b.WriteString(") rfi_matches GROUP BY record_id HAVING COUNT(DISTINCT field_id) = ?")
	args = append(args, len(filters))

	return b.String(), args
}

func (s *RecordService) findRecordPage(req QueryRequest, clauses []recordFilterClause) ([]models.Record, error) {
	var records []models.Record
	if s.db.Name() == "mysql" {
		sql, args := buildMySQLRecordListSQL(req, clauses)
		if err := s.db.Raw(sql, args...).Scan(&records).Error; err != nil {
			return nil, err
		}
		return records, nil
	}

	query := s.db.Where("table_id = ? AND deleted_at IS NULL", req.TableID)
	for _, clause := range clauses {
		query = query.Where(clause.sql, clause.args...)
	}
	if err := query.Order("created_at DESC").Limit(req.Limit).Offset(req.Offset).Find(&records).Error; err != nil {
		return nil, err
	}
	return records, nil
}

func (s *RecordService) countRecords(tableID string, clauses []recordFilterClause) (int64, error) {
	var total int64
	if s.db.Name() == "mysql" {
		sql, args := buildMySQLRecordCountSQL(tableID, clauses)
		if err := s.db.Raw(sql, args...).Scan(&total).Error; err != nil {
			return 0, err
		}
		return total, nil
	}

	query := s.db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", tableID)
	for _, clause := range clauses {
		query = query.Where(clause.sql, clause.args...)
	}
	if err := query.Count(&total).Error; err != nil {
		return 0, err
	}
	return total, nil
}

// tryParseStructuredFilter parses the filter string as a JSON object.
// Callers should use the structured push-down path only when it returns true; otherwise treat as keyword.
func tryParseStructuredFilter(filter string) (map[string]interface{}, bool) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil, false
	}
	var structured map[string]interface{}
	if err := json.UnmarshalString(filter, &structured); err != nil {
		return nil, false
	}
	if len(structured) == 0 {
		return nil, false
	}
	return structured, true
}

// buildStructuredFilterClauses translates JSON filter conditions into push-down SQL WHERE fragments based on visible fields.
//
// Returns:
//
//	clauses          : (sql, args) applied to GORM query; field names passed via parameterized placeholders, not interpolated into SQL
//	refsHiddenField  : true if any filter key references a hidden/unknown field; caller should return empty results (aligned with
//	                  in-memory permission-aware filtering, preventing side-channel detection of hidden field values via 200 vs 400)
//	err              : structural errors such as value serialization failure, returned as 4xx to the client
func (s *RecordService) buildStructuredFilterClauses(
	fields []models.Field,
	readableFields map[string]models.Field,
	structured map[string]interface{},
) ([]recordFilterClause, bool, error) {
	return s.buildStructuredFilterClausesForDB(s.db.Name(), fields, readableFields, structured)
}

func (s *RecordService) buildStructuredFilterClausesForDB(
	dbType string,
	fields []models.Field,
	readableFields map[string]models.Field,
	structured map[string]interface{},
) ([]recordFilterClause, bool, error) {
	clauses := make([]recordFilterClause, 0, len(structured))

	for key, value := range structured {
		field, ok := resolveReadableFilterField(fields, readableFields, key)
		if !ok {
			return nil, true, nil
		}
		fieldName := field.Name

		jsonValue, err := json.Marshal(value)
		if err != nil {
			return nil, false, fmt.Errorf("filter value serialization failed: %w", err)
		}

		if dbType == "postgres" {
			// PG: Build {"<field>":<value>} literal and pass as jsonb parameter,
			// field name is escaped via json.Marshal for safe Unicode and no conflict with SQL placeholders.
			filterDoc, err := json.Marshal(map[string]interface{}{fieldName: value})
			if err != nil {
				return nil, false, fmt.Errorf("filter condition serialization failed: %w", err)
			}
			clauses = append(clauses, recordFilterClause{
				sql:  "data @> ?",
				args: []interface{}{string(filterDoc)},
			})
		} else {
			// SQLite / MySQL: JSON_EXTRACT
			var scalar interface{}
			if err := json.Unmarshal(jsonValue, &scalar); err != nil {
				return nil, false, fmt.Errorf("invalid filter value format: %w", err)
			}
			if dbType == "mysql" {
				indexClause, ok, err := mysqlRecordFieldIndexClause(field, scalar)
				if err != nil {
					return nil, false, err
				}
				if ok {
					clauses = append(clauses, indexClause)
					continue
				}
			}
			clauses = append(clauses, recordFilterClause{
				sql:  "JSON_EXTRACT(data, ?) = ?",
				args: []interface{}{"$." + fieldName, scalar},
			})
		}
	}
	return clauses, false, nil
}

func jsonValuesEqual(actual, expected interface{}) bool {
	actualJSON, err := json.Marshal(actual)
	if err != nil {
		return false
	}
	expectedJSON, err := json.Marshal(expected)
	if err != nil {
		return false
	}
	return bytes.Equal(actualJSON, expectedJSON)
}

func resolveReadableFilterField(fields []models.Field, readableFields map[string]models.Field, filterKey string) (models.Field, bool) {
	if field, ok := readableFields[filterKey]; ok {
		return field, true
	}

	for _, field := range fields {
		if field.ID != filterKey {
			continue
		}
		if _, ok := readableFields[field.Name]; !ok {
			return models.Field{}, false
		}
		return field, true
	}

	return models.Field{}, false
}

func (s *RecordService) matchesRecordFilter(fields []models.Field, readableFields map[string]models.Field, payload map[string]interface{}, filter string) (bool, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true, nil
	}

	var structuredFilter map[string]interface{}
	if err := json.Unmarshal([]byte(filter), &structuredFilter); err == nil && len(structuredFilter) > 0 {
		for filterKey, expected := range structuredFilter {
			field, ok := resolveReadableFilterField(fields, readableFields, filterKey)
			if !ok {
				return false, nil
			}

			actual, exists := payload[field.Name]
			if !exists || !jsonValuesEqual(actual, expected) {
				return false, nil
			}
		}
		return true, nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("record filtering failed: %w", err)
	}
	return strings.Contains(strings.ToLower(string(payloadJSON)), strings.ToLower(filter)), nil
}

func (s *RecordService) filterRecordsByReadablePayload(records []models.Record, fields []models.Field, readableFields map[string]models.Field, filter string) ([]models.Record, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return records, nil
	}

	filtered := make([]models.Record, 0, len(records))
	for _, record := range records {
		payload := s.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))
		matched, err := s.matchesRecordFilter(fields, readableFields, payload, filter)
		if err != nil {
			return nil, err
		}
		if matched {
			filtered = append(filtered, record)
		}
	}

	return filtered, nil
}

// CreateRecord creates a new record
func (s *RecordService) CreateRecord(req CreateRecordRequest, userID string) (*models.Record, error) {
	// 1. Check table access (owner, admin, editor can create records)
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	fields, err := s.getTableFields(req.TableID)
	if err != nil {
		return nil, err
	}
	readableFields, writableFields, err := s.getFieldAccessMaps(fields, userID)
	if err != nil {
		return nil, err
	}

	normalizedData, err := s.normalizeRecordData(fields, req.Data)
	if err != nil {
		return nil, err
	}
	if err := s.ensureWritableFields(normalizedData, writableFields); err != nil {
		return nil, err
	}

	// 2. Validate data
	if err := s.validateRecordData(req.TableID, normalizedData, "", userID); err != nil {
		return nil, err
	}

	// 3. Serialize data
	dataJSON, err := json.MarshalString(normalizedData)
	if err != nil {
		return nil, fmt.Errorf("data serialization failed: %w", err)
	}

	// 4. Create record and bind attachments
	record := models.Record{
		TableID: req.TableID,
		Data:    models.JSONField(dataJSON),
		Version: 1,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("failed to create record: %w", err)
		}
		if err := s.syncAttachmentBindings(tx, record.ID, fields, normalizedData); err != nil {
			return err
		}
		if err := s.syncRecordFieldIndexes(tx, record.ID, record.TableID, fields, normalizedData); err != nil {
			return err
		}
		return nil
	}); err != nil {
		return nil, err
	}

	filteredData := s.filterReadableData(fields, readableFields, normalizedData)
	record.Data, err = marshalRecordPayload(filteredData)
	if err != nil {
		return nil, err
	}

	// Reload to get database-generated timestamps
	if err := s.db.First(&record, "id = ?", record.ID).Error; err != nil {
		return nil, fmt.Errorf("failed to reload record: %w", err)
	}

	return &record, nil
}

// maxKeywordScanRecords is the max row limit for in-memory keyword fallback scanning.
// Exceeding this limit rejects the query and suggests using the /query endpoint to avoid loading the entire table into memory.
// Declared as var for test replacement; production code should not modify.
var maxKeywordScanRecords = 5000

// ListRecords lists records with query and pagination support
func (s *RecordService) ListRecords(req QueryRequest, userID string) (*QueryResponse, error) {
	// 1. Check table access
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	fields, err := s.getTableFields(req.TableID)
	if err != nil {
		return nil, err
	}
	readableFields, _, err := s.getFieldAccessMaps(fields, userID)
	if err != nil {
		return nil, err
	}

	// 2. Set defaults
	if req.Limit == 0 {
		req.Limit = 20
	}

	var records []models.Record
	var total int64
	filter := strings.TrimSpace(req.Filter)

	switch filter {
	case "":
		// 3a. No filter: SQL pagination + COUNT
		records, err = s.findRecordPage(req, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to query records: %w", err)
		}
		total, err = s.countRecords(req.TableID, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to count records: %w", err)
		}

	default:
		structured, isStructured := tryParseStructuredFilter(filter)
		if isStructured {
			// 3b. Structured JSON filter: translate to parameterized WHERE, push down to SQL, COUNT in sync
			clauses, refsHidden, err := s.buildStructuredFilterClauses(fields, readableFields, structured)
			if err != nil {
				return nil, err
			}
			if refsHidden {
				// When referencing hidden/unknown fields, return empty results,
				// preventing side-channel detection of hidden field values via 200 vs 400
				return &QueryResponse{Records: []RecordResponse{}, Total: 0, HasMore: false}, nil
			}

			records, err = s.findRecordPage(req, clauses)
			if err != nil {
				return nil, fmt.Errorf("failed to query records: %w", err)
			}
			total, err = s.countRecords(req.TableID, clauses)
			if err != nil {
				return nil, fmt.Errorf("failed to count records: %w", err)
			}
		} else {
			// 3c. Keyword fallback: pre-filter with SQL LIKE (limit +1 for overflow detection),
			// then do permission-aware in-memory filtering to prevent hidden field leakage via fuzzy matching
			likePattern := "%" + filter + "%"
			var likeSQL string
			if s.db.Name() == "postgres" {
				likeSQL = "table_id = ? AND deleted_at IS NULL AND data::text LIKE ?"
			} else {
				likeSQL = "table_id = ? AND deleted_at IS NULL AND data LIKE ?"
			}
			narrowQ := s.db.Where(likeSQL, req.TableID, likePattern).
				Order("created_at DESC").Limit(maxKeywordScanRecords + 1)
			var narrowed []models.Record
			if err := narrowQ.Find(&narrowed).Error; err != nil {
				return nil, fmt.Errorf("failed to query records: %w", err)
			}
			if len(narrowed) > maxKeywordScanRecords {
				return nil, fmt.Errorf("keyword filter matched too many records (>%d), use a more specific filter or the /query endpoint", maxKeywordScanRecords)
			}
			filtered, err := s.filterRecordsByReadablePayload(narrowed, fields, readableFields, filter)
			if err != nil {
				return nil, err
			}
			total = int64(len(filtered))
			if req.Offset >= len(filtered) {
				records = []models.Record{}
			} else {
				end := req.Offset + req.Limit
				if end > len(filtered) {
					end = len(filtered)
				}
				records = filtered[req.Offset:end]
			}
		}
	}

	// 7. Convert to response format
	result := make([]RecordResponse, len(records))
	for i, r := range records {
		data := s.filterReadableData(fields, readableFields, parseRecordPayload(r.Data))
		data = filterDataFields(data, req.Fields)

		result[i] = RecordResponse{
			ID:      r.ID,
			TableID: r.TableID,
			Data:    data,
			Version: r.Version,
		}
	}

	return &QueryResponse{
		Records: result,
		Total:   total,
		HasMore: int64(req.Offset+len(records)) < total,
	}, nil
}

func stringifyExportValue(value interface{}) string {
	if value == nil {
		return ""
	}

	switch v := value.(type) {
	case string:
		return v
	case float64, float32, int, int64, int32, bool:
		return fmt.Sprintf("%v", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}

// ExportRecords exports record data
func (s *RecordService) ExportRecords(tableID, userID, format, filter string) ([]byte, string, string, error) {
	if err := s.checkTableAccess(tableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, "", "", err
	}

	fields, err := s.getTableFields(tableID)
	if err != nil {
		return nil, "", "", err
	}
	readableFields, _, err := s.getFieldAccessMaps(fields, userID)
	if err != nil {
		return nil, "", "", err
	}

	exportFields := make([]models.Field, 0, len(fields))
	for _, field := range fields {
		if _, ok := readableFields[field.Name]; ok {
			exportFields = append(exportFields, field)
		}
	}

	query := s.db.Where("table_id = ? AND deleted_at IS NULL", tableID).Order("created_at DESC")
	var records []models.Record
	if err := query.Find(&records).Error; err != nil {
		return nil, "", "", fmt.Errorf("failed to read records: %w", err)
	}
	records, err = s.filterRecordsByReadablePayload(records, fields, readableFields, filter)
	if err != nil {
		return nil, "", "", err
	}

	switch strings.ToLower(format) {
	case "json":
		exportRows := make([]map[string]interface{}, 0, len(records))
		for _, record := range records {
			row := map[string]interface{}{
				"id":       record.ID,
				"table_id": record.TableID,
				"version":  record.Version,
			}

			payload := s.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))
			row["data"] = payload
			exportRows = append(exportRows, row)
		}

		data, err := json.MarshalIndent(exportRows, "", "  ")
		if err != nil {
			return nil, "", "", fmt.Errorf("failed to export JSON: %w", err)
		}

		filename := fmt.Sprintf("records_%s_%s.json", tableID, time.Now().Format("20060102150405"))
		return data, "application/json; charset=utf-8", filename, nil

	case "csv":
		var buf bytes.Buffer
		writer := csv.NewWriter(&buf)

		header := []string{"id"}
		for _, field := range exportFields {
			header = append(header, field.Name)
		}
		header = append(header, "version")
		if err := writer.Write(header); err != nil {
			return nil, "", "", fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, record := range records {
			row := []string{record.ID}
			payload := s.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))

			for _, field := range exportFields {
				value := payload[field.Name]
				row = append(row, stringifyExportValue(value))
			}

			row = append(row,
				fmt.Sprintf("%d", record.Version),
			)

			if err := writer.Write(row); err != nil {
				return nil, "", "", fmt.Errorf("failed to write CSV data: %w", err)
			}
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			return nil, "", "", fmt.Errorf("failed to generate CSV: %w", err)
		}

		filename := fmt.Sprintf("records_%s_%s.csv", tableID, time.Now().Format("20060102150405"))
		return buf.Bytes(), "text/csv; charset=utf-8", filename, nil

	default:
		return nil, "", "", errors.New("unsupported export format, only csv/json are supported")
	}
}

// GetRecord gets a single record
func (s *RecordService) GetRecord(recordID, userID, fieldFilter string) (*RecordResponse, error) {
	// 1. Get record
	var record models.Record
	err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error
	if err != nil {
		return nil, fmt.Errorf("record not found: %w", err)
	}

	// 2. Check table access
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	fields, err := s.getTableFields(record.TableID)
	if err != nil {
		return nil, err
	}
	readableFields, _, err := s.getFieldAccessMaps(fields, userID)
	if err != nil {
		return nil, err
	}

	// 3. Parse data
	data := s.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))
	data = filterDataFields(data, fieldFilter)

	return &RecordResponse{
		ID:      record.ID,
		TableID: record.TableID,
		Data:    data,
		Version: record.Version,
	}, nil
}

// UpdateRecord updates a record (optimistic locking)
func (s *RecordService) UpdateRecord(recordID string, req UpdateRecordRequest, userID string) (*models.Record, error) {
	// 1. Get record
	var record models.Record
	err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error
	if err != nil {
		return nil, fmt.Errorf("record not found: %w", err)
	}

	// 2. Check table access
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. Optimistic lock check
	if req.Version > 0 && record.Version != req.Version {
		return nil, fmt.Errorf("record was modified by another user (current version: %d, requested version: %d)", record.Version, req.Version)
	}

	fields, err := s.getTableFields(record.TableID)
	if err != nil {
		return nil, err
	}
	readableFields, writableFields, err := s.getFieldAccessMaps(fields, userID)
	if err != nil {
		return nil, err
	}

	normalizedData, err := s.normalizeRecordData(fields, req.Data)
	if err != nil {
		return nil, err
	}
	if err := s.ensureWritableFields(normalizedData, writableFields); err != nil {
		return nil, err
	}

	currentData, _ := s.extractKnownRecordData(fields, parseRecordPayload(record.Data))
	for key, value := range normalizedData {
		currentData[key] = value
	}

	// 4. Validate data
	if err := s.validateRecordData(record.TableID, currentData, record.ID, userID); err != nil {
		return nil, err
	}

	// 5. Serialize data
	dataJSON, err := json.MarshalString(currentData)
	if err != nil {
		return nil, fmt.Errorf("data serialization failed: %w", err)
	}

	// 6. Atomic update to prevent concurrent overwrites
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		updateQuery := tx.Model(&models.Record{}).
			Where("id = ? AND deleted_at IS NULL", recordID)
		if req.Version > 0 {
			updateQuery = updateQuery.Where("version = ?", req.Version)
		}

		updateResult := updateQuery.Updates(map[string]interface{}{
			"data":    dataJSON,
			"version": gorm.Expr("version + 1"),
		})
		if updateResult.Error != nil {
			return fmt.Errorf("failed to update record: %w", updateResult.Error)
		}
		if updateResult.RowsAffected == 0 {
			return errors.New("record was modified by another user, please refresh and retry")
		}

		if err := s.syncAttachmentBindings(tx, recordID, fields, currentData); err != nil {
			return err
		}
		if err := s.syncRecordFieldIndexes(tx, recordID, record.TableID, fields, currentData); err != nil {
			return err
		}

		if err := tx.Where("id = ?", recordID).First(&record).Error; err != nil {
			return fmt.Errorf("failed to read updated record: %w", err)
		}

		return nil
	}); err != nil {
		return nil, err
	}

	filteredData := s.filterReadableData(fields, readableFields, currentData)
	record.Data, err = marshalRecordPayload(filteredData)
	if err != nil {
		return nil, err
	}

	return &record, nil
}

// DeleteRecord soft-deletes a record
func (s *RecordService) DeleteRecord(recordID, userID string) error {
	// 1. Get record
	var record models.Record
	err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error
	if err != nil {
		return fmt.Errorf("record not found: %w", err)
	}

	// 2. Check table access - only owner and admin can delete records
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin"}); err != nil {
		return err
	}

	// 3. Soft-delete record
	now := time.Now()
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&models.Record{}).
			Where("id = ? AND deleted_at IS NULL", recordID).
			Updates(map[string]interface{}{
				"deleted_at": now,
				"updated_at": now,
				"version":    gorm.Expr("version + 1"),
			})
		if result.Error != nil {
			return fmt.Errorf("failed to delete record: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("record not found: %w", gorm.ErrRecordNotFound)
		}
		if err := tx.Model(&models.RecordFieldIndex{}).
			Where("record_id = ? AND deleted_at IS NULL", recordID).
			Update("deleted_at", now).Error; err != nil {
			return fmt.Errorf("failed to delete record field indexes: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	payload := map[string]interface{}{
		"record_id": record.ID,
		"user_id":   userID,
	}
	if record.Data != "" {
		var deletedData map[string]interface{}
		if err := json.UnmarshalString(string(record.Data), &deletedData); err == nil {
			payload["data"] = deletedData
		}
	}

	return nil
}

// BatchCreateRecords creates multiple records at once
func (s *RecordService) BatchCreateRecords(req CreateRecordRequest, userID string, count int) ([]*models.Record, error) {
	// 1. Check table access
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	fields, err := s.getTableFields(req.TableID)
	if err != nil {
		return nil, err
	}
	readableFields, writableFields, err := s.getFieldAccessMaps(fields, userID)
	if err != nil {
		return nil, err
	}

	normalizedData, err := s.normalizeRecordData(fields, req.Data)
	if err != nil {
		return nil, err
	}
	if err := s.ensureWritableFields(normalizedData, writableFields); err != nil {
		return nil, err
	}

	for _, field := range fields {
		if !isAttachmentFieldType(field.Type) {
			continue
		}
		fileIDs, err := attachmentFieldIDsFromData(field, normalizedData)
		if err != nil {
			return nil, err
		}
		if len(fileIDs) > 0 {
			return nil, errors.New("batch creation does not support file fields")
		}
	}

	// 2. Validate data
	if err := s.validateRecordData(req.TableID, normalizedData, "", userID); err != nil {
		return nil, err
	}

	// 3. Serialize data
	dataJSON, err := json.MarshalString(normalizedData)
	if err != nil {
		return nil, fmt.Errorf("data serialization failed: %w", err)
	}

	const batchSize = 100

	// 4. Batch-create in a single transaction for atomicity; batching controls per-INSERT size
	records := make([]*models.Record, 0, count)
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		for i := 0; i < count; i += batchSize {
			end := i + batchSize
			if end > count {
				end = count
			}
			batch := make([]models.Record, 0, end-i)
			for j := i; j < end; j++ {
				batch = append(batch, models.Record{
					TableID: req.TableID,
					Data:    models.JSONField(dataJSON),
					Version: 1,
				})
			}
			if err := tx.Create(&batch).Error; err != nil {
				return fmt.Errorf("batch creation failed: %w", err)
			}
			indexRows := make([]models.RecordFieldIndex, 0, len(batch)*len(fields))
			for j := range batch {
				rows, err := buildRecordFieldIndexRows(req.TableID, batch[j].ID, fields, normalizedData)
				if err != nil {
					return err
				}
				indexRows = append(indexRows, rows...)
				records = append(records, &batch[j])
			}
			if len(indexRows) > 0 {
				if err := tx.Create(&indexRows).Error; err != nil {
					return fmt.Errorf("failed to write record field indexes: %w", err)
				}
			}
		}
		return nil
	}); err != nil {
		return nil, err
	}

	filteredData := s.filterReadableData(fields, readableFields, normalizedData)
	filteredJSON, err := marshalRecordPayload(filteredData)
	if err != nil {
		return nil, err
	}
	for _, record := range records {
		record.Data = filteredJSON
	}

	return records, nil
}

func (s *RecordService) GenerateTestData(tableID, userID string, count int) ([]*models.Record, error) {
	if count <= 0 {
		return []*models.Record{}, nil
	}
	if err := s.checkTableAccess(tableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	fields, err := s.getTableFields(tableID)
	if err != nil {
		return nil, err
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec // test data generation, not security-sensitive
	records := make([]*models.Record, 0, count)
	for i := 0; i < count; i++ {
		data := make(map[string]interface{}, len(fields))
		for _, field := range fields {
			data[field.Name] = generateFieldValue(rng, field.Type)
		}
		record, err := s.CreateRecord(CreateRecordRequest{
			TableID: tableID,
			Data:    data,
		}, userID)
		if err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}
