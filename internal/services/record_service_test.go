package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
)

// ============================================================
// 1. 日期/时间格式验证 (P1)
// ============================================================

func TestValidateFieldValue_DateValid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "date"}
	err := s.validateFieldValue(field, "2024-01-15")
	assert.NoError(t, err)
}

func TestValidateFieldValue_DateInvalid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "date"}
	err := s.validateFieldValue(field, "not-a-date")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "日期格式")
}

func TestValidateFieldValue_DateTimeValid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "datetime"}
	err := s.validateFieldValue(field, "2024-01-15T10:30:00Z")
	assert.NoError(t, err)
}

func TestValidateFieldValue_DateTimeInvalid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "datetime"}
	err := s.validateFieldValue(field, "hello-world")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "日期格式")
}

// ============================================================
// 2. JSON 类型验证 (P1)
// ============================================================

func TestValidateFieldValue_JSONValid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "json"}
	err := s.validateFieldValue(field, map[string]any{"k": "v"})
	assert.NoError(t, err)
}

func TestValidateFieldValue_JSONInvalid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "json"}
	// json.Number("NaN") 不能被标准库序列化，属于非法 JSON 值
	err := s.validateFieldValue(field, map[string]any{"bad": json.Number("NaN")})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无效的 JSON")
}

func TestValidateFieldValue_JSONStringInvalid(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "json"}
	// 测试非法 JSON 字符串是否能被拦截——当前实现直接返回 nil，应该改进
	err := s.validateFieldValue(field, "{not json")
	// 期望报错
	require.Error(t, err)
}

// ============================================================
// 3. number 类型支持 json.Number (P1)
// ============================================================

func TestValidateFieldValue_NumberJSONNumber(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "number"}
	err := s.validateFieldValue(field, json.Number("42"))
	assert.NoError(t, err)
}

func TestValidateFieldValue_NumberFloat64(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "number"}
	err := s.validateFieldValue(field, 3.14)
	assert.NoError(t, err)
}

func TestValidateFieldValue_NumberStringRejected(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	field := models.Field{Type: "number"}
	err := s.validateFieldValue(field, "42")
	require.Error(t, err)
}

// ============================================================
// 4. 字段配置解析缓存 (P0) — validateRecordData 不应重复 Unmarshal
// ============================================================

func TestValidateRecordData_PreParsesFieldConfig(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	// 创建表、字段（带 Options）
	dbModel := &models.Database{Name: "test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)

	config := FieldConfig{MaxLength: intPtr(5)}
	configJSON, _ := json.Marshal(config)
	field := &models.Field{
		TableID:  tbl.ID,
		Name:     "title",
		Type:     "string",
		Required: true,
		Options:  string(configJSON),
	}
	require.NoError(t, db.Create(field).Error)

	// 正常值
	err := s.validateRecordData(tbl.ID, map[string]any{"title": "hi"}, "", "user1")
	assert.NoError(t, err)

	// 超长值——应触发配置中的 max_length 限制
	err = s.validateRecordData(tbl.ID, map[string]any{"title": "hello world"}, "", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "长度不能超过")
}

// ============================================================
// 5. BatchCreateRecords 大批量创建 (P2)
// ============================================================

func TestBatchCreateRecords_LargeBatchSuccess(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)

	// 创建两个字段，确保有字段定义
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	// 批量创建 150 条（超过默认批次大小 100），验证大批量场景
	records, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "batch"},
	}, "user1", 150)
	require.NoError(t, err)
	assert.Len(t, records, 150)

	// 验证数据库中确实有 150 条
	var count int64
	require.NoError(t, db.Model(&models.Record{}).Where("table_id = ?", tbl.ID).Count(&count).Error)
	assert.Equal(t, int64(150), count)
}

func TestBatchCreateRecords_EmptyData(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	records, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "x"},
	}, "user1", 0)
	require.NoError(t, err)
	assert.Empty(t, records)
}

// ============================================================
// 6. 字段权限批量检查 (P0)
// ============================================================

func TestFieldService_CheckFieldPermissions_Batch(t *testing.T) {
	db := setupTestDB(t)
	s := NewFieldService(db)

	dbModel := &models.Database{Name: "test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)

	f1, err := s.CreateField(CreateFieldRequest{TableID: tbl.ID, Name: "a", Type: "string"}, "user1")
	require.NoError(t, err)
	f2, err := s.CreateField(CreateFieldRequest{TableID: tbl.ID, Name: "b", Type: "string"}, "user1")
	require.NoError(t, err)

	// Master token 可以访问所有字段
	results, err := s.CheckFieldPermissions("user1", []string{f1.ID, f2.ID}, "read")
	require.NoError(t, err)
	assert.True(t, results[f1.ID])
	assert.True(t, results[f2.ID])
}

// ============================================================
// 7. BatchCreateRecords 原子性测试
// ============================================================

func TestBatchCreateRecords_AtomicRollback(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	// 先正常创建 50 条
	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "batch"},
	}, "user1", 50)
	require.NoError(t, err)

	var beforeCount int64
	require.NoError(t, db.Model(&models.Record{}).Where("table_id = ?", tbl.ID).Count(&beforeCount).Error)
	require.Equal(t, int64(50), beforeCount)

	// 注入 Create 错误，使后续批量创建失败
	err = db.Callback().Create().Before("gorm:create").Register("test_batch_rollback", func(d *gorm.DB) {
		d.Error = fmt.Errorf("injected batch create error")
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Callback().Create().Remove("test_batch_rollback") })

	// 尝试创建 100 条，应失败并回滚
	_, err = s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]any{"name": "batch2"},
	}, "user1", 100)
	assert.Error(t, err)

	// 验证回滚：数据库中仍只有最初 50 条
	var afterCount int64
	require.NoError(t, db.Model(&models.Record{}).Where("table_id = ?", tbl.ID).Count(&afterCount).Error)
	assert.Equal(t, int64(50), afterCount, "事务回滚后记录数应保持不变")
}

// ============================================================
// 8. CheckFieldPermissions 空数组保护
// ============================================================

func TestFieldService_CheckFieldPermissions_Empty(t *testing.T) {
	db := setupTestDB(t)
	s := NewFieldService(db)

	results, err := s.CheckFieldPermissions("user1", []string{}, "read")
	require.NoError(t, err)
	assert.Empty(t, results)
}

// ============================================================
// 9. UpdateRecord 错误路径
// ============================================================

func TestRecordService_UpdateRecord_VersionConflict(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "test"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	record := &models.Record{TableID: tbl.ID, Data: `{"name":"alice"}`, Version: 1}
	require.NoError(t, db.Create(record).Error)

	_, err := s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data:    map[string]any{"name": "bob"},
		Version: 999,
	}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "版本")
}

func TestRecordService_UpdateRecord_NonexistentRecord(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, err := s.UpdateRecord("rec_nonexistent", UpdateRecordRequest{
		Data: map[string]any{"name": "bob"},
	}, "user1")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "记录不存在")
}

func TestRecordService_DeleteRecord_NonexistentRecord(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	err := s.DeleteRecord("rec_nonexistent", "user1")
	assert.Error(t, err)
}

func TestRecordService_GetRecord_NonexistentRecord(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, err := s.GetRecord("rec_nonexistent", "user1")
	assert.Error(t, err)
}

// ============================================================
// 辅助函数
// ============================================================

func intPtr(v int) *int {
	return &v
}

// ============================================================
// 从 record_gaps_test.go 合并：validateRecordData 边界
// ============================================================

func TestValidateRecordData_AttachmentFieldValidation(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "attach_val_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "attach_val_table"}
	require.NoError(t, db.Create(tbl).Error)

	fileField := &models.Field{
		TableID: tbl.ID,
		Name:    "doc",
		Type:    "file",
		Options: "{}",
	}
	require.NoError(t, db.Create(fileField).Error)

	err := s.validateRecordData(tbl.ID, map[string]interface{}{"doc": 12345}, "", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "doc")
}

func TestValidateRecordData_NilValueOnOptionalField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "nil_val_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "nil_val_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "optional_field",
		Type:    "string",
		Options: `{"max_length":5}`,
	}).Error)

	err := s.validateRecordData(tbl.ID, map[string]interface{}{"optional_field": nil}, "", "user1")
	assert.NoError(t, err)
}

func TestValidateRecordData_EmptyStringOnOptionalField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "empty_str_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "empty_str_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "opt_str",
		Type:    "string",
		Options: `{"max_length":3}`,
	}).Error)

	err := s.validateRecordData(tbl.ID, map[string]interface{}{"opt_str": ""}, "", "user1")
	assert.NoError(t, err)
}

// ============================================================
// 从 record_gaps_test.go 合并：BatchCreateRecords 边界
// ============================================================

func TestBatchCreateRecords_AttachmentFieldWithFileIDs(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "batch_attach_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "batch_attach_table"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{
		TableID: tbl.ID,
		Name:    "doc",
		Type:    "file",
		Options: "{}",
	}).Error)

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"doc": []string{"fil_123"}},
	}, "user1", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "批量创建暂不支持 file 字段")
}

func TestBatchCreateRecords_RequiredFieldMissing(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "batch_req_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "batch_req_table"}
	require.NoError(t, db.Create(tbl).Error)

	require.NoError(t, db.Create(&models.Field{
		TableID:  tbl.ID,
		Name:     "title",
		Type:     "string",
		Required: true,
	}).Error)

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{},
	}, "user1", 5)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必填")
}

func TestBatchCreateRecords_NonWritableField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	dbModel := &models.Database{Name: "batch_write_db"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "batch_write_table"}
	require.NoError(t, db.Create(tbl).Error)
	require.NoError(t, db.Create(&models.Field{TableID: tbl.ID, Name: "name", Type: "string"}).Error)

	viewer := &models.Token{
		Name:     "viewer_batch",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, db.Create(viewer).Error)
	authz.ClearTokenCache()

	_, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"name": "test"},
	}, viewer.ID, 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问该表")
}

// ============================================================
// 从 record_gaps_test.go 合并：结构化过滤错误路径
// ============================================================

func TestBuildStructuredFilterClauses_MalformedJSON(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{Name: "status", Type: "string"}}
	readable := map[string]models.Field{"status": {Name: "status"}}

	_, _, err := s.buildStructuredFilterClauses(fields, readable, map[string]interface{}{
		"status": json.Number("not-a-number"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "过滤值序列化失败")
}
