package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreateRecord 创建记录
func CreateRecord(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.CreateRecord(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	// 解析数据返回
	var data interface{}
	if record.Data != "" {
		_ = json.Unmarshal([]byte(record.Data), &data)
	}

	types.Success(c, gin.H{
		"id":         record.ID,
		"table_id":   record.TableID,
		"data":       data,
		"version":    record.Version,
		"created_at": record.CreatedAt,
	})
}

// ExportRecords 导出记录
func ExportRecords(c *gin.Context) {
	userID := middleware.GetUserID(c)
	tableID := c.Query("table_id")
	if tableID == "" {
		types.Error(c, 400, "table_id 不能为空")
		return
	}

	format := c.DefaultQuery("format", "csv")
	filter := c.Query("filter")

	recordService := services.NewRecordService(db.DB())
	data, contentType, filename, err := recordService.ExportRecords(tableID, userID, format, filter)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, contentType, data)
}

// ListRecords 获取记录列表
func ListRecords(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.QueryRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	result, err := recordService.ListRecords(req, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"records":  result.Records,
		"total":    result.Total,
		"has_more": result.HasMore,
	})
}

// GetRecord 获取单个记录
func GetRecord(c *gin.Context) {
	userID := middleware.GetUserID(c)
	recordID := c.Param("id")

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.GetRecord(recordID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, record)
}

// UpdateRecord 更新记录
func UpdateRecord(c *gin.Context) {
	userID := middleware.GetUserID(c)
	recordID := c.Param("id")

	var req services.UpdateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	recordService := services.NewRecordService(db.DB())
	record, err := recordService.UpdateRecord(recordID, req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	// 解析数据返回
	var data interface{}
	if record.Data != "" {
		_ = json.Unmarshal([]byte(record.Data), &data)
	}

	types.Success(c, gin.H{
		"id":         record.ID,
		"data":       data,
		"version":    record.Version,
		"updated_at": record.UpdatedAt,
	})
}

// DeleteRecord 删除记录
func DeleteRecord(c *gin.Context) {
	userID := middleware.GetUserID(c)
	recordID := c.Param("id")

	recordService := services.NewRecordService(db.DB())
	if err := recordService.DeleteRecord(recordID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "记录已删除",
	})
}

// BatchCreateRecords 批量创建记录
func BatchCreateRecords(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateRecordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	// 获取批量数量参数
	count := c.DefaultQuery("count", "1")
	var batchCount int
	if _, err := fmt.Sscanf(count, "%d", &batchCount); err != nil || batchCount < 1 || batchCount > 100 {
		types.Error(c, 400, "批量数量必须在1-100之间")
		return
	}

	recordService := services.NewRecordService(db.DB())
	records, err := recordService.BatchCreateRecords(req, userID, batchCount)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	// 解析数据返回
	result := make([]interface{}, len(records))
	for i, record := range records {
		var data interface{}
		if record.Data != "" {
			_ = json.Unmarshal([]byte(record.Data), &data)
		}
		result[i] = gin.H{
			"id":      record.ID,
			"data":    data,
			"version": record.Version,
		}
	}

	types.Success(c, gin.H{
		"records": result,
		"count":   len(records),
	})
}
