package services

import (
	"testing"
	"time"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupDatabaseTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	t.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), newServiceTestGormConfig())
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.Database{},
		&models.DatabaseAccess{},
	))

	return db
}

func createDatabaseTestUser(t *testing.T, db *gorm.DB, username string) models.User {
	t.Helper()

	user := models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func createOwnedDatabase(t *testing.T, service *DatabaseService, ownerID, name string) *models.Database {
	t.Helper()

	database, err := service.CreateDatabase(CreateDBRequest{
		Name:        name,
		Description: "test database",
		IsPersonal:  true,
	}, ownerID)
	require.NoError(t, err)
	return database
}

func TestDatabaseService_CreateDatabaseSanitizesAndGrantsOwnerAccess(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)
	owner := createDatabaseTestUser(t, db, "creator")

	database, err := service.CreateDatabase(CreateDBRequest{
		Name:        `  <Test "DB">  `,
		Description: `  "quoted" <desc>  `,
		IsPublic:    true,
		IsPersonal:  true,
	}, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "Test DB", database.Name)
	require.Equal(t, "quoted desc", database.Description)
	require.True(t, database.IsPublic)
	require.True(t, database.IsPersonal)

	var access models.DatabaseAccess
	require.NoError(t, db.Where("database_id = ? AND user_id = ?", database.ID, owner.ID).First(&access).Error)
	require.Equal(t, "owner", access.Role)
}

func TestDatabaseService_DeleteDatabaseSoftDeleteAndAllowsRecreate(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)
	owner := createDatabaseTestUser(t, db, "owner_soft_delete")
	admin := createDatabaseTestUser(t, db, "admin_soft_delete")

	database := createOwnedDatabase(t, service, owner.ID, "Archive DB")
	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: admin.ID,
		Role:   "admin",
	}, owner.ID))

	err := service.DeleteDatabase(database.ID, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "只有所有者可以删除数据库")

	require.NoError(t, service.DeleteDatabase(database.ID, owner.ID))

	var stored models.Database
	require.NoError(t, db.Where("id = ?", database.ID).First(&stored).Error)
	require.NotNil(t, stored.DeletedAt)

	listed, err := service.ListDatabases(owner.ID)
	require.NoError(t, err)
	require.Empty(t, listed)

	_, err = service.GetDatabase(database.ID, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	err = service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: admin.ID,
		Role:   "viewer",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	recreated, err := service.CreateDatabase(CreateDBRequest{Name: "Archive DB"}, owner.ID)
	require.NoError(t, err)
	require.NotEqual(t, database.ID, recreated.ID)
}

func TestDatabaseService_ShareDatabaseRoleRules(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)

	owner := createDatabaseTestUser(t, db, "owner_share")
	admin := createDatabaseTestUser(t, db, "admin_share")
	viewer := createDatabaseTestUser(t, db, "viewer_share")
	editor := createDatabaseTestUser(t, db, "editor_share")

	database := createOwnedDatabase(t, service, owner.ID, "Share DB")

	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: admin.ID,
		Role:   "admin",
	}, owner.ID))

	err := service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: viewer.ID,
		Role:   "owner",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的数据库角色")

	err = service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: editor.ID,
		Role:   "owner",
	}, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的数据库角色")

	err = service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: viewer.ID,
		Role:   "viewer",
	}, viewer.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: viewer.ID,
		Role:   "viewer",
	}, admin.ID))

	err = service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: viewer.ID,
		Role:   "viewer",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "该用户已有访问权限")
}

func TestDatabaseService_UpdateDatabaseUserRoleRules(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)

	owner := createDatabaseTestUser(t, db, "owner_update_role")
	admin := createDatabaseTestUser(t, db, "admin_update_role")
	editor := createDatabaseTestUser(t, db, "editor_update_role")

	database := createOwnedDatabase(t, service, owner.ID, "Role DB")
	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: admin.ID,
		Role:   "admin",
	}, owner.ID))
	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{
		UserID: editor.ID,
		Role:   "editor",
	}, owner.ID))

	require.NoError(t, service.UpdateDatabaseUserRole(database.ID, editor.ID, UpdateDBUserRoleRequest{
		Role: "viewer",
	}, owner.ID))

	var editorAccess models.DatabaseAccess
	require.NoError(t, db.Where("database_id = ? AND user_id = ?", database.ID, editor.ID).First(&editorAccess).Error)
	require.Equal(t, "viewer", editorAccess.Role)

	err := service.UpdateDatabaseUserRole(database.ID, editor.ID, UpdateDBUserRoleRequest{
		Role: "owner",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无效的数据库角色")

	err = service.UpdateDatabaseUserRole(database.ID, editor.ID, UpdateDBUserRoleRequest{
		Role: "admin",
	}, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "只有数据库所有者可以修改用户角色")

	err = service.UpdateDatabaseUserRole(database.ID, owner.ID, UpdateDBUserRoleRequest{
		Role: "viewer",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "不能修改数据库所有者的角色")
}

func TestDatabaseService_UpdateDatabaseRejectsDuplicateActiveName(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)
	owner := createDatabaseTestUser(t, db, "owner_duplicate_update")

	first := createOwnedDatabase(t, service, owner.ID, "Alpha DB")
	second := createOwnedDatabase(t, service, owner.ID, "Beta DB")

	_, err := service.UpdateDatabase(second.ID, UpdateDBRequest{
		Name:        "Alpha DB",
		Description: "renamed",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "您已创建过同名数据库")

	current, err := service.GetDatabase(second.ID, owner.ID)
	require.NoError(t, err)
	require.Equal(t, "Beta DB", current.Name)
	require.Equal(t, first.OwnerID, current.OwnerID)
}

func TestDatabaseService_ListDatabasesExcludesDeletedAndOrdersNewestFirst(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)
	owner := createDatabaseTestUser(t, db, "owner_list")

	oldest := createOwnedDatabase(t, service, owner.ID, "Oldest DB")
	middle := createOwnedDatabase(t, service, owner.ID, "Middle DB")
	newest := createOwnedDatabase(t, service, owner.ID, "Newest DB")

	require.NoError(t, db.Model(&models.Database{}).Where("id = ?", oldest.ID).Update("created_at", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)).Error)
	require.NoError(t, db.Model(&models.Database{}).Where("id = ?", middle.ID).Update("created_at", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)).Error)
	require.NoError(t, db.Model(&models.Database{}).Where("id = ?", newest.ID).Update("created_at", time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC)).Error)

	require.NoError(t, service.DeleteDatabase(middle.ID, owner.ID))

	listed, err := service.ListDatabases(owner.ID)
	require.NoError(t, err)
	require.Len(t, listed, 2)
	require.Equal(t, newest.ID, listed[0].ID)
	require.Equal(t, oldest.ID, listed[1].ID)
}

func TestDatabaseService_ListUsersAndRemoveUserRespectRoleBoundaries(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)

	owner := createDatabaseTestUser(t, db, "owner_members")
	admin := createDatabaseTestUser(t, db, "admin_members")
	editor := createDatabaseTestUser(t, db, "editor_members")
	viewer := createDatabaseTestUser(t, db, "viewer_members")
	outsider := createDatabaseTestUser(t, db, "outsider_members")

	database := createOwnedDatabase(t, service, owner.ID, "Members DB")
	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{UserID: admin.ID, Role: "admin"}, owner.ID))
	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{UserID: editor.ID, Role: "editor"}, owner.ID))
	require.NoError(t, service.ShareDatabase(database.ID, ShareDBRequest{UserID: viewer.ID, Role: "viewer"}, owner.ID))

	listedByViewer, err := service.ListDatabaseUsers(database.ID, viewer.ID)
	require.NoError(t, err)
	require.Len(t, listedByViewer, 4)

	_, err = service.ListDatabaseUsers(database.ID, outsider.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "无权访问该数据库")

	err = service.RemoveDatabaseUser(database.ID, owner.ID, admin.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "不能移除数据库所有者")

	err = service.RemoveDatabaseUser(database.ID, editor.ID, viewer.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "只有所有者和管理员可以移除用户")

	err = service.RemoveDatabaseUser(database.ID, editor.ID, admin.ID)
	require.NoError(t, err)

	var editorAccess models.DatabaseAccess
	err = db.Where("database_id = ? AND user_id = ?", database.ID, editor.ID).First(&editorAccess).Error
	require.Error(t, err)

	err = service.RemoveDatabaseUser(database.ID, viewer.ID, owner.ID)
	require.NoError(t, err)

	var viewerAccess models.DatabaseAccess
	err = db.Where("database_id = ? AND user_id = ?", database.ID, viewer.ID).First(&viewerAccess).Error
	require.Error(t, err)
}
