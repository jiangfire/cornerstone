package services

import (
	"os"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupOrgTestDB 创建组织测试数据库
func setupOrgTestDB(t *testing.T) *gorm.DB {
	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.User{}, &models.Organization{}, &models.OrganizationMember{})
	assert.NoError(t, err)

	return db
}

// TestOrganizationService_CreateOrganization 测试创建组织
func TestOrganizationService_CreateOrganization(t *testing.T) {
	db := setupOrgTestDB(t)
	service := NewOrganizationService(db)

	// 创建测试用户
	user := models.User{Username: "testuser", Email: "test@example.com"}
	db.Create(&user)

	t.Run("Successful creation", func(t *testing.T) {
		req := CreateOrgRequest{
			Name:        "Test Org",
			Description: "Test Description",
		}

		org, err := service.CreateOrganization(req, user.ID)
		assert.NoError(t, err)
		assert.NotNil(t, org)
		assert.Equal(t, "Test Org", org.Name)
		assert.Equal(t, user.ID, org.OwnerID)
	})

	t.Run("Duplicate name for same owner", func(t *testing.T) {
		req := CreateOrgRequest{
			Name:        "Test Org",
			Description: "Another Description",
		}

		_, err := service.CreateOrganization(req, user.ID)
		assert.Error(t, err)
	})
}
