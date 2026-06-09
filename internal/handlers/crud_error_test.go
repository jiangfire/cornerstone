package handlers

import (
	"net/http"
	"testing"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListDatabases_Error(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Close()

	rec := doJSON(t, router, "GET", "/api/v1/databases/", master.Token, nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestGetDatabase_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/databases/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestUpdateDatabase_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]string{"name": "xx", "description": "yy"}
	rec := doJSON(t, router, "PUT", "/api/v1/databases/nonexistent-id", master.Token, body)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDeleteDatabase_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "DELETE", "/api/v1/databases/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestCreateDatabase_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]string{"name": "", "description": ""}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestCreateDatabase_RegularTokenForbidden(t *testing.T) {
	router, db, _ := setupCRUDTest(t)

	client := &models.Token{Name: "regular", IsMaster: false, Scopes: "{}"}
	require.NoError(t, db.Create(client).Error)

	body := map[string]string{"name": "forbidden_db"}
	rec := doJSON(t, router, "POST", "/api/v1/databases/", client.Token, body)

	assert.Equal(t, http.StatusForbidden, rec.Code)
	resp := decodeResp(t, rec)
	assert.Equal(t, float64(http.StatusForbidden), resp["code"])
	assert.Contains(t, resp["message"], "master token required")
}

func TestCreateDatabaseWithTables_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "POST", "/api/v1/databases/with-tables", master.Token, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestCreateDatabaseWithTables_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{
		"name": "",
	}
	rec := doJSON(t, router, "POST", "/api/v1/databases/with-tables", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestListTables_ServiceError(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	dbModel := createDBDirect(t, db, "testdb")

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Close()

	rec := doJSON(t, router, "GET", "/api/v1/databases/"+dbModel.ID+"/tables", master.Token, nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestGetTable_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/tables/detail/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestUpdateTable_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "PUT", "/api/v1/tables/some-id", master.Token, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestUpdateTable_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]string{"name": "xx", "description": "yy"}
	rec := doJSON(t, router, "PUT", "/api/v1/tables/nonexistent-id", master.Token, body)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDeleteTable_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "DELETE", "/api/v1/tables/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestCreateTable_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]string{"database_id": "nonexistent-id", "name": "t"}
	rec := doJSON(t, router, "POST", "/api/v1/tables/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestListFields_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/tables/nonexistent-id/fields", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestGetField_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "GET", "/api/v1/fields/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestUpdateField_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "PUT", "/api/v1/fields/some-id", master.Token, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestUpdateField_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{"name": "x", "type": "string"}
	rec := doJSON(t, router, "PUT", "/api/v1/fields/nonexistent-id", master.Token, body)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDeleteField_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	rec := doJSON(t, router, "DELETE", "/api/v1/fields/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestCreateField_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	body := map[string]interface{}{"table_id": "nonexistent-id", "name": "f", "type": "string"}
	rec := doJSON(t, router, "POST", "/api/v1/fields/", master.Token, body)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestCreateToken_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	rec := doJSON(t, router, "POST", "/api/v1/tokens/", master.Token, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestUpdateToken_BindingError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	rec := doJSON(t, router, "PUT", "/api/v1/tokens/some-id", master.Token, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	resp := decodeResp(t, rec)
	assert.Contains(t, resp["message"], "invalid request")
}

func TestUpdateToken_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	body := map[string]string{"scopes": "read,write"}
	rec := doJSON(t, router, "PUT", "/api/v1/tokens/nonexistent-id", master.Token, body)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestDeleteToken_ServiceError(t *testing.T) {
	router, _, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	rec := doJSON(t, router, "DELETE", "/api/v1/tokens/nonexistent-id", master.Token, nil)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}

func TestListTokens_ServiceError(t *testing.T) {
	router, db, master := setupCRUDTest(t)

	t.Setenv("MASTER_TOKEN", master.Token)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.Close()

	rec := doJSON(t, router, "GET", "/api/v1/tokens/", master.Token, nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	resp := decodeResp(t, rec)
	assert.NotEqual(t, float64(0), resp["code"])
}
