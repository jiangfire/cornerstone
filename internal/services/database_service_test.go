package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/testutil"
	"github.com/jiangfire/cornerstone/pkg/dto"
)

func setupTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupTestDBWithTokens(t, "user1", "test_user")
}

func TestDatabaseService_ImportYAML(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	yamlContent := `
name: "yaml-import-test"
description: "Imported via YAML"
tables:
  - name: "users"
    description: "User accounts"
    fields:
      - name: "title"
        type: "string"
        description: "The record title"
        required: false
      - name: "status"
        type: "string"
        description: "Current status"
        required: false
`

	result, err := svc.ImportYAML([]byte(yamlContent), "user1")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "yaml-import-test", result.Database.Name)
	assert.Len(t, result.Tables, 1)
	assert.Equal(t, "users", result.Tables[0].Name)
	assert.Len(t, result.Fields, 2)
}

func TestDatabaseService_ImportYAML_InvalidFormat(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	invalidYAML := `this: [is: broken: yaml`

	_, err := svc.ImportYAML([]byte(invalidYAML), "user1")
	require.Error(t, err)
}

func TestDatabaseService_ImportYAML_MissingName(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	missingName := `
description: "No name field"
tables:
  - name: "items"
`

	_, err := svc.ImportYAML([]byte(missingName), "user1")
	require.Error(t, err)
}

func TestDatabaseService_CreateDatabaseWithTables(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	t.Run("create database with nested tables and fields", func(t *testing.T) {
		req := dto.DatabaseBulkCreateRequest{
			Name:        "ecommerce",
			Description: "E-commerce database",
			Tables: []dto.BulkCreateTable{
				{
					Name:        "orders",
					Description: "Order table",
					Fields: []dto.BulkCreateTableField{
						{Name: "order_no", Type: "string", Required: true},
						{Name: "amount", Type: "number", Required: true},
						{Name: "status", Type: "string"},
					},
				},
				{
					Name:        "customers",
					Description: "Customer table",
					Fields: []dto.BulkCreateTableField{
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
		req := dto.DatabaseBulkCreateRequest{
			Name:   "ecommerce",
			Tables: []dto.BulkCreateTable{},
		}

		_, err := svc.CreateDatabaseWithTables(req, "test_user")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "database name already exists")
	})

	t.Run("invalid database name", func(t *testing.T) {
		req := dto.DatabaseBulkCreateRequest{
			Name:   "a",
			Tables: []dto.BulkCreateTable{},
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
		database, err := svc.CreateDatabase(dto.DatabaseCreateRequest{
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
		svc.CreateDatabase(dto.DatabaseCreateRequest{Name: "DB1"}, "user1")
		svc.CreateDatabase(dto.DatabaseCreateRequest{Name: "DB2"}, "user1")

		databases, err := svc.ListDatabases("user1")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(databases), 2)
	})

	t.Run("update database", func(t *testing.T) {
		database, _ := svc.CreateDatabase(dto.DatabaseCreateRequest{Name: "UpdateDB"}, "user1")

		updated, err := svc.UpdateDatabase(database.ID, dto.DatabaseUpdateRequest{
			Name:        "UpdatedDB",
			Description: "Updated description",
		}, "user1")
		require.NoError(t, err)
		assert.Equal(t, "UpdatedDB", updated.Name)
	})

	t.Run("delete database", func(t *testing.T) {
		database, _ := svc.CreateDatabase(dto.DatabaseCreateRequest{Name: "DeleteDB"}, "user1")

		err := svc.DeleteDatabase(database.ID, "user1")
		require.NoError(t, err)

		_, err = svc.GetDatabase(database.ID, "user1")
		assert.Error(t, err)
	})
}
