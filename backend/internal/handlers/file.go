package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// UploadFile 上传文件
func UploadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	recordID := c.PostForm("record_id")

	if recordID == "" {
		types.Error(c, 400, "记录ID不能为空")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		types.Error(c, 400, "请选择要上传的文件")
		return
	}

	req := services.UploadFileRequest{
		RecordID: recordID,
		File:     file,
	}

	fileService := services.NewFileService(db.DB())
	uploadedFile, err := fileService.UploadFile(req, userID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, uploadedFile)
}

// GetFile 获取文件信息
func GetFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	file, err := fileService.GetFile(fileID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, file)
}

// DownloadFile 下载文件
func DownloadFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	file, err := fileService.GetFile(fileID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	c.FileAttachment(file.StorageURL, file.FileName)
}

// DeleteFile 删除文件
func DeleteFile(c *gin.Context) {
	userID := middleware.GetUserID(c)
	fileID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	if err := fileService.DeleteFile(fileID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, "文件删除成功")
}

// ListRecordFiles 列出记录的所有文件
func ListRecordFiles(c *gin.Context) {
	userID := middleware.GetUserID(c)
	recordID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	files, err := fileService.ListRecordFiles(recordID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, files)
}
