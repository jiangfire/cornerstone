package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupCRUDTest(t *testing.T) (*gin.Engine, *gorm.DB, *models.Token) {
	t.Helper()
	db := testutil.SetupTestDB(t)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	pkgdb.SetDB(db)

	router := gin.New()
	router.Use(middleware.Auth())

	dbSvc := router.Group("/api/v1/databases")
	dbSvc.POST("/", CreateDatabase)
	dbSvc.GET("/", ListDatabases)
	dbSvc.GET("/:id", GetDatabase)
	dbSvc.PUT("/:id", UpdateDatabase)
	dbSvc.DELETE("/:id", DeleteDatabase)
	dbSvc.POST("/with-tables", CreateDatabaseWithTables)
	dbSvc.GET("/:id/tables", ListTables)

	tblSvc := router.Group("/api/v1/tables")
	tblSvc.POST("/", CreateTable)
	tblSvc.GET("/detail/:id", GetTable)
	tblSvc.PUT("/:id", UpdateTable)
	tblSvc.DELETE("/:id", DeleteTable)
	tblSvc.GET("/:id/fields", ListFields)

	fldSvc := router.Group("/api/v1/fields")
	fldSvc.POST("/", CreateField)
	fldSvc.GET("/:id", GetField)
	fldSvc.PUT("/:id", UpdateField)
	fldSvc.DELETE("/:id", DeleteField)

	tokSvc := router.Group("/api/v1/tokens")
	tokSvc.GET("/", ListTokens)
	tokSvc.POST("/", middleware.RequireMaster(), CreateToken)
	tokSvc.PUT("/:id", middleware.RequireMaster(), UpdateToken)
	tokSvc.DELETE("/:id", DeleteToken)

	recSvc := router.Group("/api/v1/records")
	recSvc.POST("/", CreateRecord)
	recSvc.GET("/", ListRecords)
	recSvc.GET("/export", ExportRecords)
	recSvc.GET("/:id", GetRecord)
	recSvc.PUT("/:id", UpdateRecord)
	recSvc.DELETE("/:id", DeleteRecord)
	recSvc.POST("/batch", BatchCreateRecords)

	return router, db, master
}

func doJSON(t *testing.T, router *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.DoRequest(t, router, method, path, token, body)
}

func decodeResp(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	return testutil.DecodeJSONResponseRaw(t, rec)
}

func createDBDirect(t *testing.T, db *gorm.DB, name string) *models.Database {
	t.Helper()
	m := &models.Database{Name: name}
	require.NoError(t, db.Create(m).Error)
	return m
}

func createTableDirect(t *testing.T, db *gorm.DB, dbID, name string) *models.Table {
	t.Helper()
	m := &models.Table{DatabaseID: dbID, Name: name}
	require.NoError(t, db.Create(m).Error)
	return m
}

func createFieldDirect(t *testing.T, db *gorm.DB, tableID, name, fieldType string) *models.Field {
	t.Helper()
	m := &models.Field{TableID: tableID, Name: name, Type: fieldType}
	require.NoError(t, db.Create(m).Error)
	return m
}

func createRecordDirect(t *testing.T, db *gorm.DB, tableID string, data map[string]interface{}) *models.Record {
	t.Helper()
	return testutil.CreateRecordDirect(t, db, tableID, data)
}

// ── Database Handlers ──

func TestCreateDatabase_Success(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]string{"name": "testdb", "description": "a test db"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, "testdb", data["name"])
	assert.Equal(t, "a test db", data["description"])
}

func TestCreateDatabase_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "POST", "/api/v1/databases/", master.Token, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(400), resp["code"])
	assert.Contains(t, resp["message"], "invalid request")
}

func TestCreateDatabase_Unauthorized(t *testing.T) {
	router, _, _ := setupCRUDTest(t)

	body := map[string]string{"name": "testdb"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", "", body)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestListDatabases_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	createDBDirect(t, db, "db1")
	createDBDirect(t, db, "db2")

	rec := doJSON(t, router, "GET", "/api/v1/databases/", master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	databases, ok := data["databases"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(databases))
	assert.Equal(t, float64(2), data["total"])
}

func TestGetDatabase_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "mydb")

	rec := doJSON(t, router, "GET", "/api/v1/databases/"+dbModel.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestGetDatabase_NotFound(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/databases/nonexistent", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestUpdateDatabase_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "olddb")

	body := map[string]string{"name": "newdb", "description": "updated"}
	rec := doJSON(t, router, "PUT", "/api/v1/databases/"+dbModel.ID, master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "newdb", data["name"])
	assert.Equal(t, "updated", data["description"])
}

func TestUpdateDatabase_BindingError(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "mydb")

	rec := doJSON(t, router, "PUT", "/api/v1/databases/"+dbModel.ID, master.Token, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestDeleteDatabase_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "deleteme")

	rec := doJSON(t, router, "DELETE", "/api/v1/databases/"+dbModel.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data["message"], "database deleted")
}

func TestCreateDatabaseWithTables_Success(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"name":        "bulkdb",
		"description": "bulk create test",
		"tables": []map[string]interface{}{
			{
				"name": "table1",
				"fields": []map[string]interface{}{
					{"name": "title", "type": "string"},
					{"name": "count", "type": "number"},
				},
			},
			{
				"name": "table2",
				"fields": []map[string]interface{}{
					{"name": "active", "type": "boolean"},
				},
			},
		},
	}
	rec := doJSON(t, router, "POST", "/api/v1/databases/with-tables", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotNil(t, data["database"])
	assert.NotNil(t, data["tables"])
	assert.NotNil(t, data["fields"])
	summary, ok := data["summary"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(2), summary["table_count"])
	assert.Equal(t, float64(3), summary["field_count"])
}

// ── Table Handlers ──

func TestCreateTable_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")

	body := map[string]string{"database_id": dbModel.ID, "name": "items", "description": "items table"}
	rec := doJSON(t, router, "POST", "/api/v1/tables/", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, dbModel.ID, data["database_id"])
	assert.Equal(t, "items", data["name"])
}

func TestCreateTable_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "POST", "/api/v1/tables/", master.Token, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestListTables_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	createTableDirect(t, db, dbModel.ID, "table1")
	createTableDirect(t, db, dbModel.ID, "table2")

	rec := doJSON(t, router, "GET", "/api/v1/databases/"+dbModel.ID+"/tables", master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	tables, ok := data["tables"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(tables))
	assert.Equal(t, float64(2), data["total"])
}

func TestGetTable_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")

	rec := doJSON(t, router, "GET", "/api/v1/tables/detail/"+tbl.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestUpdateTable_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "oldname")

	body := map[string]string{"name": "newname", "description": "updated"}
	rec := doJSON(t, router, "PUT", "/api/v1/tables/"+tbl.ID, master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "newname", data["name"])
	assert.Equal(t, "updated", data["description"])
}

func TestDeleteTable_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "deleteme")

	rec := doJSON(t, router, "DELETE", "/api/v1/tables/"+tbl.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data["message"], "table deleted")
}

// ── Field Handlers ──

func TestCreateField_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")

	body := map[string]interface{}{
		"table_id": tbl.ID,
		"name":     "title",
		"type":     "string",
		"required": true,
	}
	rec := doJSON(t, router, "POST", "/api/v1/fields/", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, tbl.ID, data["table_id"])
	assert.Equal(t, "title", data["name"])
	assert.Equal(t, "string", data["type"])
}

func TestCreateField_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "POST", "/api/v1/fields/", master.Token, map[string]string{})

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestListFields_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")
	createFieldDirect(t, db, tbl.ID, "title", "string")
	createFieldDirect(t, db, tbl.ID, "count", "number")

	rec := doJSON(t, router, "GET", "/api/v1/tables/"+tbl.ID+"/fields", master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	items, ok := data["items"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(items))
	assert.Equal(t, float64(2), data["total"])
}

func TestUpdateField_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")
	fld := createFieldDirect(t, db, tbl.ID, "title", "string")

	body := map[string]interface{}{
		"name":        "renamed_title",
		"type":        "text",
		"description": "updated desc",
		"required":    true,
	}
	rec := doJSON(t, router, "PUT", "/api/v1/fields/"+fld.ID, master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "renamed_title", data["name"])
	assert.Equal(t, "text", data["type"])
}

func TestDeleteField_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")
	fld := createFieldDirect(t, db, tbl.ID, "title", "string")

	rec := doJSON(t, router, "DELETE", "/api/v1/fields/"+fld.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data["message"], "field deleted")
}

// ── Token Handlers ──

func TestListTokens_MasterSeesAll(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	client := &models.Token{Name: "client1", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(client).Error)

	rec := doJSON(t, router, "GET", "/api/v1/tokens/", master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	tokens, ok := data["tokens"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(tokens), 1)
	assert.GreaterOrEqual(t, int(data["total"].(float64)), 1)
}

func TestCreateToken_Success(t *testing.T) {
	router, db, _ := setupCRUDTest(t)

	client := &models.Token{Name: "deleteme", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(client).Error)

	body := map[string]string{"name": "newclient", "scopes": "read,write"}
	rec := doJSON(t, router, "POST", "/api/v1/tokens/", client.Token, body)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestCreateToken_MasterSuccess(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	body := map[string]string{"name": "newclient", "scopes": "read,write"}
	rec := doJSON(t, router, "POST", "/api/v1/tokens/", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, "newclient", data["name"])
	assert.NotEmpty(t, data["token"])
	assert.True(t, strings.HasPrefix(data["token"].(string), "cs_"))
}

func TestUpdateToken_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	client := &models.Token{Name: "updateme", IsMaster: false, Scopes: "read"}
	require.NoError(t, db.Create(client).Error)

	body := map[string]string{"scopes": "read,write"}
	rec := doJSON(t, router, "PUT", "/api/v1/tokens/"+client.ID, master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "read,write", data["scopes"])
}

func TestDeleteToken_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	client := &models.Token{Name: "deleteme", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(client).Error)

	rec := doJSON(t, router, "DELETE", "/api/v1/tokens/"+client.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, client.ID, data["id"])
}

// ── Record Handlers ──

func setupRecordPrereqs(t *testing.T, db *gorm.DB) (*models.Database, *models.Table, *models.Field) {
	t.Helper()
	dbModel := createDBDirect(t, db, "recdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")
	fld := createFieldDirect(t, db, tbl.ID, "title", "string")
	return dbModel, tbl, fld
}

func TestCreateRecord_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	body := map[string]interface{}{
		"table_id": tbl.ID,
		"data":     map[string]interface{}{"title": "hello"},
	}
	rec := doJSON(t, router, "POST", "/api/v1/records/", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, tbl.ID, data["table_id"])
}

func TestListRecords_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "r1"})
	createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "r2"})

	path := fmt.Sprintf("/api/v1/records/?table_id=%s&limit=20", tbl.ID)
	rec := doJSON(t, router, "GET", path, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	items, ok := data["records"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(items))
}

func TestGetRecord_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	record := createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "hello"})

	rec := doJSON(t, router, "GET", "/api/v1/records/"+record.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, record.ID, data["id"])
}

func TestUpdateRecord_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	record := createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "old"})

	body := map[string]interface{}{
		"data":    map[string]interface{}{"title": "new"},
		"version": 1,
	}
	rec := doJSON(t, router, "PUT", "/api/v1/records/"+record.ID, master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, record.ID, data["id"])
	assert.Equal(t, float64(2), data["version"])
}

func TestDeleteRecord_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	record := createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "bye"})

	rec := doJSON(t, router, "DELETE", "/api/v1/records/"+record.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, data["message"], "record deleted")
}

func TestBatchCreateRecords_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	body := map[string]interface{}{
		"table_id": tbl.ID,
		"data":     map[string]interface{}{"title": "batch"},
	}
	path := fmt.Sprintf("/api/v1/records/batch?count=%d", 5)
	rec := doJSON(t, router, "POST", path, master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(5), data["count"])
	records, ok := data["records"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 5, len(records))
}

func TestExportRecords_CSV(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "row1"})

	path := fmt.Sprintf("/api/v1/records/export?table_id=%s&format=csv", tbl.ID)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	body := w.Body.String()
	assert.Contains(t, body, "title")
	assert.Contains(t, body, "row1")
}

func TestCreateRecord_MissingTableID(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"data": map[string]interface{}{"title": "hello"},
	}
	rec := doJSON(t, router, "POST", "/api/v1/records/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestListRecords_MissingTableID(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/records/", master.Token, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestGetField_Success(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	dbModel := createDBDirect(t, db, "testdb")
	tbl := createTableDirect(t, db, dbModel.ID, "items")
	fld := createFieldDirect(t, db, tbl.ID, "title", "string")

	rec := doJSON(t, router, "GET", "/api/v1/fields/"+fld.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestExportRecords_JSON(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "row1"})

	path := fmt.Sprintf("/api/v1/records/export?table_id=%s&format=json", tbl.ID)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")

	var exported []map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &exported))
	assert.GreaterOrEqual(t, len(exported), 1)
}

func TestCreateDatabase_DuplicateName(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	createDBDirect(t, db, "uniquename")

	body := map[string]string{"name": "uniquename", "description": "dup"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDeleteToken_SelfDelete(t *testing.T) {
	router, db, _ := setupCRUDTest(t)

	client := &models.Token{Name: "selfdelete", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(client).Error)

	rec := doJSON(t, router, "DELETE", "/api/v1/tokens/"+client.ID, client.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}
