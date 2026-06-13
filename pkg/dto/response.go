package dto

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Success(c *gin.Context, data any) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: data})
}

func SuccessWithMessage(c *gin.Context, message string, data any) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: message, Data: data})
}

func Error(c *gin.Context, code int, message string) {
	c.JSON(code, APIResponse{Code: code, Message: message})
}

func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, message)
}

func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, message)
}

func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, message)
}

func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, message)
}

func InternalServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, message)
}
