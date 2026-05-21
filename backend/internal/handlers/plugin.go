package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
	"github.com/jiangfire/cornerstone/backend/pkg/dto"
)

// CreatePlugin 创建插件
func CreatePlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	plugin, err := pluginService.CreatePlugin(req, userID)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, plugin)
}

// ListPlugins 列出插件
//
// @Param page query int false "Page number (1-based, default 1)"
// @Param page_size query int false "Items per page (default 20, max 200)"
func ListPlugins(c *gin.Context) {
	userID := middleware.GetUserID(c)

	page, _ := strconv.Atoi(c.Query("page"))
	pageSize, _ := strconv.Atoi(c.Query("page_size"))

	pluginService := services.NewPluginService(db.DB())
	result, err := pluginService.ListPlugins(userID, services.PluginListFilter{
		Page:     page,
		PageSize: pageSize,
	})
	if err != nil {
		dto.Error(c, 500, err.Error())
		return
	}

	dto.Success(c, result)
}

// GetPlugin 获取插件详情
func GetPlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	pluginService := services.NewPluginService(db.DB())
	plugin, err := pluginService.GetPlugin(pluginID, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, plugin)
}

// UpdatePlugin 更新插件
func UpdatePlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	var req services.UpdatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.UpdatePlugin(pluginID, req, userID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, "更新成功")
}

// DeletePlugin 删除插件
func DeletePlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.DeletePlugin(pluginID, userID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, "删除成功")
}

// BindPlugin 绑定插件
func BindPlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	var req struct {
		TableID string `json:"table_id" binding:"required"`
		Trigger string `json:"trigger" binding:"required,oneof=create update delete manual"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.BindPlugin(pluginID, req.TableID, req.Trigger, userID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, "绑定成功")
}

// UnbindPlugin 解绑插件
func UnbindPlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	var req struct {
		TableID string `json:"table_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.UnbindPlugin(pluginID, req.TableID, userID); err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, "解绑成功")
}

// ListPluginBindings 列出插件的所有绑定
func ListPluginBindings(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	pluginService := services.NewPluginService(db.DB())
	bindings, err := pluginService.ListBindings(pluginID, userID)
	if err != nil {
		dto.Error(c, 403, err.Error())
		return
	}

	dto.Success(c, bindings)
}

// ExecutePlugin 手动执行插件
func ExecutePlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	var req services.ExecutePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		dto.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	execution, err := pluginService.ExecutePlugin(pluginID, userID, req)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, execution)
}

// ListPluginExecutions 查询插件执行记录
func ListPluginExecutions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	pluginService := services.NewPluginService(db.DB())
	executions, err := pluginService.ListExecutions(pluginID, userID, limit)
	if err != nil {
		dto.Error(c, 400, err.Error())
		return
	}

	dto.Success(c, executions)
}
