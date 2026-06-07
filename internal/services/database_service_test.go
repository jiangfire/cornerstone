package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/testutil"
)

func setupTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupTestDBWithTokens(t, "user1", "test_user")
}

func TestDatabaseService_CreateDatabaseWithTables(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	t.Run("create database with nested tables and fields", func(t *testing.T) {
		req := CreateDBWithTablesRequest{
			Name:        "ecommerce",
			Description: "E-commerce database",
			Tables: []CreateTableWithFieldsRequest{
				{
					Name:        "orders",
					Description: "Order table",
					Fields: []struct {
						Name        string `json:"name" binding:"required"`
						Type        string `json:"type" binding:"required"`
						Description string `json:"description"`
						Required    bool   `json:"required"`
					}{
						{Name: "order_no", Type: "string", Required: true},
						{Name: "amount", Type: "number", Required: true},
						{Name: "status", Type: "string"},
					},
				},
				{
					Name:        "customers",
					Description: "Customer table",
					Fields: []struct {
						Name        string `json:"name" binding:"required"`
						Type        string `json:"type" binding:"required"`
						Description string `json:"description"`
						Required    bool   `json:"required"`
					}{
						{Name: "name", Type: "string", Required: true},
						{Name: "email", Type: "string", Required: true},
					},
				},
			},
		}

		result, err := svc.CreateDatabaseWithTables(req, "test_user")
		require.NoError(t, err)

		assert.NotNil(t, result.Database)
		assert.Equal(t, "ecommerce", result.Database.Name)
		assert.Len(t, result.Tables, 2)
		assert.Len(t, result.Fields, 5)

		assert.Equal(t, "orders", result.Tables[0].Name)
		assert.Equal(t, "customers", result.Tables[1].Name)
	})

	t.Run("duplicate database name", func(t *testing.T) {
		req := CreateDBWithTablesRequest{
			Name:   "ecommerce",
			Tables: []CreateTableWithFieldsRequest{},
		}

		_, err := svc.CreateDatabaseWithTables(req, "test_user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database name already exists")
	})

	t.Run("invalid database name", func(t *testing.T) {
		req := CreateDBWithTablesRequest{
			Name:   "a",
			Tables: []CreateTableWithFieldsRequest{},
		}

		_, err := svc.CreateDatabaseWithTables(req, "test_user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database name validation failed")
	})
}

func TestDatabaseService_CRUD(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	t.Run("create and get database", func(t *testing.T) {
		database, err := svc.CreateDatabase(CreateDBRequest{
			Name:        "TestDB",
			Description: "Test database",
		}, "user1")
		require.NoError(t, err)
		assert.Equal(t, "TestDB", database.Name)

		result, err := svc.GetDatabase(database.ID, "user1")
		require.NoError(t, err)
		assert.Equal(t, "TestDB", result.Name)
	})

	t.Run("list databases", func(t *testing.T) {
		svc.CreateDatabase(CreateDBRequest{Name: "DB1"}, "user1")
		svc.CreateDatabase(CreateDBRequest{Name: "DB2"}, "user1")

		databases, err := svc.ListDatabases("user1")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(databases), 2)
	})

	t.Run("update database", func(t *testing.T) {
		database, _ := svc.CreateDatabase(CreateDBRequest{Name: "UpdateDB"}, "user1")

		updated, err := svc.UpdateDatabase(database.ID, UpdateDBRequest{
			Name:        "UpdatedDB",
			Description: "Updated description",
		}, "user1")
		require.NoError(t, err)
		assert.Equal(t, "UpdatedDB", updated.Name)
	})

	t.Run("delete database", func(t *testing.T) {
		database, _ := svc.CreateDatabase(CreateDBRequest{Name: "DeleteDB"}, "user1")

		err := svc.DeleteDatabase(database.ID, "user1")
		require.NoError(t, err)

		_, err = svc.GetDatabase(database.ID, "user1")
		assert.Error(t, err)
	})
}
