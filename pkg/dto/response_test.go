package dto

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func setupTestContext(t *testing.T) (*gin.Context, *httptest.ResponseRecorder) {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	return c, w
}

func decodeResponse(t *testing.T, w *httptest.ResponseRecorder) HttpResult {
	t.Helper()
	var result HttpResult
	err := json.Unmarshal(w.Body.Bytes(), &result)
	require.NoError(t, err)
	return result
}

func TestSuccess(t *testing.T) {
	c, w := setupTestContext(t)
	Success(c, map[string]string{"key": "value"})

	assert.Equal(t, http.StatusOK, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, "", result.Message)
	require.NotNil(t, result.Data)
}

func TestSuccess_WithNilData(t *testing.T) {
	c, w := setupTestContext(t)
	Success(c, nil)

	assert.Equal(t, http.StatusOK, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, 0, result.Code)
}

func TestSuccessWithMessage(t *testing.T) {
	c, w := setupTestContext(t)
	SuccessWithMessage(c, "operation succeeded", map[string]int{"count": 42})

	assert.Equal(t, http.StatusOK, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, "operation succeeded", result.Message)
	require.NotNil(t, result.Data)
}

func TestSuccessWithMessage_WithNilData(t *testing.T) {
	c, w := setupTestContext(t)
	SuccessWithMessage(c, "done", nil)

	assert.Equal(t, http.StatusOK, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, 0, result.Code)
	assert.Equal(t, "done", result.Message)
}

func TestError(t *testing.T) {
	c, w := setupTestContext(t)
	Error(c, http.StatusConflict, "conflict occurred")

	assert.Equal(t, http.StatusConflict, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, http.StatusConflict, result.Code)
	assert.Equal(t, "conflict occurred", result.Message)
}

func TestBadRequest(t *testing.T) {
	c, w := setupTestContext(t)
	BadRequest(c, "invalid input")

	assert.Equal(t, http.StatusBadRequest, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, http.StatusBadRequest, result.Code)
	assert.Equal(t, "invalid input", result.Message)
}

func TestUnauthorized(t *testing.T) {
	c, w := setupTestContext(t)
	Unauthorized(c, "not authenticated")

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, http.StatusUnauthorized, result.Code)
	assert.Equal(t, "not authenticated", result.Message)
}

func TestForbidden(t *testing.T) {
	c, w := setupTestContext(t)
	Forbidden(c, "access denied")

	assert.Equal(t, http.StatusForbidden, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, http.StatusForbidden, result.Code)
	assert.Equal(t, "access denied", result.Message)
}

func TestNotFound(t *testing.T) {
	c, w := setupTestContext(t)
	NotFound(c, "resource not found")

	assert.Equal(t, http.StatusNotFound, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, http.StatusNotFound, result.Code)
	assert.Equal(t, "resource not found", result.Message)
}

func TestInternalServerError(t *testing.T) {
	c, w := setupTestContext(t)
	InternalServerError(c, "something went wrong")

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	result := decodeResponse(t, w)
	assert.Equal(t, http.StatusInternalServerError, result.Code)
	assert.Equal(t, "something went wrong", result.Message)
}
