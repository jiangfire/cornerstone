package handlers

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreatePlugin 创建插件
func CreatePlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	plugin, err := pluginService.CreatePlugin(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, plugin)
}

// ListPlugins 列出插件
func ListPlugins(c *gin.Context) {
	userID := middleware.GetUserID(c)

	pluginService := services.NewPluginService(db.DB())
	plugins, err := pluginService.ListPlugins(userID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, plugins)
}

// GetPlugin 获取插件详情
func GetPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	pluginService := services.NewPluginService(db.DB())
	plugin, err := pluginService.GetPlugin(pluginID)
	if err != nil {
		types.Error(c, 404, err.Error())
		return
	}

	types.Success(c, plugin)
}

// UpdatePlugin 更新插件
func UpdatePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	var req services.UpdatePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.UpdatePlugin(pluginID, req); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, "更新成功")
}

// DeletePlugin 删除插件
func DeletePlugin(c *gin.Context) {
	pluginID := c.Param("id")

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.DeletePlugin(pluginID); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, "删除成功")
}

// BindPlugin 绑定插件
func BindPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	var req struct {
		TableID string `json:"table_id" binding:"required"`
		Trigger string `json:"trigger" binding:"required,oneof=create update delete manual"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.BindPlugin(pluginID, req.TableID, req.Trigger); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, "绑定成功")
}

// UnbindPlugin 解绑插件
func UnbindPlugin(c *gin.Context) {
	pluginID := c.Param("id")

	var req struct {
		TableID string `json:"table_id" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	if err := pluginService.UnbindPlugin(pluginID, req.TableID); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, "解绑成功")
}

// ListPluginBindings 列出插件的所有绑定
func ListPluginBindings(c *gin.Context) {
	pluginID := c.Param("id")

	pluginService := services.NewPluginService(db.DB())
	bindings, err := pluginService.ListBindings(pluginID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, bindings)
}

// ExecutePlugin 手动执行插件
func ExecutePlugin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")

	var req services.ExecutePluginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	pluginService := services.NewPluginService(db.DB())
	execution, err := pluginService.ExecutePlugin(pluginID, userID, req)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, execution)
}

// ListPluginExecutions 查询插件执行记录
func ListPluginExecutions(c *gin.Context) {
	userID := middleware.GetUserID(c)
	pluginID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	pluginService := services.NewPluginService(db.DB())
	executions, err := pluginService.ListExecutions(pluginID, userID, limit)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, executions)
}
