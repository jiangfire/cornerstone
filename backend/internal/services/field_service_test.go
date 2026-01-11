package services

import (
	"os"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupFieldTestDB 创建字段测试环境
func setupFieldTestDB(t *testing.T) *gorm.DB {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
	)
	assert.NoError(t, err)

	return db
}

// TestFieldService_CreateField 测试创建字段
func TestFieldService_CreateField(t *testing.T) {
	db := setupFieldTestDB(t)
	service := NewFieldService(db)

	// 创建测试数据
	user := models.User{Username: "testuser", Email: "test@example.com"}
	db.Create(&user)

	database := models.Database{Name: "Test DB", OwnerID: user.ID}
	db.Create(&database)

	table := models.Table{Name: "Test Table", DatabaseID: database.ID}
	result := db.Create(&table)
	assert.NoError(t, result.Error)
	assert.NotEmpty(t, table.ID, "Table ID should be generated")

	t.Run("Successful creation", func(t *testing.T) {
		req := CreateFieldRequest{
			Name: "Test Field",
			Type: "string",
		}

		field, err := service.CreateField(req, table.ID)
		assert.NoError(t, err)
		assert.NotNil(t, field)
		assert.Equal(t, "Test Field", field.Name)
		assert.Equal(t, "string", field.Type)
	})
}
