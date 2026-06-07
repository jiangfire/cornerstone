package handlers

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimplifiedQuery_PageNonNumeric(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&page=abc"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(1), data["page"])
}

func TestSimplifiedQuery_PageNegative(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&page=-1"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(1), data["page"])
}

func TestSimplifiedQuery_SizeZero(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&size=0"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20), data["size"])
}

func TestSimplifiedQuery_SizeExceedsMax(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&size=1001"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20), data["size"])
}

func TestSimplifiedQuery_SizeNonNumeric(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&size=abc"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	data, ok := resp["data"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, float64(20), data["size"])
}

func makePostEmptyBody(t *testing.T, router *gin.Engine, path, token string) *httptest.ResponseRecorder {
	t.Helper()
	req, err := http.NewRequest("POST", path, bytes.NewBufferString(""))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func TestQueryExplain_EmptyQParam(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makePostEmptyBody(t, router, "/api/v1/query/explain", master.Token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "missing query parameter")
}

func TestQueryValidate_EmptyQParam(t *testing.T) {
	router, _, master := setupQueryTest(t)

	rec := makePostEmptyBody(t, router, "/api/v1/query/validate", master.Token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "missing query parameter")
}

func TestSimplifiedQuery_WithSelect(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	sel := url.QueryEscape(`["id","name"]`)
	path := fmt.Sprintf("/api/v1/query/simple?table=databases&select=%s", sel)
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestSimplifiedQuery_WithSort(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	path := "/api/v1/query/simple?table=databases&sort=name"
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestSimplifiedQuery_WithQ(t *testing.T) {
	router, db, master := setupQueryTest(t)
	createQueryData(t, db)

	q := url.QueryEscape(`{"name":"querydb"}`)
	path := fmt.Sprintf("/api/v1/query/simple?table=databases&q=%s", q)
	rec := makeGetWithQuery(t, router, path, master.Token)

	assert.Equal(t, http.StatusOK, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Equal(t, float64(0), resp["code"])
}

func TestQueryExplain_QParamInvalidJSON(t *testing.T) {
	router, _, master := setupQueryTest(t)

	path := fmt.Sprintf("/api/v1/query/explain?q=%s", url.QueryEscape("not-json"))
	rec := makePostEmptyBody(t, router, path, master.Token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "invalid query format")
}

func TestQueryValidate_QParamInvalidJSON(t *testing.T) {
	router, _, master := setupQueryTest(t)

	path := fmt.Sprintf("/api/v1/query/validate?q=%s", url.QueryEscape("not-json"))
	rec := makePostEmptyBody(t, router, path, master.Token)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeQueryResp(t, rec)
	assert.Contains(t, resp["message"], "invalid query format")
}
