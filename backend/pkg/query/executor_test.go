package query

import (
	"context"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestExecutor_SQLite(t *testing.T) {
	// 创建内存 SQLite 数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	// 自动迁移表结构
	if err := db.AutoMigrate(&testRecord{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	// 创建执行器
	executor := NewExecutor(db)

	// 插入测试数据
	for i := 1; i <= 5; i++ {
		db.Create(&testRecord{
			ID:      i,
			Name:    "record",
			Status:  "active",
			Version: i * 10,
		})
	}

	// 测试查询
	req := &QueryRequest{
		From:   "test_records",
		Select: []string{"id", "name", "status"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "status", Op: "eq", Value: "active"},
			},
		},
		Page: 1,
		Size: 10,
	}

	// 验证生成的 SQL
	sqlQuery, err := executor.Explain(req)
	if err != nil {
		t.Fatalf("Explain failed: %v", err)
	}

	// SQLite 应该生成正确的 SQL
	if sqlQuery.SQL == "" {
		t.Error("expected non-empty SQL")
	}

	t.Logf("Generated SQL: %s", sqlQuery.SQL)
	t.Logf("Params: %v", sqlQuery.Params)
}

func TestExecutor_DBTypeDetection(t *testing.T) {
	// PostgreSQL 风格的查询
	postgresDB, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	exec1 := NewExecutor(postgresDB)

	if !exec1.generator.isSQLite {
		t.Error("expected isSQLite to be true for sqlite db")
	}

	// 验证 Explain 不返回错误
	req := &QueryRequest{
		From:   "test",
		Select: []string{"id"},
	}
	_, err := exec1.Explain(req)
	if err != nil {
		t.Errorf("Explain failed: %v", err)
	}
}

func TestExecutor_PrepareExpandsWildcardSelection(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	executor := NewExecutor(db)
	req := &QueryRequest{
		From:   "users",
		Select: []string{"*"},
	}

	if err := executor.Prepare(context.Background(), req, "usr_any"); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	for _, field := range req.Select {
		if field == "*" {
			t.Fatal("wildcard should be expanded before execution")
		}
		if field == "password" {
			t.Fatal("password should never be expanded from wildcard selection")
		}
	}
}

func TestExecutor_PrepareInjectsPermissionFilterWithoutDroppingExistingWhere(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DatabaseAccess{}, &models.Table{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}

	if err := db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error; err != nil {
		t.Fatalf("failed to seed access: %v", err)
	}

	executor := NewExecutor(db)
	req := &QueryRequest{
		From:   "tables",
		Select: []string{"id", "name"},
		Where: &WhereClause{
			And: []Condition{
				{Field: "name", Op: "eq", Value: "Orders"},
			},
		},
		Page: 1,
		Size: 20,
	}

	if err := executor.Prepare(context.Background(), req, "usr_reader"); err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	if len(req.Where.And) != 2 {
		t.Fatalf("expected permission filter plus original where, got %#v", req.Where.And)
	}
	if req.Where.And[0].Field != "tables.database_id" || req.Where.And[1].Field != "name" {
		t.Fatalf("unexpected where order after Prepare: %#v", req.Where.And)
	}
}

func TestExecutor_ValidateRejectsAdminTableForViewer(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&models.DatabaseAccess{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	if err := db.Create(&models.DatabaseAccess{
		UserID:     "usr_viewer",
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error; err != nil {
		t.Fatalf("failed to seed access: %v", err)
	}

	executor := NewExecutor(db)
	req := &QueryRequest{
		From:   "database_access",
		Select: []string{"id", "database_id", "role"},
		Page:   1,
		Size:   20,
	}

	err = executor.Validate(context.Background(), req, "usr_viewer")
	if err == nil {
		t.Fatal("expected validation error for viewer querying admin table")
	}
}

func TestExecutor_PrepareRejectsNestedNonJSONField(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}

	executor := NewExecutor(db)
	req := &QueryRequest{
		From:   "users",
		Select: []string{"email.domain"},
		Page:   1,
		Size:   20,
	}

	err = executor.Prepare(context.Background(), req, "usr_any")
	if err == nil {
		t.Fatal("expected Prepare to reject nested non-JSON field")
	}
}

func TestExecutor_ExecuteSetsHasMoreAcrossPages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DatabaseAccess{}, &models.Table{}, &models.Record{}))

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "Orders",
	}).Error)
	for _, rec := range []models.Record{
		{ID: "rec_1", TableID: "tbl_allowed", Data: `{"seq":1}`, CreatedBy: "usr_reader", UpdatedBy: "usr_reader", Version: 1},
		{ID: "rec_2", TableID: "tbl_allowed", Data: `{"seq":2}`, CreatedBy: "usr_reader", UpdatedBy: "usr_reader", Version: 1},
		{ID: "rec_3", TableID: "tbl_allowed", Data: `{"seq":3}`, CreatedBy: "usr_reader", UpdatedBy: "usr_reader", Version: 1},
	} {
		require.NoError(t, db.Create(&rec).Error)
	}

	executor := NewExecutor(db)

	firstPage, err := executor.Execute(context.Background(), &QueryRequest{
		From:    "records",
		Select:  []string{"id", "table_id"},
		OrderBy: []OrderByClause{{Field: "id", Dir: "asc"}},
		Page:    1,
		Size:    2,
	}, "usr_reader")
	require.NoError(t, err)
	require.EqualValues(t, 3, firstPage.Total)
	require.Len(t, firstPage.Data, 2)
	require.True(t, firstPage.HasMore)

	secondPage, err := executor.Execute(context.Background(), &QueryRequest{
		From:    "records",
		Select:  []string{"id", "table_id"},
		OrderBy: []OrderByClause{{Field: "id", Dir: "asc"}},
		Page:    2,
		Size:    2,
	}, "usr_reader")
	require.NoError(t, err)
	require.EqualValues(t, 3, secondPage.Total)
	require.Len(t, secondPage.Data, 1)
	require.False(t, secondPage.HasMore)
}

func TestExecutor_ExecuteReturnsEmptySliceForNoRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.DatabaseAccess{}, &models.Table{}, &models.Record{}))

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     "usr_reader",
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, db.Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "Orders",
	}).Error)

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "records",
		Select: []string{"id", "table_id"},
		Page:   1,
		Size:   20,
	}, "usr_reader")
	require.NoError(t, err)
	require.NotNil(t, result.Data)
	require.Len(t, result.Data, 0)
	require.EqualValues(t, 0, result.Total)
	require.False(t, result.HasMore)
}

func TestExecutor_SimplifiedQueryRejectsInvalidFilterOperator(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	executor := NewExecutor(db)
	_, err = executor.SimplifiedQuery(context.Background(), "users", map[string]interface{}{
		"email": map[string]interface{}{
			"contains": "example.com",
		},
	}, "", 1, 20, "usr_any")
	require.Error(t, err)
	require.Contains(t, err.Error(), "字段 'email' 包含无效操作符")
}

func TestExecutor_ExecuteBatchRawIncludesFailingQueryNameForNormalizeError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	executor := NewExecutor(db)
	_, err = executor.ExecuteBatchRaw(context.Background(), []byte(`{
		"queries":{
			"safe":{"from":"users","page":1,"size":20},
			"broken":{"select":["id"],"page":1,"size":20}
		}
	}`), "usr_any")
	require.Error(t, err)
	require.Contains(t, err.Error(), "查询 'broken' 格式错误")
}

func TestExecutor_ExecuteBatchRawIncludesFailingQueryNameForValidateError(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	executor := NewExecutor(db)
	_, err = executor.ExecuteBatchRaw(context.Background(), []byte(`{
		"queries":{
			"safe":{"from":"users","page":1,"size":20},
			"oversize":{"from":"users","page":1,"size":1001}
		}
	}`), "usr_any")
	require.Error(t, err)
	require.Contains(t, err.Error(), "查询 'oversize' 验证失败")
}

type testRecord struct {
	ID      int
	Name    string
	Status  string
	Version int
}

func (testRecord) TableName() string {
	return "test_records"
}
