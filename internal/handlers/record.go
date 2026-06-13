package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
	"go.uber.org/zap"
)

// decodeRecordData decodes Record.Data (JSON string) into serializable values.
// When parsing fails, log a Warn and return empty object + corrupted=true so the caller can mark it in the response, preventing historical dirty data from causing a 500 while still allowing the client to detect the need for repair.
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

func recordObjectFromModel(record *models.Record, extraFields map[string]any) dto.RecordObject {
	data, corrupted := decodeRecordData(record)
	obj := dto.RecordObject{
		Data: data,
	}
	if id, ok := extraFields["id"].(string); ok {
		obj.ID = id
	}
	if tableID, ok := extraFields["table_id"].(string); ok {
		obj.TableID = tableID
	}
	if v, ok := extraFields["version"].(int); ok {
		obj.Version = v
	}
	if corrupted {
		obj.Data = map[string]any{"_corrupted": true, "data": data}
	}
	return obj
}

// CreateRecord creates a record
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
// @Param        body  body  dto.RecordCreateRequest  true  "Record to create"
// @Success      200  {object}  dto.APIResponse{data=dto.RecordObject}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body or field values"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records [post]
func CreateRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req dto.RecordCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.CreateRecord(req, userID)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	dto.Success(c, recordObjectFromModel(record, map[string]any{
		"id":       record.ID,
		"table_id": record.TableID,
		"version":  record.Version,
	}))
}

// ExportRecords exports records
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
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - missing table_id or invalid format"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records/export [get]
func ExportRecords(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	tableID := c.Query("table_id")
	if tableID == "" {
		dto.Error(c, 400, "table ID is required")
		return
	}

	format := c.DefaultQuery("format", "csv")
	filter := c.Query("filter")

	recordService := services.NewRecordService(db.DB())
	data, contentType, filename, err := recordService.ExportRecords(tableID, userID, format, filter)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, data)
}

// ListRecords lists records
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
// @Param        fields    query  string  false  "Comma-separated field names to include in data"
// @Success      200  {object}  dto.APIResponse{data=dto.RecordListData}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - missing table_id"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records [get]
func ListRecords(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req dto.RecordListQueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	result, err := recordService.ListRecords(req, userID)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, result)
}

// GetRecord gets a single record
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
// @Param        fields  query  string  false  "Comma-separated field names to include in data"
// @Success      200  {object}  dto.APIResponse{data=dto.RecordObject}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to this record"
// @Failure      404  {object}  dto.ErrorResponse  "Record not found"
// @Router       /api/v1/records/{id} [get]
func GetRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	recordService := services.NewRecordService(db.DB())
	fields := c.Query("fields")
	record, err := recordService.GetRecord(recordID, userID, fields)
	if err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, record)
}

// UpdateRecord updates a record
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
// @Param        body  body  dto.RecordUpdateRequest  true  "Record update with optional version"
// @Success      200  {object}  dto.APIResponse{data=dto.RecordObject}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error or version conflict"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to this record"
// @Failure      404  {object}  dto.ErrorResponse  "Record not found"
// @Router       /api/v1/records/{id} [put]
func UpdateRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	var req dto.RecordUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.UpdateRecord(recordID, req, userID)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	dto.Success(c, recordObjectFromModel(record, map[string]any{
		"id":      record.ID,
		"version": record.Version,
	}))
}

// DeleteRecord deletes a record
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
// @Success      200  {object}  dto.APIResponse{data=object}
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to this record"
// @Failure      404  {object}  dto.ErrorResponse  "Record not found"
// @Router       /api/v1/records/{id} [delete]
func DeleteRecord(c *gin.Context) {
	userID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	recordService := services.NewRecordService(db.DB())
	if err := recordService.DeleteRecord(recordID, userID); err != nil {
		handleServiceError(c, err)
		return
	}

	dto.Success(c, dto.MessageData{Message: "record deleted"})
}

// BatchCreateRecords creates records in batch
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
// @Param        body   body   dto.RecordBatchCreateRequest  true  "Record template"
// @Success      200  {object}  dto.APIResponse{data=object}
// @Failure      400  {object}  dto.ErrorResponse  "Validation error - invalid request body or count out of range"
// @Failure      401  {object}  dto.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  dto.ErrorResponse  "Forbidden - no access to target table"
// @Router       /api/v1/records/batch [post]
func BatchCreateRecords(c *gin.Context) {
	userID := middleware.GetTokenID(c)

	var req dto.RecordCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "invalid request: "+err.Error())
		return
	}

	// Get batch count parameter
	count := c.DefaultQuery("count", "1")
	var batchCount int
	if _, err := fmt.Sscanf(count, "%d", &batchCount); err != nil || batchCount < 1 || batchCount > 100 {
		dto.Error(c, 400, "batch count must be between 1 and 100")
		return
	}

	recordService := services.NewRecordService(db.DB())
	records, err := recordService.BatchCreateRecords(req, userID, batchCount)
	if err != nil {
		handleCreateServiceError(c, err)
		return
	}

	result := make([]dto.RecordObject, len(records))
	for i, record := range records {
		result[i] = recordObjectFromModel(record, map[string]any{
			"id":      record.ID,
			"version": record.Version,
		})
	}

	dto.Success(c, dto.RecordBatchCreateData{
		Records: result,
		Count:   len(records),
	})
}
