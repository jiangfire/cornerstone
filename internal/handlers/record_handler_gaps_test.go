package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateRecord_NonexistentTable(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"table_id": "nonexistent_table_id",
		"data":     map[string]interface{}{"title": "hello"},
	}
	rec := doJSON(t, router, "POST", "/api/v1/records/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestExportRecords_MissingTableID(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	req, err := http.NewRequest("GET", "/api/v1/records/export", nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestExportRecords_InvalidFormat(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	createRecordDirect(t, db, tbl.ID, map[string]interface{}{"title": "row1"})

	path := fmt.Sprintf("/api/v1/records/export?table_id=%s&format=xml", tbl.ID)
	req, err := http.NewRequest("GET", path, nil)
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetRecord_NotFound(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/records/nonexistent_record_id", master.Token, nil)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestUpdateRecord_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	req, err := http.NewRequest("PUT", "/api/v1/records/some_id", strings.NewReader(""))
	require.NoError(t, err)
	req.Header.Set("Authorization", "Bearer "+master.Token)
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateRecord_NotFound(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"data":    map[string]interface{}{"title": "updated"},
		"version": 1,
	}
	rec := doJSON(t, router, "PUT", "/api/v1/records/nonexistent_record_id", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDeleteRecord_NotFound(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "DELETE", "/api/v1/records/nonexistent_record_id", master.Token, nil)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestBatchCreateRecords_MissingTableID(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"data": map[string]interface{}{"title": "batch"},
	}
	rec := doJSON(t, router, "POST", "/api/v1/records/batch?count=1", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "参数错误")
}

func TestBatchCreateRecords_CountOutOfRange(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	body := map[string]interface{}{
		"table_id": tbl.ID,
		"data":     map[string]interface{}{"title": "batch"},
	}

	t.Run("count_zero", func(t *testing.T) {
		rec := doJSON(t, router, "POST", "/api/v1/records/batch?count=0", master.Token, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		resp := decodeResp(t, rec)
		assert.Contains(t, resp["message"], "批量数量必须在1-100之间")
	})

	t.Run("count_101", func(t *testing.T) {
		rec := doJSON(t, router, "POST", "/api/v1/records/batch?count=101", master.Token, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		resp := decodeResp(t, rec)
		assert.Contains(t, resp["message"], "批量数量必须在1-100之间")
	})

	t.Run("count_negative", func(t *testing.T) {
		rec := doJSON(t, router, "POST", "/api/v1/records/batch?count=-1", master.Token, body)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		resp := decodeResp(t, rec)
		assert.Contains(t, resp["message"], "批量数量必须在1-100之间")
	})
}

func TestBatchCreateRecords_NonNumericCount(t *testing.T) {
	router, db, master := setupCRUDTest(t)
	_, tbl, _ := setupRecordPrereqs(t, db)

	body := map[string]interface{}{
		"table_id": tbl.ID,
		"data":     map[string]interface{}{"title": "batch"},
	}

	rec := doJSON(t, router, "POST", "/api/v1/records/batch?count=abc", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "批量数量必须在1-100之间")
}

func TestDecodeRecordData_NilRecord(t *testing.T) {
	data, corrupted := decodeRecordData(nil)
	assert.False(t, corrupted)

	asMap, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, asMap)
}

func TestBatchCreateRecords_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"table_id": "nonexistent_table_id",
		"data":     map[string]interface{}{"title": "batch"},
	}
	rec := doJSON(t, router, "POST", "/api/v1/records/batch?count=3", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}
