package services

import (
	"encoding/json"
	"fmt"
	"strings"
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

func TestBuildRecordFieldIndexRows_SupportedScalarValues(t *testing.T) {
	fields := []models.Field{
		{ID: "fld_status", TableID: "tbl_1", Name: "status", Type: "string"},
		{ID: "fld_title", TableID: "tbl_1", Name: "title", Type: "text"},
		{ID: "fld_due", TableID: "tbl_1", Name: "due_on", Type: "date"},
		{ID: "fld_score", TableID: "tbl_1", Name: "score", Type: "number"},
		{ID: "fld_active", TableID: "tbl_1", Name: "active", Type: "boolean"},
		{ID: "fld_meta", TableID: "tbl_1", Name: "meta", Type: "json"},
	}

	rows, err := buildRecordFieldIndexRows("tbl_1", "rec_1", fields, map[string]interface{}{
		"status": "paid",
		"title":  "Invoice",
		"due_on": "2026-06-06",
		"score":  float64(42.5),
		"active": true,
		"meta":   map[string]interface{}{"tier": "gold"},
	})

	require.NoError(t, err)
	require.Len(t, rows, 6)

	byField := make(map[string]models.RecordFieldIndex, len(rows))
	for _, row := range rows {
		byField[row.FieldName] = row
	}

	assert.Equal(t, "text", byField["status"].ValueType)
	assert.Equal(t, "paid", byField["status"].ValueText)
	assert.Equal(t, "Invoice", byField["title"].ValueText)
	assert.Equal(t, "2026-06-06", byField["due_on"].ValueText)
	assert.Equal(t, "number", byField["score"].ValueType)
	require.NotNil(t, byField["score"].ValueNumber)
	assert.Equal(t, 42.5, *byField["score"].ValueNumber)
	assert.Equal(t, "bool", byField["active"].ValueType)
	require.NotNil(t, byField["active"].ValueBool)
	assert.True(t, *byField["active"].ValueBool)
	assert.Equal(t, "text", byField["meta"].ValueType)
	assert.Contains(t, byField["meta"].ValueText, `"tier"`)
}

func TestBuildRecordFieldIndexRows_SkipsUnsupportedValues(t *testing.T) {
	fields := []models.Field{
		{ID: "fld_tags", TableID: "tbl_1", Name: "tags", Type: "list"},
		{ID: "fld_doc", TableID: "tbl_1", Name: "doc", Type: "file"},
		{ID: "fld_empty", TableID: "tbl_1", Name: "empty", Type: "string"},
		{ID: "fld_long", TableID: "tbl_1", Name: "long_text", Type: "text"},
	}

	rows, err := buildRecordFieldIndexRows("tbl_1", "rec_1", fields, map[string]interface{}{
		"tags":      []interface{}{"a", "b"},
		"doc":       "fil_1",
		"empty":     nil,
		"long_text": strings.Repeat("x", maxRecordFieldIndexTextLength+1),
	})

	require.NoError(t, err)
	assert.Empty(t, rows)
}

func TestRecordFieldIndexSync_CreateUpdateDeleteBatch(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	_, tbl, _ := createTestTableWithFields(t, db, "user1", "IndexSyncDB", "records",
		struct {
			Name     string
			Type     string
			Required bool
		}{"status", "string", false},
		struct {
			Name     string
			Type     string
			Required bool
		}{"score", "number", false},
	)

	record, err := s.CreateRecord(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"status": "paid", "score": float64(10)},
	}, "user1")
	require.NoError(t, err)

	var indexes []models.RecordFieldIndex
	require.NoError(t, db.Where("record_id = ? AND deleted_at IS NULL", record.ID).Find(&indexes).Error)
	require.Len(t, indexes, 2)

	_, err = s.UpdateRecord(record.ID, UpdateRecordRequest{
		Data:    map[string]interface{}{"status": "refunded", "score": float64(20)},
		Version: 1,
	}, "user1")
	require.NoError(t, err)

	indexes = nil
	require.NoError(t, db.Where("record_id = ? AND deleted_at IS NULL", record.ID).Find(&indexes).Error)
	require.Len(t, indexes, 2)
	for _, idx := range indexes {
		if idx.FieldName == "status" {
			assert.Equal(t, "refunded", idx.ValueText)
		}
		if idx.FieldName == "score" {
			require.NotNil(t, idx.ValueNumber)
			assert.Equal(t, 20.0, *idx.ValueNumber)
		}
	}

	require.NoError(t, s.DeleteRecord(record.ID, "user1"))
	var liveCount int64
	require.NoError(t, db.Model(&models.RecordFieldIndex{}).Where("record_id = ? AND deleted_at IS NULL", record.ID).Count(&liveCount).Error)
	assert.Equal(t, int64(0), liveCount)

	batchRecords, err := s.BatchCreateRecords(CreateRecordRequest{
		TableID: tbl.ID,
		Data:    map[string]interface{}{"status": "batch", "score": float64(30)},
	}, "user1", 3)
	require.NoError(t, err)
	require.Len(t, batchRecords, 3)

	var batchIndexCount int64
	require.NoError(t, db.Model(&models.RecordFieldIndex{}).
		Where("table_id = ? AND deleted_at IS NULL", tbl.ID).
		Where("value_text = ? OR value_number = ?", "batch", 30.0).
		Count(&batchIndexCount).Error)
	assert.Equal(t, int64(6), batchIndexCount)
}

func TestBuildStructuredFilterClauses_MySQLUsesRecordFieldIndexForSupportedScalar(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{
		{ID: "fld_status", Name: "status", Type: "string"},
		{ID: "fld_score", Name: "score", Type: "number"},
		{ID: "fld_active", Name: "active", Type: "boolean"},
	}
	readable := map[string]models.Field{
		"status": fields[0],
		"score":  fields[1],
		"active": fields[2],
	}

	clauses, refsHidden, err := s.buildStructuredFilterClausesForDB("mysql", fields, readable, map[string]interface{}{
		"status": "paid",
		"score":  float64(42),
		"active": true,
	})

	require.NoError(t, err)
	assert.False(t, refsHidden)
	require.Len(t, clauses, 3)
	for _, clause := range clauses {
		assert.Contains(t, clause.sql, "record_field_indexes")
		assert.Contains(t, clause.sql, "record_id = records.id")
		assert.NotContains(t, clause.sql, "JSON_EXTRACT")
	}
	argsByField := make(map[string][]interface{}, len(clauses))
	for _, clause := range clauses {
		argsByField[clause.args[0].(string)] = clause.args
	}
	assert.Equal(t, []interface{}{"fld_status", "paid"}, argsByField["fld_status"])
	assert.Equal(t, []interface{}{"fld_score", float64(42)}, argsByField["fld_score"])
	assert.Equal(t, []interface{}{"fld_active", true}, argsByField["fld_active"])
}

func TestBuildStructuredFilterClauses_MySQLFallsBackForUnsupportedValues(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{ID: "fld_tags", Name: "tags", Type: "list"}}
	readable := map[string]models.Field{"tags": fields[0]}

	clauses, refsHidden, err := s.buildStructuredFilterClausesForDB("mysql", fields, readable, map[string]interface{}{
		"tags": []interface{}{"a", "b"},
	})

	require.NoError(t, err)
	assert.False(t, refsHidden)
	require.Len(t, clauses, 1)
	assert.Equal(t, "JSON_EXTRACT(data, ?) = ?", clauses[0].sql)
}

func TestBuildStructuredFilterClauses_MySQLFallsBackForJSONField(t *testing.T) {
	db := setupTestDB(t)
	s := NewRecordService(db)

	fields := []models.Field{{ID: "fld_meta", Name: "meta", Type: "json"}}
	readable := map[string]models.Field{"meta": fields[0]}

	clauses, refsHidden, err := s.buildStructuredFilterClausesForDB("mysql", fields, readable, map[string]interface{}{
		"meta": map[string]interface{}{"tier": "gold"},
	})

	require.NoError(t, err)
	assert.False(t, refsHidden)
	require.Len(t, clauses, 1)
	assert.Equal(t, "JSON_EXTRACT(data, ?) = ?", clauses[0].sql)
}

func TestBuildMySQLRecordListSQL_NoFilter(t *testing.T) {
	sql, args := buildMySQLRecordListSQL(QueryRequest{
		TableID: "tbl_1",
		Limit:   50,
		Offset:  10,
	}, nil)

	assert.Equal(t, "SELECT id, table_id, data, version, created_at, updated_at FROM records FORCE INDEX (idx_records_table_deleted_created) WHERE table_id = ? AND deleted_at IS NULL ORDER BY created_at DESC LIMIT ? OFFSET ?", sql)
	assert.Equal(t, []interface{}{"tbl_1", 50, 10}, args)
}

func TestBuildMySQLRecordListSQL_WithStructuredClauses(t *testing.T) {
	sql, args := buildMySQLRecordListSQL(QueryRequest{
		TableID: "tbl_1",
		Limit:   20,
		Offset:  0,
	}, []recordFilterClause{
		{sql: "JSON_EXTRACT(data, ?) = ?", args: []interface{}{"$.status", "paid"}},
		{sql: "JSON_EXTRACT(data, ?) = ?", args: []interface{}{"$.category", "beta"}},
	})

	assert.Equal(t, "SELECT id, table_id, data, version, created_at, updated_at FROM records FORCE INDEX (idx_records_table_deleted_created) WHERE table_id = ? AND deleted_at IS NULL AND JSON_EXTRACT(data, ?) = ? AND JSON_EXTRACT(data, ?) = ? ORDER BY created_at DESC LIMIT ? OFFSET ?", sql)
	assert.Equal(t, []interface{}{"tbl_1", "$.status", "paid", "$.category", "beta", 20, 0}, args)
}

func TestBuildMySQLRecordCountSQL_WithStructuredClauses(t *testing.T) {
	sql, args := buildMySQLRecordCountSQL("tbl_1", []recordFilterClause{
		{sql: "JSON_EXTRACT(data, ?) = ?", args: []interface{}{"$.status", "paid"}},
	})

	assert.Equal(t, "SELECT COUNT(*) FROM records FORCE INDEX (idx_records_table_deleted_created) WHERE table_id = ? AND deleted_at IS NULL AND JSON_EXTRACT(data, ?) = ?", sql)
	assert.Equal(t, []interface{}{"tbl_1", "$.status", "paid"}, args)
}
