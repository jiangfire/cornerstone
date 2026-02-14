package services

import (
	"os"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// setupFieldTestDB 创建字段测试环境
func setupFieldTestDB(t *testing.T) *gorm.DB {
	os.Setenv("JWT_SECRET", "test-secret-key-for-testing-only")

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.Table{},
		&models.Field{},
	)
	require.NoError(t, err)

	return db
}

// TestFieldService_CreateField 测试创建字段
func TestFieldService_CreateField(t *testing.T) {
	db := setupFieldTestDB(t)
	service := NewFieldService(db)

	// 创建测试数据
	user := models.User{Username: "testuser", Email: "test@example.com", Password: "hashed_password"}
	require.NoError(t, db.Create(&user).Error)
	require.NotEmpty(t, user.ID)

	database := models.Database{Name: "Test DB", OwnerID: user.ID}
	require.NoError(t, db.Create(&database).Error)
	require.NotEmpty(t, database.ID)

	table := models.Table{Name: "Test Table", DatabaseID: database.ID}
	require.NoError(t, db.Create(&table).Error)
	require.NotEmpty(t, table.ID, "Table ID should be generated")

	t.Run("Successful creation", func(t *testing.T) {
		req := CreateFieldRequest{
			Name: "Test Field",
			Type: "string",
		}

		field, err := service.CreateField(req, table.ID)
		require.NoError(t, err)
		require.NotNil(t, field)
		require.Equal(t, "Test Field", field.Name)
		require.Equal(t, "string", field.Type)
	})
}
