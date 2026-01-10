package services

import (
	"errors"
	"fmt"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"gorm.io/gorm"
)

// OrganizationService 组织服务
type OrganizationService struct {
	db *gorm.DB
}

// NewOrganizationService 创建组织服务实例
func NewOrganizationService(db *gorm.DB) *OrganizationService {
	return &OrganizationService{db: db}
}

// CreateOrgRequest 创建组织请求
type CreateOrgRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"max=500"`
}

// UpdateOrgRequest 更新组织请求
type UpdateOrgRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description" binding:"max=500"`
}

// AddMemberRequest 添加成员请求
type AddMemberRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"required,oneof=owner admin member"`
}

// UpdateMemberRequest 更新成员角色请求
type UpdateMemberRequest struct {
	Role string `json:"role" binding:"required,oneof=owner admin member"`
}

// OrgResponse 组织响应（包含用户角色信息）
type OrgResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     string `json:"owner_id"`
	Role        string `json:"role"` // 当前用户在该组织的角色
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// CreateOrganization 创建组织
func (s *OrganizationService) CreateOrganization(req CreateOrgRequest, ownerID string) (*models.Organization, error) {
	// 1. 检查组织名称是否已被该用户创建
	var existingOrg models.Organization
	err := s.db.Where("name = ? AND owner_id = ?", req.Name, ownerID).First(&existingOrg).Error
	if err == nil {
		return nil, errors.New("您已创建过同名组织")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 2. 开始事务
	tx := s.db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 3. 创建组织
	org := models.Organization{
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     ownerID,
	}

	if err := tx.Create(&org).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("创建组织失败: %w", err)
	}

	// 4. 自动将创建者添加为组织所有者
	member := models.OrganizationMember{
		OrganizationID: org.ID,
		UserID:         ownerID,
		Role:           "owner",
	}

	if err := tx.Create(&member).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("添加组织所有者失败: %w", err)
	}

	// 5. 提交事务
	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	return &org, nil
}

// ListOrganizations 获取用户所属组织列表
func (s *OrganizationService) ListOrganizations(userID string) ([]OrgResponse, error) {
	// 查询用户所属的组织
	var results []struct {
		OrganizationID   string
		Name             string
		Description      string
		OwnerID          string
		OrganizationRole string
		CreatedAt        string
		UpdatedAt        string
	}

	err := s.db.Raw(`
		SELECT
			o.id as organization_id,
			o.name,
			o.description,
			o.owner_id,
			om.role as organization_role,
			o.created_at,
			o.updated_at
		FROM organizations o
		INNER JOIN organization_members om ON o.id = om.organization_id
		WHERE om.user_id = ?
		ORDER BY o.created_at DESC
	`, userID).Scan(&results).Error

	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 转换为响应格式
	orgs := make([]OrgResponse, len(results))
	for i, r := range results {
		orgs[i] = OrgResponse{
			ID:          r.OrganizationID,
			Name:        r.Name,
			Description: r.Description,
			OwnerID:     r.OwnerID,
			Role:        r.OrganizationRole,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
		}
	}

	return orgs, nil
}

// GetOrganization 获取组织详情
func (s *OrganizationService) GetOrganization(orgID, userID string) (*OrgResponse, error) {
	// 检查用户是否是该组织成员
	var member models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ?", orgID, userID).First(&member).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("无权访问该组织")
		}
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 获取组织信息
	var org models.Organization
	err = s.db.Where("id = ?", orgID).First(&org).Error
	if err != nil {
		return nil, fmt.Errorf("组织不存在: %w", err)
	}

	return &OrgResponse{
		ID:          org.ID,
		Name:        org.Name,
		Description: org.Description,
		OwnerID:     org.OwnerID,
		Role:        member.Role,
		CreatedAt:   org.CreatedAt.Format("2006-01-02 15:04:05"),
		UpdatedAt:   org.UpdatedAt.Format("2006-01-02 15:04:05"),
	}, nil
}

// UpdateOrganization 更新组织信息
func (s *OrganizationService) UpdateOrganization(orgID string, req UpdateOrgRequest, userID string) (*models.Organization, error) {
	// 1. 检查用户是否是组织所有者或管理员
	var member models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ?", orgID, userID).First(&member).Error
	if err != nil {
		return nil, errors.New("无权访问该组织")
	}

	if member.Role != "owner" && member.Role != "admin" {
		return nil, errors.New("只有组织所有者和管理员可以修改组织信息")
	}

	// 2. 更新组织信息
	var org models.Organization
	err = s.db.Where("id = ?", orgID).First(&org).Error
	if err != nil {
		return nil, fmt.Errorf("组织不存在: %w", err)
	}

	org.Name = req.Name
	org.Description = req.Description

	if err := s.db.Save(&org).Error; err != nil {
		return nil, fmt.Errorf("更新组织失败: %w", err)
	}

	return &org, nil
}

// DeleteOrganization 删除组织（软删除）
func (s *OrganizationService) DeleteOrganization(orgID, userID string) error {
	// 1. 检查用户是否是组织所有者
	var member models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ? AND role = ?", orgID, userID, "owner").First(&member).Error
	if err != nil {
		return errors.New("只有组织所有者可以删除组织")
	}

	// 2. 软删除组织
	if err := s.db.Delete(&models.Organization{}, orgID).Error; err != nil {
		return fmt.Errorf("删除组织失败: %w", err)
	}

	return nil
}

// AddMember 添加组织成员
func (s *OrganizationService) AddMember(orgID string, req AddMemberRequest, operatorID string) error {
	// 1. 检查操作者权限
	var operator models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ?", orgID, operatorID).First(&operator).Error
	if err != nil {
		return errors.New("无权访问该组织")
	}

	if operator.Role != "owner" && operator.Role != "admin" {
		return errors.New("只有组织所有者和管理员可以添加成员")
	}

	// 2. 检查被添加用户是否存在
	var user models.User
	err = s.db.Where("id = ?", req.UserID).First(&user).Error
	if err != nil {
		return errors.New("用户不存在")
	}

	// 3. 检查用户是否已是成员
	var existingMember models.OrganizationMember
	err = s.db.Where("organization_id = ? AND user_id = ?", orgID, req.UserID).First(&existingMember).Error
	if err == nil {
		return errors.New("该用户已是组织成员")
	}

	// 4. 添加成员
	member := models.OrganizationMember{
		OrganizationID: orgID,
		UserID:         req.UserID,
		Role:           req.Role,
	}

	if err := s.db.Create(&member).Error; err != nil {
		return fmt.Errorf("添加成员失败: %w", err)
	}

	return nil
}

// ListMembers 获取组织成员列表
func (s *OrganizationService) ListMembers(orgID, userID string) ([]interface{}, error) {
	// 1. 检查用户是否是组织成员
	var member models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ?", orgID, userID).First(&member).Error
	if err != nil {
		return nil, errors.New("无权访问该组织")
	}

	// 2. 查询成员列表
	var members []struct {
		ID             string `json:"id"`
		OrganizationID string `json:"organization_id"`
		UserID         string `json:"user_id"`
		Username       string `json:"username"`
		Email          string `json:"email"`
		Role           string `json:"role"`
		JoinedAt       string `json:"joined_at"`
	}

	err = s.db.Raw(`
		SELECT
			om.id,
			om.organization_id,
			om.user_id,
			u.username,
			u.email,
			om.role,
			om.joined_at
		FROM organization_members om
		INNER JOIN users u ON om.user_id = u.id
		WHERE om.organization_id = ?
		ORDER BY om.joined_at ASC
	`, orgID).Scan(&members).Error

	if err != nil {
		return nil, fmt.Errorf("数据库查询失败: %w", err)
	}

	// 转换为 interface{} 切片
	result := make([]interface{}, len(members))
	for i, m := range members {
		result[i] = m
	}

	return result, nil
}

// RemoveMember 移除组织成员
func (s *OrganizationService) RemoveMember(orgID, memberID, operatorID string) error {
	// 1. 检查操作者权限
	var operator models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ?", orgID, operatorID).First(&operator).Error
	if err != nil {
		return errors.New("无权访问该组织")
	}

	// 2. 不能移除所有者
	var targetMember models.OrganizationMember
	err = s.db.Where("id = ? AND organization_id = ?", memberID, orgID).First(&targetMember).Error
	if err != nil {
		return errors.New("成员不存在")
	}

	if targetMember.Role == "owner" {
		return errors.New("不能移除组织所有者")
	}

	// 3. 检查权限（只有所有者和管理员可以移除成员）
	if operator.Role != "owner" && operator.Role != "admin" {
		return errors.New("只有组织所有者和管理员可以移除成员")
	}

	// 4. 移除成员
	if err := s.db.Delete(&targetMember).Error; err != nil {
		return fmt.Errorf("移除成员失败: %w", err)
	}

	return nil
}

// UpdateMemberRole 更新成员角色
func (s *OrganizationService) UpdateMemberRole(orgID, memberID string, req UpdateMemberRequest, operatorID string) error {
	// 1. 检查操作者权限（只有所有者可以修改角色）
	var operator models.OrganizationMember
	err := s.db.Where("organization_id = ? AND user_id = ? AND role = ?", orgID, operatorID, "owner").First(&operator).Error
	if err != nil {
		return errors.New("只有组织所有者可以修改成员角色")
	}

	// 2. 获取目标成员
	var targetMember models.OrganizationMember
	err = s.db.Where("id = ? AND organization_id = ?", memberID, orgID).First(&targetMember).Error
	if err != nil {
		return errors.New("成员不存在")
	}

	// 3. 不能修改所有者角色
	if targetMember.Role == "owner" {
		return errors.New("不能修改组织所有者的角色")
	}

	// 4. 更新角色
	targetMember.Role = req.Role
	if err := s.db.Save(&targetMember).Error; err != nil {
		return fmt.Errorf("更新角色失败: %w", err)
	}

	return nil
}
