package middleware

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesUUID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	RequestID()(c)

	rid := GetRequestID(c)
	assert.NotEmpty(t, rid)
	assert.Len(t, rid, 36)
}

func TestRequestID_UsesProvidedHeader(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Request-ID", "custom-id-123")

	RequestID()(c)

	rid := GetRequestID(c)
	assert.Equal(t, "custom-id-123", rid)
}

func TestRequestID_SetsResponseHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Request-ID", "req-123")

	RequestID()(c)

	assert.Equal(t, "req-123", w.Header().Get("X-Request-ID"))
	assert.NotEmpty(t, w.Header().Get("X-Trace-ID"))
}

func TestRequestID_GeneratedSetsResponseHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	RequestID()(c)

	assert.NotEmpty(t, w.Header().Get("X-Request-ID"))
	assert.NotEmpty(t, w.Header().Get("X-Trace-ID"))
}

func TestRequestID_TraceParent(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	traceID := "0af7651916cd43dd8448eb211c80319c"
	c.Request.Header.Set("traceparent", "00-"+traceID+"-00f067aa0ba902b7-01")

	RequestID()(c)

	tid := GetTraceID(c)
	assert.Equal(t, traceID, tid)
}

func TestRequestID_TraceParentWithXTraceID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("traceparent", "00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01")
	c.Request.Header.Set("X-Trace-ID", "custom-trace")

	RequestID()(c)
	assert.Equal(t, "0af7651916cd43dd8448eb211c80319c", GetTraceID(c))
}

func TestRequestID_XTraceID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Trace-ID", "my-trace-id")

	RequestID()(c)

	tid := GetTraceID(c)
	assert.Equal(t, "my-trace-id", tid)
}

func TestRequestID_FallbackToRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Request-ID", "req-fallback")

	RequestID()(c)

	tid := GetTraceID(c)
	assert.Equal(t, "req-fallback", tid)
}

func TestCORS_SetsHeaders(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	CORS()(c)

	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Origin, Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
	assert.Equal(t, "false", w.Header().Get("Access-Control-Allow-Credentials"))
}

func TestCORS_OptionsPreflight(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("OPTIONS", "/", nil)

	CORS()(c)

	assert.Equal(t, 204, w.Code)
}

func TestCORS_NonOptionsPassesThrough(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	called := false
	CORS()(c)
	_ = called

	assert.NotEqual(t, 204, w.Code)
	assert.True(t, c.IsAborted() == false)
}

func TestGetRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	assert.Equal(t, "", GetRequestID(c))

	c.Set("request_id", "test-rid")
	assert.Equal(t, "test-rid", GetRequestID(c))
}

func TestGetRequestID_WrongType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	c.Set("request_id", 12345)
	assert.Equal(t, "", GetRequestID(c))
}

func TestGetTraceID(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	assert.Equal(t, "", GetTraceID(c))

	c.Set("trace_id", "test-tid")
	assert.Equal(t, "test-tid", GetTraceID(c))
}

func TestGetTraceID_WrongType(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)

	c.Set("trace_id", 12345)
	assert.Equal(t, "", GetTraceID(c))
}

func TestParseTraceID_Valid(t *testing.T) {
	traceID := parseTraceID("00-0af7651916cd43dd8448eb211c80319c-00f067aa0ba902b7-01")
	assert.Equal(t, "0af7651916cd43dd8448eb211c80319c", traceID)
}

func TestParseTraceID_InvalidTooShort(t *testing.T) {
	traceID := parseTraceID("00-abc-123-01")
	assert.Equal(t, "", traceID)
}

func TestParseTraceID_Empty(t *testing.T) {
	traceID := parseTraceID("")
	assert.Equal(t, "", traceID)
}

func TestParseTraceID_InvalidFewParts(t *testing.T) {
	traceID := parseTraceID("00-0af7651916cd43dd8448eb211c80319c")
	assert.Equal(t, "", traceID)
}

func TestRequestID_TrimsWhitespace(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/", nil)
	c.Request.Header.Set("X-Request-ID", "  spaced-id  ")

	RequestID()(c)

	rid := GetRequestID(c)
	require.NotNil(t, rid)
	assert.Equal(t, "spaced-id", rid)
}
