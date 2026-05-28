package handlers

import (
	"encoding/json"
	"fmt"
	"maps"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
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
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        body  body  object  true  "Record to create"  example({"table_id":"tbl-1","data":{"title":"Hello","count":42}})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","table_id":"...","version":1,"created_at":"...","data":{...}}}"
// @Failure      400  {object}  map[string]any
// @Router       /records [post]
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
// @Summary      Export records
// @Description  Export records from a table as CSV or JSON file.
// @Tags         records
// @Accept       json
// @Produce      octet-stream
// @Security     ApiKeyAuth
// @Param        table_id  query  string  true   "Table ID"
// @Param        format    query  string  false  "Export format (csv or json)"  default(csv)
// @Param        filter    query  string  false  "JSON filter expression"
// @Success      200  {file}  binary
// @Failure      400  {object}  map[string]any
// @Router       /records/export [get]
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
// @Summary      List records
// @Description  Query records from a table with pagination and filtering.
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        table_id  query  string  true   "Table ID"
// @Param        limit     query  int     false  "Page size (1-100)"     default(20)
// @Param        offset    query  int     false  "Offset for pagination"  default(0)
// @Param        filter    query  string  false  "JSON filter expression"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"items":[...],"total":0,"has_more":false}}"
// @Failure      400  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /records [get]
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
// @Summary      Get a record
// @Description  Get a single record by ID.
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /records/{id} [get]
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
// @Description  Update record data. Supports optimistic locking via version field.
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id    path  string  true  "Record ID"
// @Param        body  body  object  true  "Record update"  example({"data":{"title":"Updated"},"version":1})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"id":"...","version":2,"updated_at":"...","data":{...}}}"
// @Failure      400  {object}  map[string]any
// @Router       /records/{id} [put]
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
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"message":"记录已删除"}}"
// @Failure      403  {object}  map[string]any
// @Router       /records/{id} [delete]
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
// @Tags         records
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        count  query  int     true   "Number of records to create (1-100)"  default(1)
// @Param        body   body  object  true  "Record template"  example({"table_id":"tbl-1","data":{"title":"Hello"}})
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"records":[...],"count":1}}"
// @Failure      400  {object}  map[string]any
// @Router       /records/batch [post]
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
