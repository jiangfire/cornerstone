package handlers

import (
	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/middleware"
	"github.com/jiangfire/cornerstone/backend/internal/services"
	"github.com/jiangfire/cornerstone/backend/internal/types"
	"github.com/jiangfire/cornerstone/backend/pkg/db"
)

// CreateOrganization 创建组织
func CreateOrganization(c *gin.Context) {
	userID := middleware.GetUserID(c)

	var req services.CreateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	orgService := services.NewOrganizationService(db.DB())
	org, err := orgService.CreateOrganization(req, userID)
	if err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":          org.ID,
		"name":        org.Name,
		"description": org.Description,
		"owner_id":    org.OwnerID,
		"created_at":  org.CreatedAt,
	})
}

// ListOrganizations 获取组织列表
func ListOrganizations(c *gin.Context) {
	userID := middleware.GetUserID(c)

	orgService := services.NewOrganizationService(db.DB())
	orgs, err := orgService.ListOrganizations(userID)
	if err != nil {
		types.Error(c, 500, err.Error())
		return
	}

	types.Success(c, gin.H{
		"organizations": orgs,
		"total":         len(orgs),
	})
}

// GetOrganization 获取组织详情
func GetOrganization(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")

	orgService := services.NewOrganizationService(db.DB())
	org, err := orgService.GetOrganization(orgID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, org)
}

// UpdateOrganization 更新组织信息
func UpdateOrganization(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")

	var req services.UpdateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	orgService := services.NewOrganizationService(db.DB())
	org, err := orgService.UpdateOrganization(orgID, req, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"id":          org.ID,
		"name":        org.Name,
		"description": org.Description,
		"updated_at":  org.UpdatedAt,
	})
}

// DeleteOrganization 删除组织
func DeleteOrganization(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")

	orgService := services.NewOrganizationService(db.DB())
	if err := orgService.DeleteOrganization(orgID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "组织已删除",
	})
}

// ListOrganizationMembers 获取组织成员列表
func ListOrganizationMembers(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")

	orgService := services.NewOrganizationService(db.DB())
	members, err := orgService.ListMembers(orgID, userID)
	if err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"members": members,
		"total":   len(members),
	})
}

// AddOrganizationMember 添加组织成员
func AddOrganizationMember(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")

	var req services.AddMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	orgService := services.NewOrganizationService(db.DB())
	if err := orgService.AddMember(orgID, req, userID); err != nil {
		types.Error(c, 400, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "成员添加成功",
	})
}

// RemoveOrganizationMember 移除组织成员
func RemoveOrganizationMember(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")
	memberID := c.Param("member_id")

	orgService := services.NewOrganizationService(db.DB())
	if err := orgService.RemoveMember(orgID, memberID, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "成员已移除",
	})
}

// UpdateOrganizationMemberRole 更新成员角色
func UpdateOrganizationMemberRole(c *gin.Context) {
	userID := middleware.GetUserID(c)
	orgID := c.Param("id")
	memberID := c.Param("member_id")

	var req services.UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		types.Error(c, 400, "参数错误: "+err.Error())
		return
	}

	orgService := services.NewOrganizationService(db.DB())
	if err := orgService.UpdateMemberRole(orgID, memberID, req, userID); err != nil {
		types.Error(c, 403, err.Error())
		return
	}

	types.Success(c, gin.H{
		"message": "角色已更新",
	})
}
