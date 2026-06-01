package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
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

func setupQueryTest(t *testing.T) (*gin.Engine, *gorm.DB, *models.Token) {
	t.Helper()
	db := testutil.SetupTestDB(t)

	master := &models.Token{Name: "master", IsMaster: true, Scopes: "{}"}
	require.NoError(t, db.Create(master).Error)

	pkgdb.SetDB(db)

	router := gin.New()
	router.Use(middleware.Auth())

	qh := NewQueryHandler()
	router.POST("/api/v1/query", qh.Query)
	router.GET("/api/v1/query", qh.Query)
	router.POST("/api/v1/query/explain", qh.QueryExplain)
	router.POST("/api/v1/query/validate", qh.QueryValidate)
	router.POST("/api/v1/query/batch", qh.BatchQuery)
	router.GET("/api/v1/query/tables", qh.ListTables)
	router.GET("/api/v1/query/schema/:table", qh.GetTableSchema)
	router.GET("/api/v1/query/simple", qh.SimplifiedQuery)

	return router, db, master
}

func createQueryData(t *testing.T, db *gorm.DB) (*models.Database, *models.Table) {
	t.Helper()
	dbModel := &models.Database{Name: "querydb"}
	require.NoError(t, db.Create(dbModel).Error)
	tbl := &models.Table{DatabaseID: dbModel.ID, Name: "items"}
	require.NoError(t, db.Create(tbl).Error)
	return dbModel, tbl
}

func doQueryRequest(t *testing.T, router *gin.Engine, method, path, token string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	return testutil.DoRequest(t, router, method, path, token, body)
}

func decodeQueryResp(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	return testutil.DecodeJSONResponseRaw(t, rec)
}

func makeGetWithQuery(t *testing.T, router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func makePostNoBody(t *testing.T, router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("POST", path, nil)
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestQuery_Post_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	body := map[string]interface{}{
		"from": "databases",
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	rows, ok := data["data"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(rows), 1)
}

func TestQuery_Post_InvalidJSON(t *testing.T) {
	router, _, master := setupQueryTest(t)

	req, err := http.NewRequest("POST", "/api/v1/query", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQuery_Post_MissingFromAndTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	body := map[string]interface{}{
		"select": []string{"id"},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "缺少查询参数")
}

func TestQuery_Post_DisallowedTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	body := map[string]interface{}{
		"from": "secret_table",
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", master.Token, body)

	assert.True(t, rec.Code == http.StatusBadRequest || rec.Code == http.StatusForbidden)
	resp := decodeQueryResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestQuery_Post_WithWhere(t *testing.T) {
	router, db, master := setupQueryTest(t)
	dbModel, _ := createQueryData(t, db)

	body := map[string]interface{}{
		"from": "databases",
		"where": map[string]interface{}{
			"and": []map[string]interface{}{
				{"field": "name", "op": "eq", "value": dbModel.Name},
			},
		},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	rows, ok := data["data"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(rows), 1)
}

func TestQuery_Post_WithSelect(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	body := map[string]interface{}{
		"from":   "databases",
		"select": []string{"id", "name"},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	rows, ok := data["data"].([]interface{})
	require.True(t, ok)
	if len(rows) > 0 {
		row, ok := rows[0].(map[string]interface{})
		require.True(t, ok)
		assert.Contains(t, row, "id")
		assert.Contains(t, row, "name")
	}
}

func TestQuery_Get_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	q := url.QueryEscape(`{"from":"databases"}`)
	path := fmt.Sprintf("/api/v1/query?q=%s", q)
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestQuery_Get_MissingQ(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query", master.Token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "缺少查询参数")
}

func TestQuery_Get_InvalidQ(t *testing.T) {
	router, _, master := setupQueryTest(t)

	path := fmt.Sprintf("/api/v1/query?q=%s", url.QueryEscape("not-json"))
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "查询格式错误")
}

func TestQueryExplain_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	body := map[string]interface{}{
		"from":   "databases",
		"select": []string{"id", "name"},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/explain", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.NotEmpty(t, data["sql"])
}

func TestQueryExplain_InvalidRequest(t *testing.T) {
	router, _, master := setupQueryTest(t)

	req, err := http.NewRequest("POST", "/api/v1/query/explain", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestQueryExplain_QParamFallback(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	q := url.QueryEscape(`{"from":"databases"}`)
	path := fmt.Sprintf("/api/v1/query/explain?q=%s", q)
	req, err := http.NewRequest("POST", path, bytes.NewBufferString(""))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		resp := decodeQueryResp(t, w)
		assert.Equal(t, float64(0), resp["code"])
		data, ok := resp["data"].(map[string]interface{})
		require.True(t, ok)
		assert.NotEmpty(t, data["sql"])
	} else {
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestQueryValidate_Valid(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	body := map[string]interface{}{
		"from": "databases",
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/validate", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	assert.Contains(t, resp["message"], "查询验证通过")
}

func TestQueryValidate_DisallowedTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	body := map[string]interface{}{
		"from": "nonexistent_table",
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/validate", master.Token, body)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestQueryValidate_QParamFallback(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	q := url.QueryEscape(`{"from":"databases"}`)
	path := fmt.Sprintf("/api/v1/query/validate?q=%s", q)
	req, err := http.NewRequest("POST", path, bytes.NewBufferString(""))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code == http.StatusOK {
		resp := decodeQueryResp(t, w)
		assert.Equal(t, float64(0), resp["code"])
	} else {
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}
}

func TestBatchQuery_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	body := map[string]interface{}{
		"queries": map[string]interface{}{
			"all_dbs": map[string]interface{}{
				"from": "databases",
			},
			"all_tables": map[string]interface{}{
				"from": "tables",
			},
		},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/batch", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	results, ok := data["results"].(map[string]interface{})
	require.True(t, ok)
	assert.Contains(t, results, "all_dbs")
	assert.Contains(t, results, "all_tables")
}

func TestBatchQuery_InvalidBody(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makePostNoBody(t, router, "/api/v1/query/batch", master.Token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "请求格式错误")
}

func TestQueryListTables_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	rec := makeGetWithQuery(t, router, "/api/v1/query/tables", master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	tables, ok := data["tables"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(tables), 1)
}

func TestGetTableSchema_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	rec := makeGetWithQuery(t, router, "/api/v1/query/schema/databases", master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "databases", data["table"])
	fields, ok := data["fields"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(fields), 1)
}

func TestGetTableSchema_InvalidTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query/schema/invalid_table", master.Token)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestSimplifiedQuery_Success(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestSimplifiedQuery_MissingTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query/simple", master.Token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "必须指定表名")
}

func TestSimplifiedQuery_WithFilter(t *testing.T) {
	router, db, master := setupQueryTest(t)
	dbModel, _ := createQueryData(t, db)

	filter := url.QueryEscape(fmt.Sprintf(`{"name":"%s"}`, dbModel.Name))
	path := fmt.Sprintf("/api/v1/query/simple?table=databases&filter=%s", filter)
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestSimplifiedQuery_CustomPageAndSize(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&page=1&size=5"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(1), data["page"])
	assert.Equal(t, float64(5), data["size"])
}

func TestQuery_NoToken(t *testing.T) {
	router, _, _ := setupQueryTest(t)

	body := map[string]interface{}{"from": "databases"}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", "", body)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestIsPermissionError_Nil(t *testing.T) {
	assert.False(t, isPermissionError(nil))
}

func TestIsPermissionError_Denied(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("access denied")))
}

func TestIsPermissionError_Forbidden(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("forbidden action")))
}

func TestIsPermissionError_Unauthorized(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("unauthorized access")))
}

func TestIsPermissionError_PermissionCN(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("权限不足")))
}

func TestIsPermissionError_NoAccess(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("无权访问")))
}

func TestIsPermissionError_Rejected(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("拒绝访问")))
}

func TestIsPermissionError_NotAllowed(t *testing.T) {
	assert.True(t, isPermissionError(fmt.Errorf("不允许操作")))
}

func TestIsPermissionError_NotPermission(t *testing.T) {
	assert.False(t, isPermissionError(fmt.Errorf("syntax error")))
}

func TestContainsString_Match(t *testing.T) {
	assert.True(t, containsString("hello world", "world"))
}

func TestContainsString_ExactMatch(t *testing.T) {
	assert.True(t, containsString("hello", "hello"))
}

func TestContainsString_NoMatch(t *testing.T) {
	assert.False(t, containsString("hello", "xyz"))
}

func TestContainsString_EmptySubstr(t *testing.T) {
	assert.True(t, containsString("hello", ""))
}

func TestContainsSubstring_Match(t *testing.T) {
	assert.True(t, containsSubstring("abcdef", "cde"))
}

func TestContainsSubstring_NoMatch(t *testing.T) {
	assert.False(t, containsSubstring("abcdef", "xyz"))
}

func TestParseInt_Valid(t *testing.T) {
	n, err := parseInt("42")
	assert.NoError(t, err)
	assert.Equal(t, 42, n)
}

func TestParseInt_Invalid(t *testing.T) {
	_, err := parseInt("abc")
	assert.Error(t, err)
}

func TestParseInt_Zero(t *testing.T) {
	n, err := parseInt("0")
	assert.NoError(t, err)
	assert.Equal(t, 0, n)
}

func TestQuery_Post_TableShorthand(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	body := map[string]interface{}{
		"table": "databases",
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query", master.Token, body)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestQueryExplain_DisallowedTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	body := map[string]interface{}{
		"from": "secret_table",
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/explain", master.Token, body)

	assert.True(t, rec.Code == http.StatusBadRequest || rec.Code == http.StatusForbidden)
}

func TestBatchQuery_DisallowedTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	body := map[string]interface{}{
		"queries": map[string]interface{}{
			"bad": map[string]interface{}{
				"from": "secret_table",
			},
		},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/batch", master.Token, body)

	assert.True(t, rec.Code == http.StatusBadRequest || rec.Code == http.StatusForbidden)
}

func TestListTables_NoToken(t *testing.T) {
	router, _, _ := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query/tables", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetTableSchema_NoToken(t *testing.T) {
	router, _, _ := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query/schema/databases", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestSimplifiedQuery_DisallowedTable(t *testing.T) {
	router, _, master := setupQueryTest(t)

	path := "/api/v1/query/simple?table=secret_table"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.True(t, rec.Code == http.StatusBadRequest || rec.Code == http.StatusForbidden)
}

func TestSimplifiedQuery_InvalidFilter(t *testing.T) {
	router, _, master := setupQueryTest(t)

	path := fmt.Sprintf("/api/v1/query/simple?table=databases&filter=%s", url.QueryEscape("not-json"))
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "filter")
}

func TestBatchQuery_EmptyQueries(t *testing.T) {
	router, _, master := setupQueryTest(t)

	body := map[string]interface{}{
		"queries": map[string]interface{}{},
	}
	rec := doQueryRequest(t, router, "POST", "/api/v1/query/batch", master.Token, body)

	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestQuery_Post_InvalidJSONBody(t *testing.T) {
	router, _, master := setupQueryTest(t)

	req, err := http.NewRequest("POST", "/api/v1/query", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var resp map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Contains(t, resp["message"], "缺少查询参数")
}

func TestQueryValidate_MissingBody(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makePostNoBody(t, router, "/api/v1/query/validate", master.Token)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestSimplifiedQuery_NoToken(t *testing.T) {
	router, _, _ := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query/simple?table=databases", "")

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestGetTableSchema_TokensRequiresMaster(t *testing.T) {
	router, db, _ := setupQueryTest(t)

	client := &models.Token{Name: "client", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(client).Error)

	rec := makeGetWithQuery(t, router, "/api/v1/query/schema/tokens", client.Token)

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestGetTableSchema_Tokens_MasterSuccess(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makeGetWithQuery(t, router, "/api/v1/query/schema/tokens", master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "tokens", data["table"])
	fields, ok := data["fields"].([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(fields), 1)
}
