package testutil

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/pkg/db"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func SetupGinEngine(dbConn *gorm.DB) *gin.Engine {
	engine := gin.New()
	engine.Use(middleware.RequestID())
	return engine
}

type APIResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Data    map[string]interface{} `json:"data,omitempty"`
}

func DecodeJSONResponse(t *testing.T, resp *httptest.ResponseRecorder) APIResponse {
	t.Helper()
	var result APIResponse
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	return result
}

func DecodeJSONResponseRaw(t *testing.T, resp *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var result map[string]interface{}
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	return result
}

func DoRequest(t *testing.T, router *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		require.NoError(t, err)
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, path, bodyReader)
	require.NoError(t, err)

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

type TestEnv struct {
	DB       *gorm.DB
	Router   *gin.Engine
	Master   *models.Token
	Viewer   *models.Token
	Editor   *models.Token
	Database *models.Database
	Table    *models.Table
	Fields   []*models.Field
}

func SetupTestEnv(t *testing.T) *TestEnv {
	t.Helper()

	database := SetupTestDB(t)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, database.Create(master).Error)

	viewer := &models.Token{
		Name:     "viewer",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, database.Create(viewer).Error)

	editor := &models.Token{
		Name:     "editor",
		IsMaster: false,
		Scopes:   `{"databases":{},"tables":{}}`,
	}
	require.NoError(t, database.Create(editor).Error)

	dbModel := &models.Database{Name: "testdb"}
	require.NoError(t, database.Create(dbModel).Error)

	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, database.Create(tbl).Error)

	fields := []*models.Field{
		{TableID: tbl.ID, Name: "title", Type: "string", Required: true},
		{TableID: tbl.ID, Name: "count", Type: "number"},
		{TableID: tbl.ID, Name: "active", Type: "boolean"},
	}
	for _, f := range fields {
		require.NoError(t, database.Create(f).Error)
	}

	db.SetDB(database)

	env := &TestEnv{
		DB:       database,
		Master:   master,
		Viewer:   viewer,
		Editor:   editor,
		Database: dbModel,
		Table:    tbl,
		Fields:   fields,
	}

	env.Router = SetupGinEngine(database)

	return env
}

func CreateRecordDirect(t *testing.T, database *gorm.DB, tableID string, data map[string]interface{}) *models.Record {
	t.Helper()
	dataJSON, err := json.Marshal(data)
	require.NoError(t, err)
	record := &models.Record{TableID: tableID, Data: string(dataJSON), Version: 1}
	require.NoError(t, database.Create(record).Error)
	return record
}
