package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

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
	CreatedBy string      `json:"created_by"`
	UpdatedBy string      `json:"updated_by"`
	Version   int         `json:"version"`
	CreatedAt string      `json:"created_at"`
	UpdatedAt string      `json:"updated_at"`
}

// QueryRequest 查询请求
type QueryRequest struct {
	TableID string `form:"table_id" binding:"required"`
	Limit   int    `form:"limit" binding:"min=1,max=100"`
	Offset  int    `form:"offset" binding:"min=0"`
	Filter  string `form:"filter"` // JSON字符串格式的过滤条件
}

// QueryResponse 查询响应
type QueryResponse struct {
	Records []RecordResponse `json:"records"`
	Total   int64            `json:"total"`
	HasMore bool             `json:"has_more"`
}

// checkTableAccess 检查表访问权限
func (s *RecordService) checkTableAccess(tableID, userID string, requiredRoles []string) error {
	var table models.Table
	err := s.db.Where("id = ?", tableID).First(&table).Error
	if err != nil {
		return errors.New("表不存在")
	}

	var access models.DatabaseAccess
	err = s.db.Where("database_id = ? AND user_id = ?", table.DatabaseID, userID).First(&access).Error
	if err != nil {
		return errors.New("无权访问该数据库")
	}

	roleAllowed := false
	for _, role := range requiredRoles {
		if access.Role == role {
			roleAllowed = true
			break
		}
	}

	if !roleAllowed {
		return fmt.Errorf("需要权限：%v，当前角色：%s", requiredRoles, access.Role)
	}

	return nil
}

// validateRecordData 验证记录数据
func (s *RecordService) validateRecordData(tableID string, data map[string]interface{}) error {
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

		// 根据字段类型验证数据
		if err := s.validateFieldValue(field, value); err != nil {
			return fmt.Errorf("字段 '%s' 验证失败: %w", field.Name, err)
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

	switch field.Type {
	case "string":
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

	case "single_select":
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

	case "multi_select":
		if _, ok := value.([]interface{}); !ok {
			return errors.New("期望数组类型")
		}
		// 检查选项是否有效
		if field.Options != "" {
			var config map[string]interface{}
			if err := json.Unmarshal([]byte(field.Options), &config); err == nil {
				if options, exists := config["options"].([]interface{}); exists {
					for _, val := range value.([]interface{}) {
						valid := false
						for _, opt := range options {
							if opt.(string) == val.(string) {
								valid = true
								break
							}
						}
						if !valid {
							return fmt.Errorf("无效的选项值: %s", val.(string))
						}
					}
				}
			}
		}
	}

	return nil
}

// CreateRecord 创建记录
func (s *RecordService) CreateRecord(req CreateRecordRequest, userID string) (*models.Record, error) {
	// 1. 检查表访问权限（owner, admin, editor可以创建记录）
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 2. 验证数据
	if err := s.validateRecordData(req.TableID, req.Data); err != nil {
		return nil, err
	}

	// 3. 序列化数据
	dataJSON, err := json.Marshal(req.Data)
	if err != nil {
		return nil, fmt.Errorf("数据序列化失败: %w", err)
	}

	// 4. 创建记录
	record := models.Record{
		TableID:   req.TableID,
		Data:      string(dataJSON),
		CreatedBy: userID,
		UpdatedBy: userID,
		Version:   1,
	}

	if err := s.db.Create(&record).Error; err != nil {
		return nil, fmt.Errorf("创建记录失败: %w", err)
	}

	return &record, nil
}

// ListRecords 获取记录列表（支持查询和分页）
func (s *RecordService) ListRecords(req QueryRequest, userID string) (*QueryResponse, error) {
	// 1. 检查表访问权限
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	// 2. 设置默认值
	if req.Limit == 0 {
		req.Limit = 20
	}

	// 3. 构建查询
	query := s.db.Where("table_id = ? AND deleted_at IS NULL", req.TableID)

	// 4. 解析过滤条件
	if req.Filter != "" {
		var filter map[string]interface{}
		if err := json.Unmarshal([]byte(req.Filter), &filter); err != nil {
			return nil, errors.New("过滤条件格式错误")
		}

		// 检测数据库类型并使用适当的查询方式
		isSQLite := s.db.Dialector.Name() == "sqlite"

		// 构建查询条件
		for fieldID, value := range filter {
			jsonValue, _ := json.Marshal(value)

			if isSQLite {
				// SQLite 使用 JSON_EXTRACT 函数
				// data 是 JSON 字符串，需要使用 JSON_EXTRACT(data, '$.fieldID') = 'value'
				query = query.Where("JSON_EXTRACT(data, ?) = ?", fmt.Sprintf("$.%s", fieldID), string(jsonValue[1:len(jsonValue)-1]))
			} else {
				// PostgreSQL 使用 JSONB @> 操作符
				query = query.Where("data @> ?", fmt.Sprintf(`{"%s":%s}`, fieldID, string(jsonValue)))
			}
		}
	}

	// 5. 执行查询
	var records []models.Record
	query = query.Order("created_at DESC").Limit(req.Limit).Offset(req.Offset)
	if err := query.Find(&records).Error; err != nil {
		return nil, fmt.Errorf("查询记录失败: %w", err)
	}

	// 6. 获取总数
	var total int64
	countQuery := s.db.Model(&models.Record{}).Where("table_id = ? AND deleted_at IS NULL", req.TableID)
	if req.Filter != "" {
		var filter map[string]interface{}
		if err := json.Unmarshal([]byte(req.Filter), &filter); err == nil {
			// 检测数据库类型并使用适当的查询方式
			isSQLite := s.db.Dialector.Name() == "sqlite"

			for fieldID, value := range filter {
				jsonValue, _ := json.Marshal(value)

				if isSQLite {
					// SQLite 使用 JSON_EXTRACT 函数
					countQuery = countQuery.Where("JSON_EXTRACT(data, ?) = ?", fmt.Sprintf("$.%s", fieldID), string(jsonValue[1:len(jsonValue)-1]))
				} else {
					// PostgreSQL 使用 JSONB @> 操作符
					countQuery = countQuery.Where("data @> ?", fmt.Sprintf(`{"%s":%s}`, fieldID, string(jsonValue)))
				}
			}
		}
	}
	countQuery.Count(&total)

	// 7. 转换为响应格式
	result := make([]RecordResponse, len(records))
	for i, r := range records {
		var data interface{}
		if r.Data != "" {
			json.Unmarshal([]byte(r.Data), &data)
		}

		result[i] = RecordResponse{
			ID:        r.ID,
			TableID:   r.TableID,
			Data:      data,
			CreatedBy: r.CreatedBy,
			UpdatedBy: r.UpdatedBy,
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

// GetRecord 获取单个记录
func (s *RecordService) GetRecord(recordID, userID string) (*RecordResponse, error) {
	// 1. 获取记录
	var record models.Record
	err := s.db.Where("id = ?", recordID).First(&record).Error
	if err != nil {
		return nil, fmt.Errorf("记录不存在: %w", err)
	}

	// 2. 检查表访问权限
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin", "editor", "viewer"}); err != nil {
		return nil, err
	}

	// 3. 解析数据
	var data interface{}
	if record.Data != "" {
		json.Unmarshal([]byte(record.Data), &data)
	}

	return &RecordResponse{
		ID:        record.ID,
		TableID:   record.TableID,
		Data:      data,
		CreatedBy: record.CreatedBy,
		UpdatedBy: record.UpdatedBy,
		Version:   record.Version,
		CreatedAt: record.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt: record.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateRecord 更新记录（乐观锁）
func (s *RecordService) UpdateRecord(recordID string, req UpdateRecordRequest, userID string) (*models.Record, error) {
	// 1. 获取记录
	var record models.Record
	err := s.db.Where("id = ?", recordID).First(&record).Error
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

	// 4. 验证数据
	if err := s.validateRecordData(record.TableID, req.Data); err != nil {
		return nil, err
	}

	// 5. 序列化数据
	dataJSON, err := json.Marshal(req.Data)
	if err != nil {
		return nil, fmt.Errorf("数据序列化失败: %w", err)
	}

	// 6. 更新记录
	record.Data = string(dataJSON)
	record.UpdatedBy = userID
	record.Version = record.Version + 1

	if err := s.db.Save(&record).Error; err != nil {
		return nil, fmt.Errorf("更新记录失败: %w", err)
	}

	return &record, nil
}

// DeleteRecord 删除记录（软删除）
func (s *RecordService) DeleteRecord(recordID, userID string) error {
	// 1. 获取记录
	var record models.Record
	err := s.db.Where("id = ?", recordID).First(&record).Error
	if err != nil {
		return fmt.Errorf("记录不存在: %w", err)
	}

	// 2. 检查表访问权限
	if err := s.checkTableAccess(record.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return err
	}

	// 3. 软删除记录
	if err := s.db.Delete(&record).Error; err != nil {
		return fmt.Errorf("删除记录失败: %w", err)
	}

	return nil
}

// BatchCreateRecords 批量创建记录
func (s *RecordService) BatchCreateRecords(req CreateRecordRequest, userID string, count int) ([]*models.Record, error) {
	// 1. 检查表访问权限
	if err := s.checkTableAccess(req.TableID, userID, []string{"owner", "admin", "editor"}); err != nil {
		return nil, err
	}

	// 2. 验证数据
	if err := s.validateRecordData(req.TableID, req.Data); err != nil {
		return nil, err
	}

	// 3. 序列化数据
	dataJSON, err := json.Marshal(req.Data)
	if err != nil {
		return nil, fmt.Errorf("数据序列化失败: %w", err)
	}

	// 4. 批量创建
	records := make([]*models.Record, count)
	for i := 0; i < count; i++ {
		records[i] = &models.Record{
			TableID:   req.TableID,
			Data:      string(dataJSON),
			CreatedBy: userID,
			UpdatedBy: userID,
			Version:   1,
		}
	}

	// 使用事务确保原子性
	tx := s.db.Begin()
	for i, record := range records {
		if err := tx.Create(record).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("批量创建失败: %w", err)
		}
		// Add small delay between records to ensure unique IDs
		// This prevents ID collisions when GenerateID() uses nanosecond timestamps
		if i < len(records)-1 {
			time.Sleep(1 * time.Millisecond)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return records, nil
}
