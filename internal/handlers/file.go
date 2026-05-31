package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/internal/middleware"
	"github.com/jiangfire/cornerstone/internal/services"
	"github.com/jiangfire/cornerstone/pkg/db"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

// UploadFile
//
// @Summary      Upload a file
// @Description  Upload a file attachment associated with a record and/or field.
//
//	At least one of record_id or field_id is required. The file is stored
//	locally and metadata is recorded in the database. File size and type
//	restrictions may apply based on field configuration.
//
// @Tags         files
// @Accept       multipart/form-data
// @Produce      json
// @Security     ApiKeyAuth
// @Param        file       formData  file    true   "File to upload"
// @Param        record_id  formData  string  false  "Record ID to attach to"
// @Param        field_id   formData  string  false  "Field ID to attach to"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FileObject}
// @Failure      400  {object}  swagger.ErrorResponse  "Validation error - missing file or record_id/field_id"
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to target record/field"
// @Router       /api/files/upload [post]
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
// @Description  Retrieve file metadata by ID, including file name, size, type, and storage path.
//
//	Does not return the file content itself. Use the download endpoint for that.
//	The authenticated token must have access to the associated record.
//
// @Tags         files
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "File ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FileObject}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this file"
// @Failure      404  {object}  swagger.ErrorResponse  "File not found"
// @Router       /api/files/{id} [get]
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
//
//	Returns the file as a binary attachment with Content-Disposition header.
//	The authenticated token must have access to the associated record.
//
// @Tags         files
// @Produce      application/octet-stream
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "File ID"
// @Success      200  {file}  binary
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this file"
// @Failure      404  {object}  swagger.ErrorResponse  "File not found"
// @Router       /api/files/{id}/download [get]
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
// @Description  Delete a file and its metadata by ID.
//
//	This action is irreversible. The physical file is removed from storage.
//	The authenticated token must own the associated record.
//
// @Tags         files
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "File ID"
// @Success      200  {object}  swagger.APIResponse{data=object}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this file"
// @Failure      404  {object}  swagger.ErrorResponse  "File not found"
// @Router       /api/files/{id} [delete]
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
//
//	Includes file metadata (name, size, type) for each attachment.
//	The authenticated token must have access to the record.
//
// @Tags         files
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id  path  string  true  "Record ID"
// @Success      200  {object}  swagger.APIResponse{data=swagger.FileListResponse}
// @Failure      401  {object}  swagger.ErrorResponse  "Unauthorized - invalid or missing API key"
// @Failure      403  {object}  swagger.ErrorResponse  "Forbidden - no access to this record"
// @Router       /api/records/{id}/files [get]
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
