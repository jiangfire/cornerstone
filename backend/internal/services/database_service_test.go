package services

import (
	"os"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupDatabaseTestDB 创建数据库测试环境
func setupDatabaseTestDB(t *testing.T) *gorm.DB {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(
		&models.User{},
		&models.Organization{},
		&models.Database{},
		&models.DatabaseAccess{},
	)
	assert.NoError(t, err)

	return db
}

// TestDatabaseService_CreateDatabase 测试创建数据库
func TestDatabaseService_CreateDatabase(t *testing.T) {
	db := setupDatabaseTestDB(t)
	service := NewDatabaseService(db)

	// 创建测试用户和组织
	user := models.User{Username: "testuser", Email: "test@example.com"}
	db.Create(&user)

	org := models.Organization{Name: "Test Org", OwnerID: user.ID}
	db.Create(&org)

	t.Run("Successful creation", func(t *testing.T) {
		req := CreateDBRequest{
			Name:        "Test DB",
			Description: "Test Description",
		}

		database, err := service.CreateDatabase(req, user.ID)
		assert.NoError(t, err)
		assert.NotNil(t, database)
		assert.Equal(t, "Test DB", database.Name)
	})
}
