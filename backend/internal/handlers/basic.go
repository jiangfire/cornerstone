package handlers

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// Register 用户注册
func Register(c *gin.Context) {
	var req services.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	authService := services.NewAuthService(db.DB())
	response, err := authService.Register(req)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, response)
}

// Login 用户登录
func Login(c *gin.Context) {
	var req services.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	authService := services.NewAuthService(db.DB())
	response, err := authService.Login(req)
	if err != nil {
		types.Error(c, 401, err.Error())
		return
	}

	types.Success(c, response)
}

// GetUserInfo 获取用户信息
func GetUserInfo(c *gin.Context) {
	userID := middleware.GetUserID(c)

	authService := services.NewAuthService(db.DB())
	user, err := authService.GetUserByID(userID)
	if err != nil {
		types.Error(c, 404, err.Error())
		return
	}

	types.Success(c, user)
}

// UpdateUserInfo 更新用户资料
func UpdateUserInfo(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	authService := services.NewAuthService(db.DB())
	user, err := authService.UpdateProfile(userID, req)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, user)
}

// ChangeUserPassword 修改用户密码
func ChangeUserPassword(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	authService := services.NewAuthService(db.DB())
	if err := authService.ChangePassword(userID, req); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "密码修改成功",
	})
}

// DeleteUserAccount 删除用户账户
func DeleteUserAccount(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.DeleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	// 取出当前会话 token 以便服务层在同事务里加入黑名单
	currentToken := c.GetHeader("Authorization")
	if len(currentToken) > 7 && currentToken[:7] == "Bearer " {
		currentToken = currentToken[7:]
	}

	authService := services.NewAuthService(db.DB())
	if err := authService.DeleteAccount(userID, currentToken, req); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "账户已删除",
	})
}

// Logout 用户登出
func Logout(c *gin.Context) {
	// 从 Authorization header 获取 token
	token := c.GetHeader("Authorization")
	if len(token) > 7 && token[:7] == "Bearer " {
		token = token[7:]
	}

	authService := services.NewAuthService(db.DB())
	if err := authService.Logout(token); err != nil {
		types.Error(c, 500, "登出失败: "+err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "登出成功",
	})
}

const avatarUploadDir = "./uploads/avatars"
const avatarMaxSize = 2 * 1024 * 1024 // 2MB

var avatarAllowedTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// UploadAvatar 上传用户头像
func UploadAvatar(c *gin.Context) {
	userID := middleware.GetUserID(c)

	file, err := c.FormFile("file")
	if err != nil {
		types.Error(c, 400, "请选择要上传的文件")
		return
	}

	if file.Size > avatarMaxSize {
		types.Error(c, 400, "头像文件不能超过 2MB")
		return
	}

	contentType := strings.ToLower(strings.TrimSpace(file.Header.Get("Content-Type")))
	ext, ok := avatarAllowedTypes[contentType]
	if !ok {
		types.Error(c, 400, "仅支持 PNG / JPEG / WebP / GIF 格式")
		return
	}

	if err := os.MkdirAll(avatarUploadDir, 0o750); err != nil {
		types.Error(c, 500, "创建上传目录失败")
		return
	}

	filename := uuid.NewString() + ext
	targetPath := filepath.Join(avatarUploadDir, filename)

	// 防御路径穿越
	uploadDirAbs, err := filepath.Abs(avatarUploadDir)
	if err != nil {
		types.Error(c, 500, "路径解析失败")
		return
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		types.Error(c, 500, "路径解析失败")
		return
	}
	rel, err := filepath.Rel(uploadDirAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		types.Error(c, 400, "非法的文件路径")
		return
	}

	src, err := file.Open()
	if err != nil {
		types.Error(c, 500, "读取上传文件失败")
		return
	}
	defer src.Close()

	dst, err := os.Create(targetPath)
	if err != nil {
		types.Error(c, 500, "保存文件失败")
		return
	}
	defer dst.Close()

	if _, err := io.Copy(dst, src); err != nil {
		types.Error(c, 500, "保存文件失败")
		return
	}

	avatarURL := "/avatars/" + filename

	authService := services.NewAuthService(db.DB())
	if _, err := authService.UpdateAvatar(userID, avatarURL); err != nil {
		// 清理已保存的文件
		_ = os.Remove(targetPath)
		types.Error(c, 500, "更新头像失败")
		return
	}

	types.Success(c, gin.H{"avatar_url": avatarURL})
}

// ServeAvatar 公开提供头像文件
func ServeAvatar(c *gin.Context) {
	filename := c.Param("filename")
	filename = filepath.Base(filename)
	if filename == "." || filename == "/" || strings.Contains(filename, "..") {
		c.Status(http.StatusNotFound)
		return
	}

	targetPath := filepath.Join(avatarUploadDir, filename)
	uploadDirAbs, err := filepath.Abs(avatarUploadDir)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	targetAbs, err := filepath.Abs(targetPath)
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	rel, err := filepath.Rel(uploadDirAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		c.Status(http.StatusNotFound)
		return
	}

	// 根据扩展名设置 Content-Type
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	case ".gif":
		c.Header("Content-Type", "image/gif")
	}

	c.File(targetPath)
}
