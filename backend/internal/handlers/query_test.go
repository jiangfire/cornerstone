package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/config"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	pkgdb "github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/utils"
	"github.com/stretchr/testify/require"
)

func setupQueryHandlerTest(t *testing.T) (*gin.Engine, models.User) {
	t.Helper()

	gin.SetMode(gin.TestMode)

	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-query-handler")
	dbFile := t.TempDir() + "\\query-handler-test.db"
	cfg := config.DatabaseConfig{
		Type: "sqlite",
		URL:  dbFile,
	}
	require.NoError(t, pkgdb.InitDB(cfg))
	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
	})

	require.NoError(t, pkgdb.DB().AutoMigrate(
		&models.User{},
		&models.TokenBlacklist{},
		&models.Database{},
		&models.DatabaseAccess{},
		&models.Table{},
		&models.Record{},
		&models.Organization{},
		&models.OrganizationMember{},
		&models.FieldPermission{},
		&models.ActivityLog{},
		&models.File{},
		&models.Plugin{},
		&models.PluginBinding{},
		&models.PluginExecution{},
	))

	user := models.User{
		Username: "query_http_user",
		Email:    "query_http@example.com",
		Password: "hashed",
	}
	require.NoError(t, pkgdb.DB().Create(&user).Error)

	queryHandler := NewQueryHandler()
	r := gin.New()
	protected := r.Group("/api")
	protected.Use(middleware.Auth())
	protected.GET("/query", queryHandler.Query)
	protected.POST("/query", queryHandler.Query)
	protected.GET("/query/simple", queryHandler.SimplifiedQuery)
	protected.POST("/query/validate", queryHandler.QueryValidate)
	protected.POST("/query/explain", queryHandler.QueryExplain)
	protected.POST("/query/batch", queryHandler.BatchQuery)
	protected.GET("/query/tables", queryHandler.ListTables)
	protected.GET("/query/schema/:table", queryHandler.GetTableSchema)

	return r, user
}

func authHeaderForQueryUser(t *testing.T, user models.User) string {
	t.Helper()
	token, err := utils.GenerateToken(user.ID, user.Username, "user")
	require.NoError(t, err)
	return "Bearer " + token
}

func TestQueryHandlerQueryAutoFiltersTablesByDatabaseAccess(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedOrders",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_blocked",
		DatabaseID: "db_blocked",
		Name:       "BlockedOrders",
	}).Error)

	q := map[string]interface{}{
		"from":   "tables",
		"select": []string{"id", "database_id", "name"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)

	firstRow := rows[0].(map[string]interface{})
	require.Equal(t, "tbl_allowed", firstRow["id"])
	require.Equal(t, "db_allowed", firstRow["database_id"])
	require.EqualValues(t, 1, data["total"])
}

func TestQueryHandlerQueryDatabasesFiltersToAccessibleOnes(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.Database{
		ID:      "db_allowed",
		Name:    "AllowedDB",
		OwnerID: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Database{
		ID:      "db_blocked",
		Name:    "BlockedDB",
		OwnerID: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)

	q := map[string]interface{}{
		"from":   "databases",
		"select": []string{"id", "name", "owner_id"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)
	require.Equal(t, "db_allowed", rows[0].(map[string]interface{})["id"])
	require.EqualValues(t, 1, data["total"])
}

func TestQueryHandlerQueryAggregateByJSONFieldReturnsGroupedCounts(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	for _, rec := range []models.Record{
		{ID: "rec_1", TableID: "tbl_allowed", Data: `{"status":"approved"}`, CreatedBy: user.ID, UpdatedBy: user.ID, Version: 1},
		{ID: "rec_2", TableID: "tbl_allowed", Data: `{"status":"approved"}`, CreatedBy: user.ID, UpdatedBy: user.ID, Version: 1},
		{ID: "rec_3", TableID: "tbl_allowed", Data: `{"status":"draft"}`, CreatedBy: user.ID, UpdatedBy: user.ID, Version: 1},
	} {
		require.NoError(t, pkgdb.DB().Create(&rec).Error)
	}

	q := map[string]interface{}{
		"from":      "records",
		"select":    []string{"data.status"},
		"aggregate": []map[string]interface{}{{"func": "count", "field": "*", "as": "total"}},
		"groupBy":   []string{"data.status"},
		"orderBy":   []map[string]interface{}{{"field": "data.status", "dir": "asc"}},
		"page":      1,
		"size":      20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)
	require.EqualValues(t, 2, data["total"])

	counts := make(map[string]float64)
	for _, item := range rows {
		row := item.(map[string]interface{})
		var status string
		for key, value := range row {
			if key != "total" {
				status = value.(string)
				break
			}
		}
		counts[status] = row["total"].(float64)
	}
	require.Equal(t, map[string]float64{"approved": 2, "draft": 1}, counts)
}

func TestQueryHandlerValidateRejectsAdminTableForViewer(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	reqBody := []byte(`{"from":"database_access","select":["id","database_id","role"],"page":1,"size":20}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/validate", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "管理员权限")
}

func TestQueryHandlerExplainIncludesPermissionFilter(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)

	reqBody := []byte(`{
		"from":"tables",
		"select":["id","name"],
		"where":{"and":[{"field":"name","op":"eq","value":"Orders"}]},
		"page":1,
		"size":20
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/explain", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	sqlText := data["sql"].(string)
	params := data["params"].([]interface{})

	require.Contains(t, sqlText, "database_id")
	require.Contains(t, sqlText, "name")
	require.Contains(t, params, "db_allowed")
	require.Contains(t, params, "Orders")
}

func TestQueryHandlerExplainRejectsMalformedJSONBody(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/query/explain", bytes.NewBufferString(`{"from":`))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "请求格式错误")
}

func TestQueryHandlerExplainPluginsIncludesCreatorFilter(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	reqBody := []byte(`{
		"from":"plugins",
		"select":["id","name","created_by"],
		"where":{"and":[{"field":"name","op":"like","value":"Owned"}]},
		"page":1,
		"size":20
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/explain", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	sqlText := data["sql"].(string)
	params := data["params"].([]interface{})

	require.Contains(t, sqlText, `"plugins"."created_by"`)
	require.Contains(t, sqlText, `"name" LIKE ?`)
	require.Contains(t, params, user.ID)
	require.Contains(t, params, "%Owned%")
}

func TestQueryHandlerExplainAggregateJSONGroupByUsesJSONExpressions(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)

	reqBody := []byte(`{
		"from":"records",
		"select":["data.status","records.table_id"],
		"aggregate":[{"func":"count","field":"*","as":"total"}],
		"groupBy":["data.status","records.table_id"],
		"page":1,
		"size":20
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/explain", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	sqlText := data["sql"].(string)

	require.Contains(t, sqlText, `SELECT JSON_EXTRACT("data", '$.status'), "records"."table_id", COUNT(*) AS "total"`)
	require.Contains(t, sqlText, `GROUP BY JSON_EXTRACT("data", '$.status'), "records"."table_id"`)
	require.Contains(t, sqlText, `"records"."table_id" IN`)
}

func TestQueryHandlerExplainActivityLogsIncludesUserAndSystemScope(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	reqBody := []byte(`{
		"from":"activity_logs",
		"select":["id","user_id","action"],
		"where":{"and":[{"field":"action","op":"eq","value":"query"}]},
		"page":1,
		"size":20
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/explain", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	sqlText := data["sql"].(string)
	params := data["params"].([]interface{})

	require.Contains(t, sqlText, `"activity_logs"."user_id" = ?`)
	require.Contains(t, sqlText, `"activity_logs"."user_id" LIKE ?`)
	require.Contains(t, sqlText, `"action" = ?`)
	require.Contains(t, params, user.ID)
	require.Contains(t, params, "system:%")
	require.Contains(t, params, "query")
}

func TestQueryHandlerSchemaUsersExcludesPassword(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/users", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	fields := data["fields"].([]interface{})
	require.NotContains(t, fields, "password")
	require.Contains(t, fields, "email")
}

func TestQueryHandlerSchemaDatabaseAccessAllowedForAdmin(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/database_access", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	fields := data["fields"].([]interface{})
	require.Contains(t, fields, "database_id")
	require.Contains(t, fields, "role")
}

func TestQueryHandlerSchemaDatabasesAllowedForViewerWithAccess(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/databases", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	fields := data["fields"].([]interface{})
	require.Contains(t, fields, "id")
	require.Contains(t, fields, "owner_id")
}

func TestQueryHandlerSchemaDatabaseAccessForbiddenForViewer(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/database_access", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "管理员权限")
}

func TestQueryHandlerSchemaDatabasesForbiddenWithoutAccess(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/databases", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "您没有访问任何数据库的权限")
}

func TestQueryHandlerSchemaFilesForbiddenWithoutAccess(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/files", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "您没有访问任何数据库的权限")
}

func TestQueryHandlerSchemaFilesAllowedForViewerWithAccess(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/schema/files", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	fields := data["fields"].([]interface{})
	require.Contains(t, fields, "record_id")
	require.Contains(t, fields, "file_name")
}

func TestQueryHandlerListTablesRespectsAdminScope(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/tables", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	tables := data["tables"].([]interface{})

	require.Contains(t, tables, "tables")
	require.Contains(t, tables, "records")
	require.NotContains(t, tables, "database_access")
	require.NotContains(t, tables, "field_permissions")
}

func TestQueryHandlerListTablesIncludesAdminTablesForOwner(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_owner",
		Role:       "owner",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/tables", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	tables := data["tables"].([]interface{})

	require.Contains(t, tables, "database_access")
	require.Contains(t, tables, "field_permissions")
}

func TestQueryHandlerListTablesIncludesDatabasesAndFilesForViewerWithDatabaseAccess(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	req := httptest.NewRequest(http.MethodGet, "/api/query/tables", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	tables := data["tables"].([]interface{})

	require.Contains(t, tables, "databases")
	require.Contains(t, tables, "files")
}

func TestQueryHandlerValidateRejectsNestedNonJSONField(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	reqBody := []byte(`{"from":"users","select":["email.domain"],"page":1,"size":20}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/validate", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "users.email.domain")
}

func TestQueryHandlerValidateRejectsMalformedJSONBody(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/query/validate", bytes.NewBufferString(`{"from":`))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "请求格式错误")
}

func TestQueryHandlerSimplifiedQueryRejectsInvalidFilterOperator(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	filter := url.QueryEscape(`{"email":{"contains":"example.com"}}`)
	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=users&filter="+filter+"&page=1&size=20", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "字段 'email' 包含无效操作符")
}

func TestQueryHandlerSimplifiedQueryRejectsEmptyInFilter(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	filter := url.QueryEscape(`{"id":{"in":[]}}`)
	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=users&filter="+filter+"&page=1&size=20", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "'in' 操作符数组不能为空")
}

func TestQueryHandlerSimplifiedQueryPaginatesAndReportsHasMore(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedRecordsTable",
	}).Error)
	for _, rec := range []models.Record{
		{ID: "rec_1", TableID: "tbl_allowed", Data: `{"status":"ok"}`, CreatedBy: user.ID, UpdatedBy: user.ID, Version: 1},
		{ID: "rec_2", TableID: "tbl_allowed", Data: `{"status":"ok"}`, CreatedBy: user.ID, UpdatedBy: user.ID, Version: 1},
		{ID: "rec_3", TableID: "tbl_allowed", Data: `{"status":"ok"}`, CreatedBy: user.ID, UpdatedBy: user.ID, Version: 1},
	} {
		require.NoError(t, pkgdb.DB().Create(&rec).Error)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=records&sort=id&page=1&size=2", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)
	require.EqualValues(t, 3, data["total"])
	require.Equal(t, true, data["has_more"])
}

func TestQueryHandlerSimplifiedQueryClampsOversizedPageSizeToDefault(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedRecordsTable",
	}).Error)
	for i := 1; i <= 25; i++ {
		rec := models.Record{
			ID:        fmt.Sprintf("rec_%02d", i),
			TableID:   "tbl_allowed",
			Data:      `{"status":"ok"}`,
			CreatedBy: user.ID,
			UpdatedBy: user.ID,
			Version:   1,
		}
		require.NoError(t, pkgdb.DB().Create(&rec).Error)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=records&sort=id&page=1&size=5000", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 20)
	require.EqualValues(t, 20, data["size"])
	require.Equal(t, true, data["has_more"])
}

func TestQueryHandlerBatchQueryFailsWhenOneQueryUnauthorized(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_view",
		Name:       "AllowedTable",
	}).Error)

	reqBody := []byte(`{
		"queries":{
			"safe":{"from":"tables","select":["id","name"],"page":1,"size":20},
			"forbidden":{"from":"database_access","select":["id","database_id","role"],"page":1,"size":20}
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/batch", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "管理员权限")
}

func TestQueryHandlerSimplifiedQueryRecordsCannotBypassAutoTableFilter(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedRecordsTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_blocked",
		DatabaseID: "db_blocked",
		Name:       "BlockedRecordsTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_allowed",
		TableID:   "tbl_allowed",
		Data:      `{"status":"allowed"}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_blocked",
		TableID:   "tbl_blocked",
		Data:      `{"status":"blocked"}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)

	filter := url.QueryEscape(`{"table_id":{"in":["tbl_allowed","tbl_blocked"]}}`)
	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=records&filter="+filter+"&sort=created_at&page=1&size=20", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)

	firstRow := rows[0].(map[string]interface{})
	require.Equal(t, "rec_allowed", firstRow["id"])
	require.Equal(t, "tbl_allowed", firstRow["table_id"])
	require.EqualValues(t, 1, data["total"])
}

func TestQueryHandlerSimplifiedQueryRecordsSupportsJSONFilterAndSort(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedRecordsTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_blocked",
		DatabaseID: "db_blocked",
		Name:       "BlockedRecordsTable",
	}).Error)

	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_low",
		TableID:   "tbl_allowed",
		Data:      `{"status":"approved","amount":10}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_mid",
		TableID:   "tbl_allowed",
		Data:      `{"status":"approved","amount":20}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_other_status",
		TableID:   "tbl_allowed",
		Data:      `{"status":"draft","amount":99}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_blocked",
		TableID:   "tbl_blocked",
		Data:      `{"status":"approved","amount":999}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)

	filter := url.QueryEscape(`{"data.status":"approved"}`)
	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=records&filter="+filter+"&sort=-data.amount&page=1&size=20", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)

	firstRow := rows[0].(map[string]interface{})
	secondRow := rows[1].(map[string]interface{})
	require.Equal(t, "rec_mid", firstRow["id"])
	require.Equal(t, "rec_low", secondRow["id"])
	require.Contains(t, firstRow["data"].(string), `"status":"approved"`)
	require.Contains(t, secondRow["data"].(string), `"status":"approved"`)
	require.EqualValues(t, 2, data["total"])
}

func TestQueryHandlerQueryRecordsSupportsJSONRangeFilterAndSort(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedRecordsTable",
	}).Error)

	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_05",
		TableID:   "tbl_allowed",
		Data:      `{"amount":5}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_10",
		TableID:   "tbl_allowed",
		Data:      `{"amount":10}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_30",
		TableID:   "tbl_allowed",
		Data:      `{"amount":30}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)

	q := map[string]interface{}{
		"from":   "records",
		"select": []string{"id", "table_id", "data"},
		"where": map[string]interface{}{
			"and": []map[string]interface{}{
				{"field": "data.amount", "op": "gte", "value": 10},
			},
		},
		"orderBy": []map[string]interface{}{
			{"field": "data.amount", "dir": "asc"},
		},
		"page": 1,
		"size": 20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)
	require.Equal(t, "rec_10", rows[0].(map[string]interface{})["id"])
	require.Equal(t, "rec_30", rows[1].(map[string]interface{})["id"])
	require.EqualValues(t, 2, data["total"])
}

func TestQueryHandlerQueryOrganizationsFiltersToOwnedAndMemberScopes(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_org_other")

	require.NoError(t, pkgdb.DB().Create(&models.Organization{
		ID:      "org_owned",
		Name:    "OwnedOrg",
		OwnerID: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Organization{
		ID:      "org_member",
		Name:    "MemberOrg",
		OwnerID: otherUser.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Organization{
		ID:      "org_blocked",
		Name:    "BlockedOrg",
		OwnerID: otherUser.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.OrganizationMember{
		OrganizationID: "org_member",
		UserID:         user.ID,
		Role:           "member",
	}).Error)

	q := map[string]interface{}{
		"from":   "organizations",
		"select": []string{"id", "name", "owner_id"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)

	ids := make([]string, 0, 2)
	for _, row := range rows {
		ids = append(ids, row.(map[string]interface{})["id"].(string))
	}
	require.ElementsMatch(t, []string{"org_owned", "org_member"}, ids)
}

func TestQueryHandlerQueryActivityLogsFiltersToCurrentUserAndSystemEntries(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_log_other")

	require.NoError(t, pkgdb.DB().Create(&models.ActivityLog{
		ID:           "act_me",
		UserID:       user.ID,
		Action:       "query",
		ResourceType: "records",
		ResourceID:   "rec_1",
		Description:  "self log",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.ActivityLog{
		ID:           "act_system",
		UserID:       "system:mcp",
		Action:       "sync",
		ResourceType: "records",
		ResourceID:   "rec_2",
		Description:  "system log",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.ActivityLog{
		ID:           "act_other",
		UserID:       otherUser.ID,
		Action:       "delete",
		ResourceType: "records",
		ResourceID:   "rec_3",
		Description:  "other log",
	}).Error)

	q := map[string]interface{}{
		"from":   "activity_logs",
		"select": []string{"id", "user_id", "action"},
		"orderBy": []map[string]interface{}{
			{"field": "id", "dir": "asc"},
		},
		"page": 1,
		"size": 20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)

	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.(map[string]interface{})["id"].(string))
	}
	require.ElementsMatch(t, []string{"act_me", "act_system"}, ids)
}

func TestQueryHandlerQueryFilesFiltersToAccessibleRecords(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_blocked",
		DatabaseID: "db_blocked",
		Name:       "BlockedTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_allowed",
		TableID:   "tbl_allowed",
		Data:      `{"status":"ok"}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Record{
		ID:        "rec_blocked",
		TableID:   "tbl_blocked",
		Data:      `{"status":"ok"}`,
		CreatedBy: user.ID,
		UpdatedBy: user.ID,
		Version:   1,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.File{
		ID:         "fil_allowed",
		RecordID:   "rec_allowed",
		FileName:   "allowed.txt",
		FileSize:   10,
		FileType:   "text/plain",
		UploadedBy: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.File{
		ID:         "fil_blocked",
		RecordID:   "rec_blocked",
		FileName:   "blocked.txt",
		FileSize:   10,
		FileType:   "text/plain",
		UploadedBy: user.ID,
	}).Error)

	q := map[string]interface{}{
		"from":   "files",
		"select": []string{"id", "record_id", "file_name"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)
	require.Equal(t, "fil_allowed", rows[0].(map[string]interface{})["id"])
	require.EqualValues(t, 1, data["total"])
}

func TestQueryHandlerQueryFilesWithoutAccessibleRecordsReturnsEmptySet(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)

	q := map[string]interface{}{
		"from":   "files",
		"select": []string{"id", "record_id", "file_name"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 0)
	require.EqualValues(t, 0, data["total"])
	require.Equal(t, false, data["has_more"])
}

func TestQueryHandlerQueryPluginsFiltersToCreator(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_plugin_other")

	require.NoError(t, pkgdb.DB().Create(&models.Plugin{
		ID:          "plg_self",
		Name:        "OwnedPlugin",
		Description: "owned",
		Language:    "go",
		EntryFile:   "main.go",
		CreatedBy:   user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Plugin{
		ID:          "plg_other",
		Name:        "OtherPlugin",
		Description: "other",
		Language:    "python",
		EntryFile:   "main.py",
		CreatedBy:   otherUser.ID,
	}).Error)

	q := map[string]interface{}{
		"from":   "plugins",
		"select": []string{"id", "name", "created_by"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)
	require.Equal(t, "plg_self", rows[0].(map[string]interface{})["id"])
	require.Equal(t, user.ID, rows[0].(map[string]interface{})["created_by"])
}

func TestQueryHandlerQueryPluginBindingsFiltersToOwnedPlugins(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_binding_other")

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Plugin{
		ID:        "plg_owned",
		Name:      "OwnedPlugin",
		Language:  "bash",
		EntryFile: "main.sh",
		CreatedBy: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Plugin{
		ID:        "plg_other",
		Name:      "OtherPlugin",
		Language:  "bash",
		EntryFile: "main.sh",
		CreatedBy: otherUser.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.PluginBinding{
		ID:       "pbd_owned",
		PluginID: "plg_owned",
		TableID:  "tbl_allowed",
		Trigger:  "manual",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.PluginBinding{
		ID:       "pbd_other",
		PluginID: "plg_other",
		TableID:  "tbl_allowed",
		Trigger:  "manual",
	}).Error)

	q := map[string]interface{}{
		"from":   "plugin_bindings",
		"select": []string{"id", "plugin_id", "table_id"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)
	require.Equal(t, "pbd_owned", rows[0].(map[string]interface{})["id"])
	require.EqualValues(t, 1, data["total"])
}

func TestQueryHandlerQueryPluginExecutionsFiltersToOwnedPlugins(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_execution_other")

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_allowed",
		DatabaseID: "db_allowed",
		Name:       "AllowedTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Plugin{
		ID:        "plg_owned",
		Name:      "OwnedPlugin",
		Language:  "bash",
		EntryFile: "main.sh",
		CreatedBy: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Plugin{
		ID:        "plg_other",
		Name:      "OtherPlugin",
		Language:  "bash",
		EntryFile: "main.sh",
		CreatedBy: otherUser.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.PluginExecution{
		ID:         "pex_owned",
		PluginID:   "plg_owned",
		TableID:    "tbl_allowed",
		Trigger:    "manual",
		Status:     "success",
		CreatedBy:  user.ID,
		DurationMS: 10,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.PluginExecution{
		ID:         "pex_other",
		PluginID:   "plg_other",
		TableID:    "tbl_allowed",
		Trigger:    "manual",
		Status:     "success",
		CreatedBy:  otherUser.ID,
		DurationMS: 12,
	}).Error)

	q := map[string]interface{}{
		"from":   "plugin_executions",
		"select": []string{"id", "plugin_id", "table_id", "status"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 1)
	require.Equal(t, "pex_owned", rows[0].(map[string]interface{})["id"])
	require.EqualValues(t, 1, data["total"])
}

func TestQueryHandlerQueryOrganizationMembersFiltersByAccessibleOrganizations(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_member_other")
	thirdUser := createTestUser(t, "query_member_third")

	require.NoError(t, pkgdb.DB().Create(&models.Organization{
		ID:      "org_owned",
		Name:    "OwnedOrg",
		OwnerID: user.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Organization{
		ID:      "org_member",
		Name:    "MemberOrg",
		OwnerID: otherUser.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Organization{
		ID:      "org_blocked",
		Name:    "BlockedOrg",
		OwnerID: otherUser.ID,
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.OrganizationMember{
		OrganizationID: "org_member",
		UserID:         user.ID,
		Role:           "member",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.OrganizationMember{
		OrganizationID: "org_owned",
		UserID:         thirdUser.ID,
		Role:           "member",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.OrganizationMember{
		OrganizationID: "org_member",
		UserID:         thirdUser.ID,
		Role:           "member",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.OrganizationMember{
		OrganizationID: "org_blocked",
		UserID:         thirdUser.ID,
		Role:           "member",
	}).Error)

	q := map[string]interface{}{
		"from":   "organization_members",
		"select": []string{"organization_id", "user_id", "role"},
		"page":   1,
		"size":   20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 3)

	orgIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		orgIDs = append(orgIDs, row.(map[string]interface{})["organization_id"].(string))
	}
	require.ElementsMatch(t, []string{"org_owned", "org_member", "org_member"}, orgIDs)
}

func TestQueryHandlerExplainJoinAliasAndNestedWherePreservesPermissionScoping(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_allowed",
		Role:       "viewer",
	}).Error)

	reqBody := []byte(`{
		"from":"tables",
		"select":["tables.id","db.name"],
		"join":[
			{
				"type":"left",
				"table":"databases",
				"as":"db",
				"on":"db.id = tables.database_id"
			}
		],
		"where":{
			"and":[
				{"field":"db.name","op":"eq","value":"MainDB"},
				{"or":[
					{"field":"tables.name","op":"like","value":"Orders"},
					{"field":"db.owner_id","op":"eq","value":"usr_owner"}
				]}
			]
		},
		"orderBy":[{"field":"db.name","dir":"asc"}],
		"page":1,
		"size":20
	}`)

	req := httptest.NewRequest(http.MethodPost, "/api/query/explain", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	sqlText := data["sql"].(string)
	params := data["params"].([]interface{})

	require.Contains(t, sqlText, `JOIN "databases" AS "db" ON db.id = tables.database_id`)
	require.Contains(t, sqlText, `"tables"."database_id"`)
	require.Contains(t, sqlText, `"db"."name"`)
	require.Contains(t, sqlText, `"tables"."name"`)
	require.Contains(t, sqlText, `"db"."owner_id"`)
	require.Contains(t, params, "db_allowed")
	require.Contains(t, params, "MainDB")
	require.Contains(t, params, "%Orders%")
	require.Contains(t, params, "usr_owner")
}

func TestQueryHandlerQueryDatabaseAccessOwnerAndAdminScopesOnly(t *testing.T) {
	router, user := setupQueryHandlerTest(t)
	otherUser := createTestUser(t, "query_db_access_other")

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_owner",
		Role:       "owner",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     otherUser.ID,
		DatabaseID: "db_owner",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     otherUser.ID,
		DatabaseID: "db_admin",
		Role:       "editor",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     otherUser.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	q := map[string]interface{}{
		"from":   "database_access",
		"select": []string{"user_id", "database_id", "role"},
		"orderBy": []map[string]interface{}{
			{"field": "database_id", "dir": "asc"},
			{"field": "user_id", "dir": "asc"},
		},
		"page": 1,
		"size": 20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 4)

	dbIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		dbIDs = append(dbIDs, row.(map[string]interface{})["database_id"].(string))
	}
	require.ElementsMatch(t, []string{"db_admin", "db_admin", "db_owner", "db_owner"}, dbIDs)
	require.NotContains(t, dbIDs, "db_view")
}

func TestQueryHandlerQueryFieldPermissionsOwnerAndAdminScopesOnly(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_owner",
		Role:       "owner",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_admin",
		Role:       "admin",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_owner",
		DatabaseID: "db_owner",
		Name:       "OwnerTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_admin",
		DatabaseID: "db_admin",
		Name:       "AdminTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.Table{
		ID:         "tbl_view",
		DatabaseID: "db_view",
		Name:       "ViewTable",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.FieldPermission{
		ID:      "flp_owner",
		TableID: "tbl_owner",
		FieldID: "fld_owner",
		Role:    "viewer",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.FieldPermission{
		ID:      "flp_admin",
		TableID: "tbl_admin",
		FieldID: "fld_admin",
		Role:    "editor",
	}).Error)
	require.NoError(t, pkgdb.DB().Create(&models.FieldPermission{
		ID:      "flp_view",
		TableID: "tbl_view",
		FieldID: "fld_view",
		Role:    "viewer",
	}).Error)

	q := map[string]interface{}{
		"from":   "field_permissions",
		"select": []string{"id", "table_id", "field_id", "role"},
		"orderBy": []map[string]interface{}{
			{"field": "table_id", "dir": "asc"},
		},
		"page": 1,
		"size": 20,
	}
	qb, err := json.Marshal(q)
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/query?q="+url.QueryEscape(string(qb)), nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	data := body["data"].(map[string]interface{})
	rows := data["data"].([]interface{})
	require.Len(t, rows, 2)

	tableIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		tableIDs = append(tableIDs, row.(map[string]interface{})["table_id"].(string))
	}
	require.ElementsMatch(t, []string{"tbl_owner", "tbl_admin"}, tableIDs)
	require.NotContains(t, tableIDs, "tbl_view")
}

func TestQueryHandlerBatchQueryForbiddenErrorIncludesFailingQueryNameAndReason(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	reqBody := []byte(`{
		"queries":{
			"safe":{"from":"tables","select":["id","name"],"page":1,"size":20},
			"forbidden":{"from":"database_access","select":["id","database_id","role"],"page":1,"size":20}
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/batch", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	message := body["message"].(string)
	require.Contains(t, message, "查询 'forbidden' 执行失败")
	require.Contains(t, message, "权限验证失败")
	require.Contains(t, message, "管理员权限")
}

func TestQueryHandlerBatchQueryBadRequestIncludesFailingQueryName(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	require.NoError(t, pkgdb.DB().Create(&models.DatabaseAccess{
		UserID:     user.ID,
		DatabaseID: "db_view",
		Role:       "viewer",
	}).Error)

	reqBody := []byte(`{
		"queries":{
			"safe":{"from":"tables","select":["id","name"],"page":1,"size":20},
			"broken":{"select":["id"],"page":1,"size":20}
		}
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query/batch", bytes.NewReader(reqBody))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)

	var body map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	message := body["message"].(string)
	require.Contains(t, message, "查询 'broken' 执行失败")
	require.Contains(t, message, "必须指定表名")
}

func TestQueryHandlerBatchQueryRejectsMalformedJSON(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/query/batch", bytes.NewBufferString(`{"queries":`))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "请求格式错误")
}

func TestQueryHandlerPostQueryRejectsMalformedJSON(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodPost, "/api/query", bytes.NewBufferString(`{"from":`))
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "请求格式错误")
}

func TestQueryHandlerQueryRequiresQueryParameter(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/query", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "缺少查询参数")
}

func TestQueryHandlerSimplifiedQueryRejectsInvalidFilterJSON(t *testing.T) {
	router, user := setupQueryHandlerTest(t)

	req := httptest.NewRequest(http.MethodGet, "/api/query/simple?table=users&filter={bad-json}&page=1&size=20", nil)
	req.Header.Set("Authorization", authHeaderForQueryUser(t, user))

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "filter 格式错误")
}
