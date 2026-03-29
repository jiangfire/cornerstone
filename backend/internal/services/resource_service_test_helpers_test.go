package services

import (
	"os"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupResourceTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	_ = os.Setenv("JWT_SECRET", "test-secret-key-for-resource-services")

	dbFile := t.TempDir() + "\\resource-service-test.db"
	db, err := gorm.Open(sqlite.Open(dbFile), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = sqlDB.Close()
	})

	require.NoError(t, db.AutoMigrate(
		&models.User{},
		&models.Database{},
		&models.DatabaseAccess{},
		&models.Table{},
		&models.Field{},
		&models.Record{},
		&models.FieldPermission{},
		&models.File{},
		&models.ActivityLog{},
		&models.Plugin{},
		&models.PluginBinding{},
		&models.PluginExecution{},
		&models.AppSettings{},
	))

	return db
}

func createResourceUser(t *testing.T, db *gorm.DB, username string) models.User {
	t.Helper()

	user := models.User{
		Username: username,
		Email:    username + "@example.com",
		Password: "hashed",
	}
	require.NoError(t, db.Create(&user).Error)
	return user
}

func createResourceDatabase(t *testing.T, db *gorm.DB, ownerID, name string) models.Database {
	t.Helper()

	database := models.Database{
		Name:       name,
		OwnerID:    ownerID,
		IsPersonal: true,
	}
	require.NoError(t, db.Create(&database).Error)
	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     ownerID,
		DatabaseID: database.ID,
		Role:       "owner",
	}).Error)
	return database
}

func grantResourceDatabaseAccess(t *testing.T, db *gorm.DB, databaseID, userID, role string) {
	t.Helper()

	require.NoError(t, db.Create(&models.DatabaseAccess{
		UserID:     userID,
		DatabaseID: databaseID,
		Role:       role,
	}).Error)
}

func createResourceTable(t *testing.T, db *gorm.DB, databaseID, name string) models.Table {
	t.Helper()

	table := models.Table{
		DatabaseID: databaseID,
		Name:       name,
	}
	require.NoError(t, db.Create(&table).Error)
	return table
}

func createResourceField(t *testing.T, db *gorm.DB, tableID, name, fieldType string, required bool, options string) models.Field {
	t.Helper()

	field := models.Field{
		TableID:  tableID,
		Name:     name,
		Type:     fieldType,
		Required: required,
		Options:  options,
	}
	require.NoError(t, db.Create(&field).Error)
	return field
}

func createResourceRecord(t *testing.T, db *gorm.DB, tableID, userID, data string) models.Record {
	t.Helper()

	record := models.Record{
		TableID:   tableID,
		Data:      data,
		CreatedBy: userID,
		UpdatedBy: userID,
		Version:   1,
	}
	require.NoError(t, db.Create(&record).Error)
	return record
}
