package services

import (
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupOrgTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), newServiceTestGormConfig())
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{}))

	return db
}

func createOrgTestUser(t *testing.T, db *gorm.DB, username string) models.User {
	t.Helper()

	user := models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func createOwnedOrganization(t *testing.T, service *OrganizationService, ownerID, name string) *models.Organization {
	t.Helper()

	org, err := service.CreateOrganization(CreateOrgRequest{
		Name:        name,
		Description: "test organization",
	}, ownerID)
	require.NoError(t, err)
	return org
}

func TestOrganizationService_CreateOrganizationCreatesOwnerMembership(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)
	owner := createOrgTestUser(t, db, "org_creator")

	org, err := service.CreateOrganization(CreateOrgRequest{
		Name:        "Test Org",
		Description: "Test Description",
	}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, owner.ID, org.OwnerID)

	var member models.OrganizationMember
	require.NoError(t, db.Where("organization_id = ? AND user_id = ?", org.ID, owner.ID).First(&member).Error)
	require.Equal(t, "owner", member.Role)
}

func TestOrganizationService_DeleteOrganizationSoftDeleteAndAllowsRecreate(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)
	owner := createOrgTestUser(t, db, "org_owner_delete")
	admin := createOrgTestUser(t, db, "org_admin_delete")

	org := createOwnedOrganization(t, service, owner.ID, "Archive Org")
	require.NoError(t, service.AddMember(org.ID, AddMemberRequest{
		UserID: admin.ID,
		Role:   "admin",
	}, owner.ID))

	err := service.DeleteOrganization(org.ID, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "只有组织所有者可以删除组织")

	require.NoError(t, service.DeleteOrganization(org.ID, owner.ID))

	var stored models.Organization
	require.NoError(t, db.Where("id = ?", org.ID).First(&stored).Error)
	require.NotNil(t, stored.DeletedAt)

	listed, err := service.ListOrganizations(owner.ID)
	require.NoError(t, err)
	require.Empty(t, listed)

	_, err = service.GetOrganization(org.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该组织")

	err = service.AddMember(org.ID, AddMemberRequest{
		UserID: admin.ID,
		Role:   "member",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该组织")

	recreated, err := service.CreateOrganization(CreateOrgRequest{Name: "Archive Org"}, owner.ID)
	require.NoError(t, err)
	require.NotEqual(t, org.ID, recreated.ID)
}

func TestOrganizationService_AddMemberRoleRules(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)

	owner := createOrgTestUser(t, db, "org_owner_add")
	admin := createOrgTestUser(t, db, "org_admin_add")
	member := createOrgTestUser(t, db, "org_member_add")
	target := createOrgTestUser(t, db, "org_target_add")

	org := createOwnedOrganization(t, service, owner.ID, "Members Org")

	require.NoError(t, service.AddMember(org.ID, AddMemberRequest{
		UserID: admin.ID,
		Role:   "admin",
	}, owner.ID))

	err := service.AddMember(org.ID, AddMemberRequest{
		UserID: target.ID,
		Role:   "owner",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的组织角色")

	err = service.AddMember(org.ID, AddMemberRequest{
		UserID: target.ID,
		Role:   "owner",
	}, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的组织角色")

	require.NoError(t, service.AddMember(org.ID, AddMemberRequest{
		UserID: member.ID,
		Role:   "member",
	}, admin.ID))

	err = service.AddMember(org.ID, AddMemberRequest{
		UserID: target.ID,
		Role:   "member",
	}, member.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "只有组织所有者和管理员可以添加成员")
}

func TestOrganizationService_UpdateMemberRoleRules(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)

	owner := createOrgTestUser(t, db, "org_owner_update")
	admin := createOrgTestUser(t, db, "org_admin_update")
	member := createOrgTestUser(t, db, "org_member_update")

	org := createOwnedOrganization(t, service, owner.ID, "Role Org")
	require.NoError(t, service.AddMember(org.ID, AddMemberRequest{
		UserID: admin.ID,
		Role:   "admin",
	}, owner.ID))
	require.NoError(t, service.AddMember(org.ID, AddMemberRequest{
		UserID: member.ID,
		Role:   "member",
	}, owner.ID))

	var current models.OrganizationMember
	require.NoError(t, db.Where("organization_id = ? AND user_id = ?", org.ID, member.ID).First(&current).Error)

	require.NoError(t, service.UpdateMemberRole(org.ID, current.ID, UpdateMemberRequest{
		Role: "admin",
	}, owner.ID))

	var updated models.OrganizationMember
	require.NoError(t, db.Where("organization_id = ? AND user_id = ?", org.ID, member.ID).First(&updated).Error)
	require.Equal(t, "admin", updated.Role)

	err := service.UpdateMemberRole(org.ID, updated.ID, UpdateMemberRequest{
		Role: "owner",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的组织角色")

	err = service.UpdateMemberRole(org.ID, updated.ID, UpdateMemberRequest{
		Role: "member",
	}, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "只有组织所有者可以修改成员角色")

	var ownerMember models.OrganizationMember
	require.NoError(t, db.Where("organization_id = ? AND user_id = ?", org.ID, owner.ID).First(&ownerMember).Error)
	err = service.UpdateMemberRole(org.ID, ownerMember.ID, UpdateMemberRequest{
		Role: "member",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "不能修改组织所有者的角色")
}

func TestOrganizationService_UpdateOrganizationRejectsDuplicateActiveName(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)
	owner := createOrgTestUser(t, db, "org_owner_duplicate")

	first := createOwnedOrganization(t, service, owner.ID, "Alpha Org")
	second := createOwnedOrganization(t, service, owner.ID, "Beta Org")

	_, err := service.UpdateOrganization(second.ID, UpdateOrgRequest{
		Name:        "Alpha Org",
		Description: "renamed",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "您已创建过同名组织")

	current, err := service.GetOrganization(second.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "Beta Org", current.Name)
	require.Equal(t, first.OwnerID, current.OwnerID)
}

func TestOrganizationService_ListOrganizationsExcludesDeletedAndOrdersNewestFirst(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)
	owner := createOrgTestUser(t, db, "org_owner_list")

	oldest := createOwnedOrganization(t, service, owner.ID, "Oldest Org")
	middle := createOwnedOrganization(t, service, owner.ID, "Middle Org")
	newest := createOwnedOrganization(t, service, owner.ID, "Newest Org")

	require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", oldest.ID).Update("created_at", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).Error)
	require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", middle.ID).Update("created_at", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)).Error)
	require.NoError(t, db.Model(&models.Organization{}).Where("id = ?", newest.ID).Update("created_at", time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)).Error)

	require.NoError(t, service.DeleteOrganization(middle.ID, owner.ID))

	listed, err := service.ListOrganizations(owner.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, newest.ID, listed[0].ID)
	require.Equal(t, oldest.ID, listed[1].ID)
}
