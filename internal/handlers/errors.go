package handlers

import (
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

// handleServiceError discriminates between not-found, permission, and generic errors
func handleServiceError(c *gin.Context, err error) {
	if isNotFoundError(err) {
		dto.NotFound(c, err.Error())
		return
	}
	if isPermissionError(err) {
		dto.Forbidden(c, err.Error())
		return
	}
	dto.InternalServerError(c, err.Error())
}

// isPermissionError checks if error is permission-related
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	permissionKeywords := []string{
		"denied", "forbidden", "unauthorized", "permission", "master token required",
	}
	for _, keyword := range permissionKeywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

// isNotFoundError checks if error is a not-found error
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	notFoundKeywords := []string{
		"not found", "does not exist",
	}
	for _, keyword := range notFoundKeywords {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}
