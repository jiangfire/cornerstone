package query

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func setupQueryTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupTestDBWithTokens(t, "user1")
}

func createTestData(t *testing.T, db *gorm.DB) (*models.Database, *models.Table) {
	authz.ClearTokenCache()
	dbModel := &models.Database{Name: "querytest"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)
	return dbModel, tbl
}

// ---------------------------------------------------------------------------
// Validator tests
// ---------------------------------------------------------------------------

func TestValidateRequest_NilRequest(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	err := v.ValidateRequest(context.Background(), nil, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "查询请求不能为空")
}

func TestValidateRequest_DisallowedTable(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	req := &QueryRequest{From: "secret_table", Select: []string{"*"}}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不在允许访问的列表中")
}

func TestValidateRequest_AllowedTable(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{From: "databases", Select: []string{"id", "name"}}
	err := v.ValidateRequest(context.Background(), req, "user1")
	assert.NoError(t, err)
}

func TestValidateRequest_ValidatesSelectFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{From: "databases", Select: []string{"id", "secret_col"}}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret_col")
}

func TestValidateRequest_ValidatesWhereFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{
		From:   "databases",
		Select: []string{"id"},
		Where: &WhereClause{
			And: []Condition{{Field: "forbidden_field", Op: "eq", Value: "x"}},
		},
	}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "forbidden_field")
}

func TestValidateRequest_ValidatesOrderByFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{
		From:    "databases",
		Select:  []string{"id"},
		OrderBy: []OrderByClause{{Field: "no_such_field", Dir: "asc"}},
	}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no_such_field")
}

func TestValidateRequest_ValidatesGroupByFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{
		From:    "databases",
		Select:  []string{"id"},
		GroupBy: []string{"bogus_col"},
	}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bogus_col")
}

func TestValidateRequest_ValidatesAggregateFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{
		From:      "databases",
		Select:    []string{"id"},
		Aggregate: []AggregateFunc{{Func: "count", Field: "invalid_col", As: "cnt"}},
	}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid_col")
}

func TestValidateRequest_ValidatesJoinFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{
		From:   "databases",
		Select: []string{"id"},
		Join: []JoinClause{
			{
				Type:   "left",
				Table:  "tables",
				As:     "t",
				On:     JoinCondition{Left: "databases.id", Op: "=", Right: "t.database_id"},
				Select: []string{"secret_join_col"},
			},
		},
	}
	err := v.ValidateRequest(context.Background(), req, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "secret_join_col")
}

func TestCheckTableAccess_AllowedTables(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	for _, table := range []string{"databases", "records", "tables", "fields", "files"} {
		err := v.CheckTableAccess(context.Background(), "user1", table)
		assert.NoError(t, err, "table %s should be allowed", table)
	}
}

func TestCheckTableAccess_DisallowedTable(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	err := v.CheckTableAccess(context.Background(), "user1", "users")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不在允许访问的列表中")
}

func TestCheckTableAccess_TokensRequiresMaster(t *testing.T) {
	db := setupQueryTestDB(t)

	nonMaster := &models.Token{ID: "nm1", Token: "cs_nm1", Name: "nm", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(nonMaster).Error)
	authz.ClearTokenCache()

	v := NewValidator(db)
	err := v.CheckTableAccess(context.Background(), "nm1", "tokens")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "无权访问 tokens")

	authz.ClearTokenCache()
	err = v.CheckTableAccess(context.Background(), "user1", "tokens")
	assert.NoError(t, err)
}

func TestCheckFieldAccess_AllowedField(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	err := v.CheckFieldAccess(context.Background(), "user1", "databases", "name")
	assert.NoError(t, err)
}

func TestCheckFieldAccess_DisallowedField(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	err := v.CheckFieldAccess(context.Background(), "user1", "databases", "internal_note")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "internal_note")
}

func TestCheckFieldAccess_JSONBaseFieldSplitting(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	err := v.CheckFieldAccess(context.Background(), "user1", "records", "data.status")
	assert.NoError(t, err)
}

func TestAutoFilterByPermission_DatabasesTable(t *testing.T) {
	db := setupQueryTestDB(t)
	dbModel, _ := createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{From: "databases", Select: []string{"id"}}
	err := v.AutoFilterByPermission(req, "user1")
	require.NoError(t, err)
	require.NotNil(t, req.Where)
	require.Len(t, req.Where.And, 1)
	assert.Equal(t, "databases.id", req.Where.And[0].Field)
	assert.Equal(t, "in", req.Where.And[0].Op)
	assert.Contains(t, req.Where.And[0].Value, dbModel.ID)
}

func TestAutoFilterByPermission_RecordsTable(t *testing.T) {
	db := setupQueryTestDB(t)
	_, tbl := createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{From: "records", Select: []string{"id"}}
	err := v.AutoFilterByPermission(req, "user1")
	require.NoError(t, err)
	require.NotNil(t, req.Where)
	assert.Equal(t, "records.table_id", req.Where.And[0].Field)
	assert.Contains(t, req.Where.And[0].Value, tbl.ID)
}

func TestAutoFilterByPermission_TablesTable(t *testing.T) {
	db := setupQueryTestDB(t)
	dbModel, _ := createTestData(t, db)
	v := NewValidator(db)
	req := &QueryRequest{From: "tables", Select: []string{"id"}}
	err := v.AutoFilterByPermission(req, "user1")
	require.NoError(t, err)
	require.NotNil(t, req.Where)
	assert.Equal(t, "tables.database_id", req.Where.And[0].Field)
	assert.Contains(t, req.Where.And[0].Value, dbModel.ID)
}

func TestAutoFilterByPermission_FilesTableWithRecordIDs(t *testing.T) {
	db := setupQueryTestDB(t)
	_, tbl := createTestData(t, db)
	rec := &models.Record{TableID: tbl.ID, Data: `{"name":"f1"}`}
	require.NoError(t, db.Create(rec).Error)
	authz.ClearTokenCache()

	v := NewValidator(db)
	req := &QueryRequest{From: "files", Select: []string{"id"}}
	err := v.AutoFilterByPermission(req, "user1")
	require.NoError(t, err)
	require.NotNil(t, req.Where)
	assert.Equal(t, "files.record_id", req.Where.And[0].Field)
}

func TestAutoFilterByPermission_EmptyAccessReturnsError(t *testing.T) {
	db := setupQueryTestDB(t)
	emptyTok := &models.Token{ID: "empty1", Token: "cs_empty1", Name: "empty", IsMaster: false, Scopes: `{}`}
	require.NoError(t, db.Create(emptyTok).Error)
	authz.ClearTokenCache()

	v := NewValidator(db)
	req := &QueryRequest{From: "databases", Select: []string{"id"}}
	err := v.AutoFilterByPermission(req, "empty1")
	require.Error(t, err)
}

func TestGetAllowedTables_MasterSeesAll(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	tables, err := v.GetAllowedTables(context.Background(), "user1")
	require.NoError(t, err)
	assert.Equal(t, len(DefaultAllowedTables), len(tables))
	found := false
	for _, t2 := range tables {
		if t2 == "tokens" {
			found = true
		}
	}
	assert.True(t, found, "master should see tokens table")
}

func TestGetAllowedTables_NonMasterExcludesTokens(t *testing.T) {
	db := setupQueryTestDB(t)
	nonMaster := &models.Token{ID: "nm2", Token: "cs_nm2", Name: "nm2", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(nonMaster).Error)
	authz.ClearTokenCache()

	v := NewValidator(db)
	tables, err := v.GetAllowedTables(context.Background(), "nm2")
	require.NoError(t, err)
	for _, t2 := range tables {
		assert.NotEqual(t, "tokens", t2)
	}
}

func TestFilterFieldsByPermission(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	data := []map[string]interface{}{
		{"id": "1", "name": "db1", "secret": "s1"},
		{"id": "2", "name": "db2", "secret": "s2"},
	}
	filtered, err := v.FilterFieldsByPermission(context.Background(), data, "databases", "user1")
	require.NoError(t, err)
	for _, row := range filtered {
		assert.Nil(t, row["secret"])
		assert.NotNil(t, row["id"])
		assert.NotNil(t, row["name"])
	}
}

func TestFilterFieldsByPermission_WildcardAllowed(t *testing.T) {
	db := setupQueryTestDB(t)
	customTables := AllowedTables{
		"my_table": {"*"},
	}
	v := NewValidatorWithTables(db, customTables)
	data := []map[string]interface{}{
		{"id": "1", "anything": "val", "extra": 42},
	}
	filtered, err := v.FilterFieldsByPermission(context.Background(), data, "my_table", "user1")
	require.NoError(t, err)
	assert.Equal(t, "val", filtered[0]["anything"])
	assert.Equal(t, 42, filtered[0]["extra"])
}

func TestGetSelectableFields(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	fields := v.GetSelectableFields("databases")
	assert.Equal(t, DefaultAllowedTables["databases"], fields)
}

func TestResolveReferenceTable(t *testing.T) {
	db := setupQueryTestDB(t)
	v := NewValidator(db)
	joins := []JoinClause{{Type: "left", Table: "tables", As: "t", On: JoinCondition{Left: "databases.id", Op: "=", Right: "t.database_id"}}}
	assert.Equal(t, "databases", v.resolveReferenceTable("databases", joins, "databases"))
	assert.Equal(t, "tables", v.resolveReferenceTable("databases", joins, "t"))
	assert.Equal(t, "", v.resolveReferenceTable("databases", joins, "unknown"))
}

func TestCheckFieldReference_DotQualifiedWithJoin(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	v := NewValidator(db)
	joins := []JoinClause{{Type: "left", Table: "tables", As: "t", On: JoinCondition{Left: "databases.id", Op: "=", Right: "t.database_id"}}}
	err := v.checkFieldReference(context.Background(), "user1", "databases", joins, "t.name")
	assert.NoError(t, err)
}

func TestSplitJSONBaseField(t *testing.T) {
	base, ok := splitJSONBaseField("data.status")
	assert.True(t, ok)
	assert.Equal(t, "data", base)

	_, ok = splitJSONBaseField("data->status")
	assert.False(t, ok)

	_, ok = splitJSONBaseField("name")
	assert.False(t, ok)
}

func TestQualifyBaseField(t *testing.T) {
	assert.Equal(t, "databases.id", qualifyBaseField("databases", "id"))
	assert.Equal(t, "databases.name", qualifyBaseField("databases", "name"))
	assert.Equal(t, "name", qualifyBaseField("", "name"))
	assert.Equal(t, "databases.name", qualifyBaseField("databases", "databases.name"))
	assert.Equal(t, "", qualifyBaseField("databases", ""))
}

// ---------------------------------------------------------------------------
// Executor tests
// ---------------------------------------------------------------------------

func TestExecute_BasicQueryOnDatabases(t *testing.T) {
	db := setupQueryTestDB(t)
	dbModel, _ := createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Page:   1,
		Size:   20,
	}, "user1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
	found := false
	for _, row := range result.Data {
		if row["id"] == dbModel.ID {
			found = true
		}
	}
	assert.True(t, found)
}

func TestExecute_QueryWithWhereConditions(t *testing.T) {
	db := setupQueryTestDB(t)
	dbModel, _ := createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Where: &WhereClause{
			And: []Condition{{Field: "name", Op: "eq", Value: "querytest"}},
		},
		Page: 1,
		Size: 20,
	}, "user1")
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, dbModel.ID, result.Data[0]["id"])
}

func TestExecute_QueryWithSelectFields(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "databases",
		Select: []string{"name"},
		Page:   1,
		Size:   20,
	}, "user1")
	require.NoError(t, err)
	for _, row := range result.Data {
		assert.Nil(t, row["description"])
		assert.NotNil(t, row["name"])
	}
}

func TestExecute_QueryWithOrderBy(t *testing.T) {
	db := setupQueryTestDB(t)
	require.NoError(t, db.Create(&models.Database{Name: "aaa"}).Error)
	require.NoError(t, db.Create(&models.Database{Name: "zzz"}).Error)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:    "databases",
		Select:  []string{"name"},
		OrderBy: []OrderByClause{{Field: "name", Dir: "desc"}},
		Page:    1,
		Size:    20,
	}, "user1")
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(result.Data), 2)
	assert.Equal(t, "zzz", result.Data[0]["name"])
}

func TestExecute_Pagination(t *testing.T) {
	db := setupQueryTestDB(t)
	for i := 0; i < 5; i++ {
		require.NoError(t, db.Create(&models.Database{Name: "page_test_" + string(rune('a'+i))}).Error)
	}
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "databases",
		Select: []string{"id"},
		Page:   1,
		Size:   2,
	}, "user1")
	require.NoError(t, err)
	assert.LessOrEqual(t, len(result.Data), 2)
	assert.True(t, result.HasMore)
	assert.Equal(t, 1, result.Page)
	assert.Equal(t, 2, result.Size)
}

func TestExecuteRaw(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	raw := map[string]interface{}{
		"from":   "databases",
		"select": []string{"id", "name"},
		"page":   1,
		"size":   20,
	}
	jsonData, err := json.Marshal(raw)
	require.NoError(t, err)
	result, err := executor.ExecuteRaw(context.Background(), jsonData, "user1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
}

func TestExecuteFromMap(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.ExecuteFromMap(context.Background(), map[string]interface{}{
		"from":   "databases",
		"select": []string{"id", "name"},
		"page":   1,
		"size":   20,
	}, "user1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
}

func TestExecuteBatch(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	batch := &BatchQueryRequest{
		Queries: map[string]QueryRequest{
			"dbs": {
				From:   "databases",
				Select: []string{"id"},
				Page:   1,
				Size:   20,
			},
		},
	}
	result, err := executor.ExecuteBatch(context.Background(), batch, "user1")
	require.NoError(t, err)
	assert.Contains(t, result.Results, "dbs")
	assert.GreaterOrEqual(t, result.Results["dbs"].Total, int64(1))
}

func TestExecuteBatchRaw(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	raw := map[string]interface{}{
		"queries": map[string]interface{}{
			"dbs": map[string]interface{}{
				"from":   "databases",
				"select": []string{"id"},
				"page":   1,
				"size":   20,
			},
		},
	}
	jsonData, err := json.Marshal(raw)
	require.NoError(t, err)
	result, err := executor.ExecuteBatchRaw(context.Background(), jsonData, "user1")
	require.NoError(t, err)
	assert.Contains(t, result.Results, "dbs")
}

func TestValidate_ValidatesWithoutExecuting(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	req := &QueryRequest{From: "databases", Select: []string{"id"}, Page: 1, Size: 20}
	err := executor.Validate(context.Background(), req, "user1")
	assert.NoError(t, err)
}

func TestExplain_GeneratesSQLWithoutExecuting(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	req := &QueryRequest{From: "databases", Select: []string{"id", "name"}, Page: 1, Size: 10}
	require.NoError(t, executor.normalize(req))
	sqlQuery, err := executor.Explain(req)
	require.NoError(t, err)
	assert.Contains(t, sqlQuery.SQL, "databases")
	assert.NotEmpty(t, sqlQuery.Params)
}

func TestExplainAuthorized(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	req := &QueryRequest{From: "databases", Select: []string{"id"}, Page: 1, Size: 10}
	sqlQuery, err := executor.ExplainAuthorized(context.Background(), req, "user1")
	require.NoError(t, err)
	assert.Contains(t, sqlQuery.SQL, "databases")
}

func TestSimplifiedQuery(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	result, err := executor.SimplifiedQuery(context.Background(), "databases", map[string]interface{}{"name": "querytest"}, "", 1, 20, "user1")
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
}

func TestPrepare_NormalizesAndValidates(t *testing.T) {
	db := setupQueryTestDB(t)
	createTestData(t, db)
	authz.ClearTokenCache()

	executor := NewExecutor(db)
	req := &QueryRequest{Table: "databases", Page: 0, Size: 0}
	err := executor.Prepare(context.Background(), req, "user1")
	require.NoError(t, err)
	assert.Equal(t, "databases", req.From)
	assert.Equal(t, 1, req.Page)
	assert.Equal(t, 20, req.Size)
	assert.Contains(t, req.Select, "id")
}

func TestNormalize_SetsDefaults(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	req := &QueryRequest{From: "databases"}
	err := executor.normalize(req)
	require.NoError(t, err)
	assert.Equal(t, 1, req.Page)
	assert.Equal(t, 20, req.Size)
	assert.Equal(t, []string{"*"}, req.Select)
}

func TestNormalize_MissingFromTable(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	req := &QueryRequest{}
	err := executor.normalize(req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "必须指定表名")
}

func TestNormalize_ConvertsFilterToWhere(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	req := &QueryRequest{
		From:   "databases",
		Filter: map[string]interface{}{"name": "test"},
	}
	err := executor.normalize(req)
	require.NoError(t, err)
	require.NotNil(t, req.Where)
}

func TestNormalize_ConvertsSortToOrderBy(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	req := &QueryRequest{
		From: "databases",
		Sort: "-name",
	}
	err := executor.normalize(req)
	require.NoError(t, err)
	require.Len(t, req.OrderBy, 1)
	assert.Equal(t, "name", req.OrderBy[0].Field)
	assert.Equal(t, "desc", req.OrderBy[0].Dir)
}

func TestExpandWildcardSelections(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	req := &QueryRequest{
		From:   "databases",
		Select: []string{"*"},
	}
	executor.expandWildcardSelections(req)
	assert.NotEqual(t, []string{"*"}, req.Select)
	assert.Greater(t, len(req.Select), 1)
}

func TestNewExecutorWithConfig(t *testing.T) {
	db := setupQueryTestDB(t)
	customLimits := QueryLimits{MaxPageSize: 50, MaxJoins: 1, MaxDepth: 2, MaxRows: 100, MaxFields: 10}
	customTables := AllowedTables{
		"databases": {"id", "name"},
	}
	executor := NewExecutorWithConfig(db, customLimits, customTables)
	assert.NotNil(t, executor)
	assert.Equal(t, customLimits, executor.limits)
	assert.NotNil(t, executor.validator)
}

func TestWithDB(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	execCopy := executor.WithDB(db)
	assert.NotNil(t, execCopy)
	assert.Equal(t, executor.parser, execCopy.parser)
	assert.Equal(t, executor.validator, execCopy.validator)
	assert.Equal(t, executor.generator, execCopy.generator)
	assert.Equal(t, executor.limits, execCopy.limits)
	assert.NotSame(t, executor, execCopy)
}

func TestExecutorAccessors(t *testing.T) {
	db := setupQueryTestDB(t)
	executor := NewExecutor(db)
	assert.NotNil(t, executor.GetParser())
	assert.NotNil(t, executor.GetValidator())
	assert.NotNil(t, executor.GetGenerator())
	assert.Equal(t, db, executor.DB())
}
