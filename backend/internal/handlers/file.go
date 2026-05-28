package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// UploadFile
//
// @Summary      Upload a file
// @Description  Upload a file attachment. At least one of record_id or field_id is required.
// @Tags         files
// @Accept       multipart/form-data
// @Produce      json
// @Security     ApiKeyAuth
// @Param        file       formData  file    true  "File to upload"
// @Param        record_id  formData  string  false "Record ID"
// @Param        field_id   formData  string  false "Field ID"
// @Success      200  {object}  map[string]any
// @Failure      400  {object}  map[string]any
// @Router       /files/upload [post]
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
//
// @Summary      Get file metadata
// @Description  Get file metadata by ID.
// @Tags         files
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "File ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /files/{id} [get]
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
//
// @Summary      Download a file
// @Description  Download the actual file content by ID.
// @Tags         files
// @Accept       json
// @Produce      octet-stream
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "File ID"
// @Success      200  {file}  binary
// @Failure      403  {object}  map[string]any
// @Router       /files/{id}/download [get]
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
//
// @Summary      Delete a file
// @Description  Delete a file by ID.
// @Tags         files
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "File ID"
// @Success      200  {object}  map[string]any
// @Failure      403  {object}  map[string]any
// @Router       /files/{id} [delete]
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
//
// @Summary      List files for a record
// @Description  Returns all files attached to a record.
// @Tags         files
// @Accept       json
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  map[string]any  "{"code":0,"data":{"items":[...]}}"
// @Failure      403  {object}  map[string]any
// @Router       /records/{id}/files [get]
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
