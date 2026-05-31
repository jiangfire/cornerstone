package handlers

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
	"go.uber.org/zap"
)

// decodeRecordData 将 Record.Data(JSON 字符串)解码为可序列化值。
// 解析失败时记录 Warn 日志,返回空对象 + corrupted=true 供调用方在响应里打标记,
// 避免历史脏数据让整个接口 500,但客户端仍能感知到需要修复。
func decodeRecordData(record *models.Record) (any, bool) {
	if record == nil || record.Data == "" {
		return map[string]any{}, false
	}
	var data any
	if err := json.Unmarshal([]byte(record.Data), &data); err != nil {
		zap.L().Warn("record data corrupted",
			zap.String("id", record.ID),
			zap.String("table_id", record.TableID),
			zap.Error(err),
		)
		return map[string]any{}, true
	}
	return data, false
}

func recordResponseWithData(record *models.Record, extra gin.H) gin.H {
	data, corrupted := decodeRecordData(record)
	resp := gin.H{"data": data}
	maps.Copy(resp, extra)
	if corrupted {
		resp["_corrupted"] = true
	}
	return resp
}

// CreateRecord 创建记录
//
// @Summary      Create a record
// @Description  Create a new record in a table.
//
//	The data field is a key-value map where keys correspond to field names
//	in the target table. Values must match the field types defined in the schema.
//	The table_id field is required.
//
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  swagger.RecordCreateRequest  true  "Record to create"
// @Success      200  {object}  swagger.APIResponse{data=swagger.RecordObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body or field values"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records [post]
func CreateRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req services.CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.CreateRecord(req, userID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, recordResponseWithData(record, gin.H{
		"id":         record.ID,
		"table_id":   record.TableID,
		"version":    record.Version,
		"created_at": record.CreatedAt,
	}))
}

// ExportRecords 导出记录
//
// @Summary      Export records as CSV or JSON
// @Description  Export records from a table as a downloadable file.
//
//	Supported formats: csv (default) and json. An optional JSON filter expression
//	can be provided to export only matching records. The response includes
//	Content-Disposition header for browser downloads.
//
// @Tags         records
// @Produce      application/octet-stream
// @Security     ApiKeyAuth
// @Param        table_id  query  string  true   "Table ID"
// @Param        format    query  string  false  "Export format: csv or json"  default(csv)
// @Param        filter    query  string  false  "JSON filter expression"
// @Success      200  {file}  binary
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - missing table_id or invalid format"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records/export [get]
func ExportRecords(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Query("table_id")
	if tableID == "" {
		dto.Error(c, 400, "table_id 不能为空")
		return
	}

	format := c.DefaultQuery("format", "csv")
	filter := c.Query("filter")

	recordService := services.NewRecordService(db.DB())
	data, contentType, filename, err := recordService.ExportRecords(tableID, userID, format, filter)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, data)
}

// ListRecords 获取记录列表
//
// @Summary      List records in a table
// @Description  Query records from a table with pagination and optional filtering.
//
//	The table_id query parameter is required. Use limit and offset for pagination.
//	An optional JSON filter expression can be provided to narrow results.
//
// @Tags         records
// @Produce      json
// @Security     ApiKeyAuth
// @Param        table_id  query  string  true   "Table ID"
// @Param        limit     query  int     false  "Page size (1-100)"  default(20)
// @Param        offset    query  int     false  "Offset for pagination"  default(0)
// @Param        filter    query  string  false  "JSON filter expression"
// @Success      200  {object}  swagger.APIResponse{data=swagger.RecordListResponse}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - missing table_id"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records [get]
func ListRecords(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req services.QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	result, err := recordService.ListRecords(req, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"items":    result.Records,
		"total":    result.Total,
		"has_more": result.HasMore,
	})
}

// GetRecord 获取单个记录
//
// @Summary      Get a record by ID
// @Description  Retrieve a single record by its ID.
//
//	Returns the full record data including all field values.
//	The authenticated token must have access to the parent table.
//
// @Tags         records
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.RecordObject}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this record"
// @Failure      404  {object}  swagger.ErrorResponse  "Record not found"
// @Router       /api/v1/records/{id} [get]
func GetRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.GetRecord(recordID, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, record)
}

// UpdateRecord 更新记录
//
// @Summary      Update a record
// @Description  Update record data. Supports optimistic locking via the version field.
//
//	If a version is provided, the server will check that it matches the current
//	record version. If it does not match, a conflict error is returned.
//	Omit the version field to skip optimistic locking.
//
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string                 true  "Record ID"
// @Param        body  body  swagger.RecordUpdateRequest  true  "Record update with optional version"
// @Success      200  {object}  swagger.APIResponse{data=swagger.RecordObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error or version conflict"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this record"
// @Failure      404  {object}  swagger.ErrorResponse  "Record not found"
// @Router       /api/v1/records/{id} [put]
func UpdateRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	var req services.UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.UpdateRecord(recordID, req, userID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, recordResponseWithData(record, gin.H{
		"id":         record.ID,
		"version":    record.Version,
		"updated_at": record.UpdatedAt,
	}))
}

// DeleteRecord 删除记录
//
// @Summary      Delete a record
// @Description  Delete a record by ID.
//
//	This action is irreversible. The authenticated token must have access
//	to the parent table.
//
// @Tags         records
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this record"
// @Failure      404  {object}  swagger.ErrorResponse  "Record not found"
// @Router       /api/v1/records/{id} [delete]
func DeleteRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	recordService := services.NewRecordService(db.DB())
	if err := recordService.DeleteRecord(recordID, userID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{
		"message": "记录已删除",
	})
}

// BatchCreateRecords 批量创建记录
//
// @Summary      Batch create records
// @Description  Create multiple identical records in one request (1-100 records).
//
//	The count query parameter specifies how many copies to create (default 1, max 100).
//	All records will have the same data values from the request body.
//	The table_id field is required in the request body.
//
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        count  query  int                        true   "Number of records to create (1-100)"  default(1)
// @Param        body   body   swagger.RecordBatchCreateRequest  true  "Record template"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - invalid request body or count out of range"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records/batch [post]
func BatchCreateRecords(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req services.CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	// 获取批量数量参数
	count := c.DefaultQuery("count", "1")
	var batchCount int
	if _, err := fmt.Sscanf(count, "%d", &batchCount); err != nil || batchCount < 1 || batchCount > 100 {
		dto.Error(c, 400, "批量数量必须在1-100之间")
		return
	}

	recordService := services.NewRecordService(db.DB())
	records, err := recordService.BatchCreateRecords(req, userID, batchCount)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	// 解析数据返回
	result := make([]interface{}, len(records))
	for i, record := range records {
		result[i] = recordResponseWithData(record, gin.H{
			"id":      record.ID,
			"version": record.Version,
		})
	}

	dto.Success(c, gin.H{
		"records": result,
		"count":   len(records),
	})
}
