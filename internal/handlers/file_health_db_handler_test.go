package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
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

func setupDBHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB, *models.Token) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)
	pkgdb.SetDB(db)

	router := gin.New()
	router.Use(middleware.Auth())

	dbSvc := router.Group("/api/v1/databases")
	dbSvc.POST("/", CreateDatabase)
	dbSvc.GET("/:id", GetDatabase)

	return router, db, master
}

func setupFileTest(t *testing.T) (*gin.Engine, *gorm.DB, *models.Token, *models.Record) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)
	pkgdb.SetDB(db)

	dbModel := &models.Database{Name: "filetestdb_" + models.GenerateID("db")}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "filetest_table"}
	require.NoError(t, db.Create(tbl).Error)
	record := &models.Record{TableID: tbl.ID, Data: "{}", Version: 1}
	require.NoError(t, db.Create(record).Error)

	router := gin.New()
	router.Use(middleware.Auth())

	router.POST("/api/v1/files/upload", UploadFile)
	router.GET("/api/v1/files/:id", GetFile)
	router.GET("/api/v1/files/:id/download", DownloadFile)
	router.DELETE("/api/v1/files/:id", DeleteFile)
	router.GET("/api/v1/records/:id/files", ListRecordFiles)

	t.Cleanup(func() { os.RemoveAll("./uploads") })

	return router, db, master, record
}

func doFileUpload(t *testing.T, router *gin.Engine, token, recordID, fileName string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", fileName)
	require.NoError(t, err)
	_, err = part.Write(content)
	require.NoError(t, err)
	if recordID != "" {
		writer.WriteField("record_id", recordID)
	}
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func uploadTestFile(t *testing.T, router *gin.Engine, token, recordID string) string {
	t.Helper()
	w := doFileUpload(t, router, token, recordID, "test.txt", []byte("hello world"))
	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	fileID, ok := data["id"].(string)
	require.True(t, ok)
	return fileID
}

// ── Health Tests ──

func TestHealth_Returns200(t *testing.T) {
	SetVersion("test-version")
	router := gin.New()
	router.GET("/health", Health)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "healthy", resp["status"])
	assert.Equal(t, "cornerstone-backend", resp["service"])
}

func TestSetVersion_StoresVersion(t *testing.T) {
	SetVersion("2.0.0")
	router := gin.New()
	router.GET("/health", Health)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "2.0.0", resp["version"])
}

func TestHealth_JSONContainsTime(t *testing.T) {
	router := gin.New()
	router.GET("/health", Health)

	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/health", nil)
	router.ServeHTTP(w, req)

	assert.Equal(t, 200, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "healthy", resp["status"])
	_, hasTime := resp["time"]
	assert.True(t, hasTime)
}

// ── Database Handler Tests ──

func TestDBHandler_CreateDatabase_Success(t *testing.T) {
	router, _, master := setupDBHandlerTest(t)

	body := map[string]string{"name": "handler_test_db", "description": "handler test"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, "handler_test_db", data["name"])
	assert.Equal(t, "handler test", data["description"])
}

func TestDBHandler_CreateDatabase_DuplicateName(t *testing.T) {
	router, gormDB, master := setupDBHandlerTest(t)

	createDBDirect(t, gormDB, "dup_db_name")

	body := map[string]string{"name": "dup_db_name", "description": "dup"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDBHandler_CreateDatabase_InvalidJSON(t *testing.T) {
	router, _, master := setupDBHandlerTest(t)

	req := httptest.NewRequest("POST", "/api/v1/databases/", bytes.NewBufferString(`{invalid`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+master.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestDBHandler_CreateDatabase_Unauthorized(t *testing.T) {
	router, _, _ := setupDBHandlerTest(t)

	body := map[string]string{"name": "testdb"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", "", body)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestDBHandler_GetDatabase_Success(t *testing.T) {
	router, gormDB, master := setupDBHandlerTest(t)

	dbModel := createDBDirect(t, gormDB, "gettest_db")

	rec := doJSON(t, router, "GET", "/api/v1/databases/"+dbModel.ID, master.Token, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestDBHandler_GetDatabase_NotFound(t *testing.T) {
	router, _, master := setupDBHandlerTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/databases/db_nonexistent", master.Token, nil)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDBHandler_GetDatabase_Unauthorized(t *testing.T) {
	router, _, _ := setupDBHandlerTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/databases/db_someid", "", nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ── File Handler Tests ──

func TestFile_Upload_Success(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	w := doFileUpload(t, router, master.Token, record.ID, "test.txt", []byte("hello world"))

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["id"])
	assert.Equal(t, "test.txt", data["file_name"])
	assert.Equal(t, record.ID, data["record_id"])
}

func TestFile_Upload_Unauthorized(t *testing.T) {
	router, _, _, record := setupFileTest(t)

	w := doFileUpload(t, router, "", record.ID, "test.txt", []byte("hello"))

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestFile_Upload_MissingFile(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	writer.WriteField("record_id", record.ID)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest("POST", "/api/v1/files/upload", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+master.Token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "请选择要上传的文件")
}

func TestFile_Upload_MissingRecordAndFieldID(t *testing.T) {
	router, _, master, _ := setupFileTest(t)

	w := doFileUpload(t, router, master.Token, "", "test.txt", []byte("hello"))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "记录ID或字段ID不能为空")
}

func TestFile_GetFile_Success(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	fileID := uploadTestFile(t, router, master.Token, record.ID)

	req := httptest.NewRequest("GET", "/api/v1/files/"+fileID, nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, fileID, data["id"])
	assert.Equal(t, "test.txt", data["file_name"])
}

func TestFile_DownloadFile_Success(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	fileID := uploadTestFile(t, router, master.Token, record.ID)

	req := httptest.NewRequest("GET", "/api/v1/files/"+fileID+"/download", nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "hello world", w.Body.String())
}

func TestFile_DeleteFile_Success(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	fileID := uploadTestFile(t, router, master.Token, record.ID)

	req := httptest.NewRequest("DELETE", "/api/v1/files/"+fileID, nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, float64(0), resp["code"])

	req2 := httptest.NewRequest("GET", "/api/v1/files/"+fileID, nil)
	req2.Header.Set("Authorization", "Bearer "+master.Token)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusForbidden, w2.Code)
}

func TestFile_ListRecordFiles_Success(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	uploadTestFile(t, router, master.Token, record.ID)
	uploadTestFile(t, router, master.Token, record.ID)

	req := httptest.NewRequest("GET", "/api/v1/records/"+record.ID+"/files", nil)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	items, ok := data["items"].([]interface{})
	require.True(t, ok)
	assert.Equal(t, 2, len(items))
}

func TestFile_Upload_OversizedFile(t *testing.T) {
	router, _, master, record := setupFileTest(t)

	bigContent := bytes.Repeat([]byte("x"), 51*1024*1024+1)
	w := doFileUpload(t, router, master.Token, record.ID, "big.txt", bigContent)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "文件大小超过限制")
}
