package services

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
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
				return nil, errors.New("列表项必须是字符串")
			}
			items = append(items, str)
		}
		return items, nil
	default:
		return nil, errors.New("期望字符串数组类型")
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
				return nil, errors.New("附件值必须是文件ID或文件ID数组")
			}
			trimmed := strings.TrimSpace(str)
			if trimmed != "" {
				items = append(items, trimmed)
			}
		}
		return items, nil
	default:
		return nil, errors.New("附件值必须是文件ID或文件ID数组")
	}
}

// RecordService 数据记录服务
type RecordService struct {
	db *gorm.DB
}

// NewRecordService 创建记录服务实例
func NewRecordService(db *gorm.DB) *RecordService {
	return &RecordService{db: db}
}

// CreateRecordRequest 创建记录请求
type CreateRecordRequest struct {
	TableID string                 `json:"table_id" binding:"required"`
	Data    map[string]interface{} `json:"data" binding:"required"`
}

// UpdateRecordRequest 更新记录请求
type UpdateRecordRequest struct {
	Data    map[string]interface{} `json:"data" binding:"required"`
	Version int                    `json:"version"` // 乐观锁版本号
}

// RecordResponse 记录响应
type RecordResponse struct {
	ID        string      `json:"id"`
	TableID   string      `json:"table_id"`
	Data      interface{} `json:"data"`
	Version   int         `json:"version"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

// QueryRequest 查询请求
type QueryRequest struct {
	TableID string `form:"table_id" binding:"required"`
	Limit   int    `form:"limit" binding:"min=1,max=100"`
	Offset  int    `form:"offset" binding:"min=0"`
	Filter  string `form:"filter"` // 支持 JSON 过滤或关键字搜索
}

// QueryResponse 查询响应
type QueryResponse struct {
	Records []RecordResponse `json:"records"`
	Total   int64            `json:"total"`
	HasMore bool             `json:"has_more"`
}

// checkTableAccess 检查表是否存在
func (s *RecordService) checkTableAccess(tableID, userID string, requiredRoles []string) error {
	var table models.Table
	err := s.db.Where("id = ? AND deleted_at IS NULL", tableID).First(&table).Error
	if err != nil {
		return errors.New("表不存在")
	}

	var db models.Database
	err = s.db.Where("id = ? AND deleted_at IS NULL", table.DatabaseID).First(&db).Error
	if err != nil {
		return errors.New("数据库不存在")
	}

	return nil
}

// validateRecordData 验证记录数据
func (s *RecordService) validateRecordData(tableID string, data map[string]interface{}, currentRecordID, userID string) error {
	// 获取表的所有字段定义
	var fields []models.Field
	err := s.db.Where("table_id = ? AND deleted_at IS NULL", tableID).Find(&fields).Error
	if err != nil {
		return fmt.Errorf("获取字段定义失败: %w", err)
	}

	// 验证每个字段
	for _, field := range fields {
		// 支持通过字段ID或字段名查找数据
		value, existsByID := data[field.ID]
		valueByName, existsByName := data[field.Name]

		// 如果通过ID和名称都找不到，但字段是必填的，则报错
		if field.Required && !existsByID && !existsByName {
			return fmt.Errorf("字段 '%s' 是必填的", field.Name)
		}

		// 如果字段不存在，跳过验证
		if !existsByID && !existsByName {
			continue
		}

		// 优先使用通过名称找到的值（如果存在）
		if existsByName {
			value = valueByName
		}

		// 对于可选字段，如果值为空或nil，则跳过验证
		if !field.Required && (value == nil || value == "") {
			continue
		}

		if isAttachmentFieldType(field.Type) {
			if err := s.validateAttachmentFieldValue(field, value, currentRecordID, userID); err != nil {
				return fmt.Errorf("字段 '%s' 验证失败: %w", field.Name, err)
			}
			continue
		}

		// 根据字段类型验证数据
		if err := s.validateFieldValue(field, value); err != nil {
			return fmt.Errorf("字段 '%s' 验证失败: %w", field.Name, err)
		}
	}

	return nil
}

func (s *RecordService) validateAttachmentFieldValue(field models.Field, value interface{}, currentRecordID, userID string) error {
	fileIDs, err := parseAttachmentValue(value)
	if err != nil {
		return err
	}

	config := parseStoredFieldConfig(field.Options)
	if !config.Multiple && len(fileIDs) > 1 {
		return errors.New("该附件字段只允许上传单个文件")
	}

	seen := make(map[string]struct{}, len(fileIDs))
	for _, fileID := range fileIDs {
		if _, exists := seen[fileID]; exists {
			return fmt.Errorf("附件ID重复: %s", fileID)
		}
		seen[fileID] = struct{}{}

		file, _, err := NewFileService(s.db).getAccessibleFile(fileID, userID, []string{"owner", "admin", "editor"})
		if err != nil {
			return err
		}
		if file.FieldID != field.ID {
			return errors.New("附件不属于当前字段")
		}
		if currentRecordID == "" {
			if file.RecordID != "" {
				return errors.New("创建记录时只能引用未绑定记录的附件")
			}
		} else if file.RecordID != "" && file.RecordID != currentRecordID {
			return errors.New("附件已绑定到其他记录")
		}
		if config.MaxFileSizeMB > 0 && file.FileSize > int64(config.MaxFileSizeMB)*1024*1024 {
			return fmt.Errorf("附件大小超过字段限制（最大%dMB）", config.MaxFileSizeMB)
		}
		if !fileMatchesAllowedTypes(file.FileName, file.FileType, config.AllowedTypes) {
			return errors.New("附件类型不符合字段限制")
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
			return fmt.Errorf("同步附件字段 %s 失败: %w", field.Name, err)
		}

		referenced := make(map[string]struct{}, len(fileIDs))
		for _, fileID := range fileIDs {
			referenced[fileID] = struct{}{}
		}

		var existingFiles []models.File
		if err := tx.Where("record_id = ? AND field_id = ?", recordID, field.ID).Find(&existingFiles).Error; err != nil {
			return fmt.Errorf("查询附件绑定失败: %w", err)
		}

		for _, existingFile := range existingFiles {
			if _, ok := referenced[existingFile.ID]; ok {
				continue
			}
			if err := tx.Model(&models.File{}).Where("id = ?", existingFile.ID).Update("record_id", "").Error; err != nil {
				return fmt.Errorf("解除附件绑定失败: %w", err)
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
			return fmt.Errorf("绑定附件失败: %w", err)
		}
	}

	return nil
}

// validateFieldValue 验证字段值
func (s *RecordService) validateFieldValue(field models.Field, value interface{}) error {
	// Handle nil values - these should be handled by the caller, but we'll be defensive
	if value == nil {
		return nil
	}

	switch normalizeFieldType(field.Type) {
	case "string", "text":
		if _, ok := value.(string); !ok {
			return errors.New("期望字符串类型")
		}

		strValue := value.(string)

		// 如果是空字符串，跳过后续验证（长度和正则）
		if strValue == "" {
			return nil
		}

		// 检查配置
		if field.Options != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(field.Options), &config); err == nil {
				// 检查最大长度
				if maxLen, exists := config["max_length"].(float64); exists {
					if len(strValue) > int(maxLen) {
						return fmt.Errorf("长度不能超过 %d 个字符", int(maxLen))
					}
				}

				// 检查正则表达式验证
				if validation, exists := config["validation"].(string); exists && validation != "" {
					matched, err := regexp.MatchString(validation, strValue)
					if err != nil {
						return fmt.Errorf("正则表达式无效: %w", err)
					}
					if !matched {
						return fmt.Errorf("格式不匹配，要求: %s", validation)
					}
				}
			}
		}

	case "number":
		switch value.(type) {
		case float64, float32, int, int32, int64:
			// OK
		default:
			return errors.New("期望数字类型")
		}

	case "boolean":
		if _, ok := value.(bool); !ok {
			return errors.New("期望布尔类型")
		}

	case "date", "datetime":
		// 简单验证：检查是否为字符串
		if _, ok := value.(string); !ok {
			return errors.New("期望字符串类型（日期格式）")
		}

	case "select", "single_select":
		if _, ok := value.(string); !ok {
			return errors.New("期望字符串类型")
		}
		// 检查选项是否有效
		if field.Options != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(field.Options), &config); err == nil {
				if options, exists := config["options"].([]interface{}); exists {
					valid := false
					for _, opt := range options {
						if opt.(string) == value.(string) {
							valid = true
							break
						}
					}
					if !valid {
						return fmt.Errorf("无效的选项值: %s", value.(string))
					}
				}
			}
		}

	case "list":
		items, err := parseStringListValue(value)
		if err != nil {
			return err
		}

		// 历史 multiselect / multi_select 字段继续沿用选项约束；
		// 新 list 类型只校验“字符串数组”，不强制命中建议项。
		if (field.Type == "multiselect" || field.Type == "multi_select") && field.Options != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(field.Options), &config); err == nil {
				if options, exists := config["options"].([]interface{}); exists {
					for _, val := range items {
						valid := false
						for _, opt := range options {
							if opt.(string) == val {
								valid = true
								break
							}
						}
						if !valid {
							return fmt.Errorf("无效的选项值: %s", val)
						}
					}
				}
			}
		}
	}

	return nil
}

func (s *RecordService) getTableFields(tableID string) ([]models.Field, error) {
	var fields []models.Field
	if err := s.db.Where("table_id = ? AND deleted_at IS NULL", tableID).
		Order("created_at ASC").
		Find(&fields).Error; err != nil {
		return nil, fmt.Errorf("获取字段定义失败: %w", err)
	}
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
				return nil, fmt.Errorf("字段 '%s' 不存在", key)
			}
		}
	}
	return normalized, nil
}

func (s *RecordService) getFieldAccessMaps(fields []models.Field, userID string) (map[string]models.Field, map[string]models.Field, error) {
	readableFields := make(map[string]models.Field, len(fields))
	writableFields := make(map[string]models.Field, len(fields))
	fieldService := NewFieldService(s.db)

	for _, field := range fields {
		if err := fieldService.CheckFieldPermission(userID, field.ID, "read"); err == nil {
			readableFields[field.Name] = field
		}
		if err := fieldService.CheckFieldPermission(userID, field.ID, "write"); err == nil {
			writableFields[field.Name] = field
		}
	}

	return readableFields, writableFields, nil
}

func (s *RecordService) ensureWritableFields(data map[string]interface{}, writableFields map[string]models.Field) error {
	for fieldName := range data {
		if _, ok := writableFields[fieldName]; !ok {
			return fmt.Errorf("字段 '%s' 无写入权限", fieldName)
		}
	}
	return nil
}

func parseRecordPayload(raw string) map[string]interface{} {
	payload := make(map[string]interface{})
	if raw == "" {
		return payload
	}
	_ = json.Unmarshal([]byte(raw), &payload)
	return payload
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

func marshalRecordPayload(payload map[string]interface{}) (string, error) {
	dataJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("数据序列化失败: %w", err)
	}
	return string(dataJSON), nil
}

// recordFilterClause 是一个可复用的 WHERE 片段,既用于分页查询,也用于 COUNT。
type recordFilterClause struct {
	sql  string
	args []interface{}
}

// tryParseStructuredFilter 把 filter 字符串当作 JSON 对象解析。
// 仅当返回 true 时调用方应走结构化下推路径;否则按关键字处理。
func tryParseStructuredFilter(filter string) (map[string]interface{}, bool) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil, false
	}
	var structured map[string]interface{}
	if err := json.Unmarshal([]byte(filter), &structured); err != nil {
		return nil, false
	}
	if len(structured) == 0 {
		return nil, false
	}
	return structured, true
}

// buildStructuredFilterClauses 根据可见字段把 JSON 过滤条件翻译为可下推 SQL WHERE 片段。
//
// 返回:
//
//	clauses          : 应用到 GORM 查询的 (sql, args);字段名通过参数化传给驱动,不直接拼入 SQL 串
//	refsHiddenField  : 任一过滤键引用了隐藏/未知字段 → true,调用方应直接返回空结果(与 in-memory
//	                  权限感知过滤的可观察行为对齐,避免通过 200 vs 400 做侧信道探测)
//	err              : value 序列化失败等结构性错误,作为 4xx 抛给客户端
func (s *RecordService) buildStructuredFilterClauses(
	fields []models.Field,
	readableFields map[string]models.Field,
	structured map[string]interface{},
) ([]recordFilterClause, bool, error) {
	isSQLite := s.db.Name() == "sqlite"
	clauses := make([]recordFilterClause, 0, len(structured))

	for key, value := range structured {
		fieldName, ok := resolveReadableFilterField(fields, readableFields, key)
		if !ok {
			return nil, true, nil
		}

		jsonValue, err := json.Marshal(value)
		if err != nil {
			return nil, false, fmt.Errorf("过滤值序列化失败: %w", err)
		}

		if isSQLite {
			var scalar interface{}
			if err := json.Unmarshal(jsonValue, &scalar); err != nil {
				return nil, false, fmt.Errorf("过滤值格式错误: %w", err)
			}
			// JSON path 与字段值都走参数化绑定,字段名只出现在第一个 ? 内,不会与 SQL 字面量混。
			clauses = append(clauses, recordFilterClause{
				sql:  "JSON_EXTRACT(data, ?) = ?",
				args: []interface{}{"$." + fieldName, scalar},
			})
		} else {
			// PG: 构造 {"<field>":<value>} 字面量后整体作为 jsonb 参数传入,
			// 字段名经过 json.Marshal 转义,既能安全表达 Unicode,也不与 SQL 占位符混。
			filterDoc, err := json.Marshal(map[string]interface{}{fieldName: value})
			if err != nil {
				return nil, false, fmt.Errorf("过滤条件序列化失败: %w", err)
			}
			clauses = append(clauses, recordFilterClause{
				sql:  "data @> ?",
				args: []interface{}{string(filterDoc)},
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

func resolveReadableFilterField(fields []models.Field, readableFields map[string]models.Field, filterKey string) (string, bool) {
	if _, ok := readableFields[filterKey]; ok {
		return filterKey, true
	}

	for _, field := range fields {
		if field.ID != filterKey {
			continue
		}
		if _, ok := readableFields[field.Name]; !ok {
			return "", false
		}
		return field.Name, true
	}

	return "", false
}

func (s *RecordService) matchesRecordFilter(fields []models.Field, readableFields map[string]models.Field, payload map[string]interface{}, filter string) (bool, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return true, nil
	}

	var structuredFilter map[string]interface{}
	if err := json.Unmarshal([]byte(filter), &structuredFilter); err == nil && len(structuredFilter) > 0 {
		for filterKey, expected := range structuredFilter {
			fieldName, ok := resolveReadableFilterField(fields, readableFields, filterKey)
			if !ok {
				return false, nil
			}

			actual, exists := payload[fieldName]
			if !exists || !jsonValuesEqual(actual, expected) {
				return false, nil
			}
		}
		return true, nil
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return false, fmt.Errorf("记录过滤失败: %w", err)
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

// CreateRecord 创建记录
func (s *RecordService) CreateRecord(req CreateRecordRequest, userID string) (*models.Record, error) {
	// 1. 检查表访问权限（owner, admin, editor可以创建记录）
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

	// 2. 验证数据
	if err := s.validateRecordData(req.TableID, normalizedData, "", userID); err != nil {
		return nil, err
	}

	// 3. 序列化数据
	dataJSON, err := json.Marshal(normalizedData)
	if err != nil {
		return nil, fmt.Errorf("数据序列化失败: %w", err)
	}

	// 4. 创建记录并绑定附件
	record := models.Record{
		TableID: req.TableID,
		Data:    string(dataJSON),
		Version: 1,
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&record).Error; err != nil {
			return fmt.Errorf("创建记录失败: %w", err)
		}
		if err := s.syncAttachmentBindings(tx, record.ID, fields, normalizedData); err != nil {
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

	return &record, nil
}

// maxKeywordScanRecords 为关键字回退路径在内存中检查的最大行数上限。
// 超过该上限时拒绝查询并提示走 /query 接口,避免一次性把整张表加载到内存。
// 声明为 var 以便测试中替换,生产代码不应改写。
var maxKeywordScanRecords = 5000

// ListRecords 获取记录列表（支持查询和分页）
func (s *RecordService) ListRecords(req QueryRequest, userID string) (*QueryResponse, error) {
	// 1. 检查表访问权限
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

	// 2. 设置默认值
	if req.Limit == 0 {
		req.Limit = 20
	}

	var records []models.Record
	var total int64
	filter := strings.TrimSpace(req.Filter)

	switch filter {
	case "":
		// 3a. 无过滤: SQL 分页 + COUNT
		listQ := s.db.Where("table_id = ? AND deleted_at IS NULL", req.TableID).
			Order("created_at DESC").Limit(req.Limit).Offset(req.Offset)
		if err := listQ.Find(&records).Error; err != nil {
			return nil, fmt.Errorf("查询记录失败: %w", err)
		}
		countQ := s.db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", req.TableID)
		if err := countQ.Count(&total).Error; err != nil {
			return nil, fmt.Errorf("统计记录失败: %w", err)
		}

	default:
		structured, isStructured := tryParseStructuredFilter(filter)
		if isStructured {
			// 3b. 结构化 JSON 过滤: 翻译为参数化 WHERE 子句,下推到 SQL,COUNT 同步下推
			clauses, refsHidden, err := s.buildStructuredFilterClauses(fields, readableFields, structured)
			if err != nil {
				return nil, err
			}
			if refsHidden {
				// 引用隐藏/未知字段时直接返回空结果,
				// 避免通过 200 vs 400 做侧信道探测隐藏字段值
				return &QueryResponse{Records: []RecordResponse{}, Total: 0, HasMore: false}, nil
			}

			listQ := s.db.Where("table_id = ? AND deleted_at IS NULL", req.TableID)
			countQ := s.db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", req.TableID)
			for _, c := range clauses {
				listQ = listQ.Where(c.sql, c.args...)
				countQ = countQ.Where(c.sql, c.args...)
			}
			if err := listQ.Order("created_at DESC").Limit(req.Limit).Offset(req.Offset).Find(&records).Error; err != nil {
				return nil, fmt.Errorf("查询记录失败: %w", err)
			}
			if err := countQ.Count(&total).Error; err != nil {
				return nil, fmt.Errorf("统计记录失败: %w", err)
			}
		} else {
			// 3c. 关键字回退: 先用 SQL LIKE 预筛(限上限+1 行用于检测溢出),
			// 再做权限感知 in-memory 过滤,确保隐藏字段不能通过模糊匹配泄漏
			likePattern := "%" + filter + "%"
			narrowQ := s.db.Where("table_id = ? AND deleted_at IS NULL AND data LIKE ?", req.TableID, likePattern).
				Order("created_at DESC").Limit(maxKeywordScanRecords + 1)
			var narrowed []models.Record
			if err := narrowQ.Find(&narrowed).Error; err != nil {
				return nil, fmt.Errorf("查询记录失败: %w", err)
			}
			if len(narrowed) > maxKeywordScanRecords {
				return nil, fmt.Errorf("关键字过滤匹配过多记录(>%d),请使用更精确的过滤条件或 /query 接口", maxKeywordScanRecords)
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

	// 7. 转换为响应格式
	result := make([]RecordResponse, len(records))
	for i, r := range records {
		data := s.filterReadableData(fields, readableFields, parseRecordPayload(r.Data))

		result[i] = RecordResponse{
			ID:        r.ID,
			TableID:   r.TableID,
			Data:      data,
			Version:   r.Version,
			CreatedAt: r.CreatedAt.Format("2006-01-02 15:04:05"),
			UpdatedAt: r.UpdatedAt.Format("2006-01-02 15:04:05"),
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

// ExportRecords 导出记录数据
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
		return nil, "", "", fmt.Errorf("读取记录失败: %w", err)
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
				"id":         record.ID,
				"table_id":   record.TableID,
				"version":    record.Version,
				"created_at": record.CreatedAt.Format(time.RFC3339),
				"updated_at": record.UpdatedAt.Format(time.RFC3339),
			}

			payload := s.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))
			row["data"] = payload
			exportRows = append(exportRows, row)
		}

		data, err := json.MarshalIndent(exportRows, "", "  ")
		if err != nil {
			return nil, "", "", fmt.Errorf("导出JSON失败: %w", err)
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
		header = append(header, "version", "created_at", "updated_at")
		if err := writer.Write(header); err != nil {
			return nil, "", "", fmt.Errorf("写入CSV表头失败: %w", err)
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
				record.CreatedAt.Format(time.RFC3339),
				record.UpdatedAt.Format(time.RFC3339),
			)

			if err := writer.Write(row); err != nil {
				return nil, "", "", fmt.Errorf("写入CSV数据失败: %w", err)
			}
		}

		writer.Flush()
		if err := writer.Error(); err != nil {
			return nil, "", "", fmt.Errorf("生成CSV失败: %w", err)
		}

		filename := fmt.Sprintf("records_%s_%s.csv", tableID, time.Now().Format("20060102150405"))
		return buf.Bytes(), "text/csv; charset=utf-8", filename, nil

	default:
		return nil, "", "", errors.New("不支持的导出格式，仅支持 csv/json")
	}
}

// GetRecord 获取单个记录
func (s *RecordService) GetRecord(recordID, userID string) (*RecordResponse, error) {
	// 1. 获取记录
	var record models.Record
	err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error
	if err != nil {
		return nil, fmt.Errorf("记录不存在: %w", err)
	}

	// 2. 检查表访问权限
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

	// 3. 解析数据
	data := s.filterReadableData(fields, readableFields, parseRecordPayload(record.Data))

	return &RecordResponse{
		ID:        record.ID,
		TableID:   record.TableID,
		Data:      data,
		Version:   record.Version,
		CreatedAt: record.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: record.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateRecord 更新记录（乐观锁）
func (s *RecordService) UpdateRecord(recordID string, req UpdateRecordRequest, userID string) (*models.Record, error) {
	// 1. 获取记录
	var record models.Record
	err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error
	if err != nil {
		return nil, fmt.Errorf("记录不存在: %w", err)
	}

	// 2. 检查表访问权限
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 3. 乐观锁检查
	if req.Version > 0 && record.Version != req.Version {
		return nil, fmt.Errorf("记录已被其他用户修改，当前版本：%d，请求版本：%d", record.Version, req.Version)
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

	// 4. 验证数据
	if err := s.validateRecordData(record.TableID, currentData, record.ID, userID); err != nil {
		return nil, err
	}

	// 5. 序列化数据
	dataJSON, err := json.Marshal(currentData)
	if err != nil {
		return nil, fmt.Errorf("数据序列化失败: %w", err)
	}

	// 6. 原子更新，避免并发覆盖写
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		updateQuery := tx.Model(&models.Record{}).
			Where("id = ? AND deleted_at IS NULL", recordID)
		if req.Version > 0 {
			updateQuery = updateQuery.Where("version = ?", req.Version)
		}

		updateResult := updateQuery.Updates(map[string]interface{}{
			"data":       string(dataJSON),
			"updated_by": userID,
			"version":    gorm.Expr("version + 1"),
		})
		if updateResult.Error != nil {
			return fmt.Errorf("更新记录失败: %w", updateResult.Error)
		}
		if updateResult.RowsAffected == 0 {
			return errors.New("记录已被其他用户修改，请刷新后重试")
		}

		if err := s.syncAttachmentBindings(tx, recordID, fields, currentData); err != nil {
			return err
		}

		if err := tx.Where("id = ?", recordID).First(&record).Error; err != nil {
			return fmt.Errorf("读取更新后记录失败: %w", err)
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

// DeleteRecord 删除记录（软删除）
func (s *RecordService) DeleteRecord(recordID, userID string) error {
	// 1. 获取记录
	var record models.Record
	err := s.db.Where("id = ? AND deleted_at IS NULL", recordID).First(&record).Error
	if err != nil {
		return fmt.Errorf("记录不存在: %w", err)
	}

	// 2. 检查表访问权限
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return err
	}

	// 3. 软删除记录
	now := time.Now()
	result := s.db.Model(&models.Record{}).
		Where("id = ? AND deleted_at IS NULL", recordID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
			"version":    gorm.Expr("version + 1"),
		})
	if result.Error != nil {
		return fmt.Errorf("删除记录失败: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("记录不存在: %w", gorm.ErrRecordNotFound)
	}

	payload := map[string]interface{}{
		"record_id": record.ID,
		"user_id":   userID,
	}
	if record.Data != "" {
		var deletedData map[string]interface{}
		if err := json.Unmarshal([]byte(record.Data), &deletedData); err == nil {
			payload["data"] = deletedData
		}
	}

	return nil
}

// BatchCreateRecords 批量创建记录
func (s *RecordService) BatchCreateRecords(req CreateRecordRequest, userID string, count int) ([]*models.Record, error) {
	// 1. 检查表访问权限
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
			return nil, errors.New("批量创建暂不支持 attachment 字段")
		}
	}

	// 2. 验证数据
	if err := s.validateRecordData(req.TableID, normalizedData, "", userID); err != nil {
		return nil, err
	}

	// 3. 序列化数据
	dataJSON, err := json.Marshal(normalizedData)
	if err != nil {
		return nil, fmt.Errorf("数据序列化失败: %w", err)
	}

	// 4. 批量创建
	records := make([]*models.Record, count)
	for i := 0; i < count; i++ {
		records[i] = &models.Record{
			TableID: req.TableID,
			Data:    string(dataJSON),
			Version: 1,
		}
	}

	// 使用事务确保原子性
	tx := s.db.Begin()
	for _, record := range records {
		if err := tx.Create(record).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("批量创建失败: %w", err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
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
