package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// UploadFile
func UploadFile(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	recordID := c.PostForm("record_id")
	fieldID := c.PostForm("field_id")

	if recordID == "" && fieldID == "" {
		dto.Error(c, 400, "记录ID或字段ID不能为空")
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		dto.Error(c, 400, "请选择要上传的文件")
		return
	}

	req := services.UploadFileRequest{
		RecordID: recordID,
		FieldID:  fieldID,
		File:     file,
	}

	fileService := services.NewFileService(db.DB())
	uploadedFile, err := fileService.UploadFile(req, tokenID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, uploadedFile)
}

// GetFile
func GetFile(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	fileID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	file, err := fileService.GetFile(fileID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, file)
}

// DownloadFile
func DownloadFile(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	fileID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	file, err := fileService.GetFile(fileID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	safePath, err := services.ResolveSecureStoragePath(file.StorageURL)
	if err != nil {
		dto.Error(c, 403, "文件路径不合法")
		return
	}
	c.FileAttachment(safePath, file.FileName)
}

// DeleteFile
func DeleteFile(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	fileID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	if err := fileService.DeleteFile(fileID, tokenID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, "文件删除成功")
}

// ListRecordFiles
func ListRecordFiles(c *gin.Context) {
	tokenID := middleware.GetTokenID(c)
	recordID := c.Param("id")

	fileService := services.NewFileService(db.DB())
	files, err := fileService.ListRecordFiles(recordID, tokenID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, gin.H{"items": files})
}
