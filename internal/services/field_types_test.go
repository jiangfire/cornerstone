package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/models"
)

func setupFieldTestEnv(t *testing.T) (*FieldService, *models.Table, *models.Token) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	return svc, table, master
}

func TestFieldService_CreateField_AcceptsValidTypes(t *testing.T) {
	svc, table, master := setupFieldTestEnv(t)

	validTypes := []string{
		"string", "text", "number", "boolean",
		"date", "datetime", "file", "json", "list",
	}

	for _, fieldType := range validTypes {
		field, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "field_" + fieldType,
			Type:    fieldType,
		}, master.ID)
		require.NoErrorf(t, err, "type %s should be accepted", fieldType)
		assert.Equal(t, fieldType, field.Type)
		assert.NotEmpty(t, field.ID)
	}
}

func TestFieldService_CreateField_RejectsInvalidType(t *testing.T) {
	svc, table, master := setupFieldTestEnv(t)

	invalidTypes := []string{
		"integer",
		"float",
		"array",
		"object",
		"blob",
		"timestamp",
		"",
	}

	for _, fieldType := range invalidTypes {
		_, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "field_bad",
			Type:    fieldType,
		}, master.ID)
		assert.Errorf(t, err, "type %q should be rejected", fieldType)
		assert.Contains(t, err.Error(), "字段类型验证失败")
	}
}

func TestFieldService_CreateField_RejectsInvalidName(t *testing.T) {
	svc, table, master := setupFieldTestEnv(t)

	t.Run("empty name", func(t *testing.T) {
		_, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "",
			Type:    "string",
		}, master.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "字段名称验证失败")
	})

	t.Run("name with spaces", func(t *testing.T) {
		_, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "has spaces",
			Type:    "string",
		}, master.ID)
		assert.Error(t, err)
	})

	t.Run("name starts with digit", func(t *testing.T) {
		_, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "1field",
			Type:    "string",
		}, master.ID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "不能以数字开头")
	})

	t.Run("name with special characters", func(t *testing.T) {
		_, err := svc.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "field@name!",
			Type:    "string",
		}, master.ID)
		assert.Error(t, err)
	})
}

func TestFieldService_CreateField_RejectsDuplicateName(t *testing.T) {
	svc, table, master := setupFieldTestEnv(t)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "username",
		Type:    "string",
	}, master.ID)
	require.NoError(t, err)

	_, err = svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "username",
		Type:    "text",
	}, master.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "已存在同名字段")
}

func TestFieldService_CreateField_StoresConfig(t *testing.T) {
	svc, table, master := setupFieldTestEnv(t)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "tags",
		Type:    "list",
		Options: "red,green,blue",
	}, master.ID)
	require.NoError(t, err)
	assert.Equal(t, "list", field.Type)
	assert.Contains(t, field.Options, "red")
	assert.Contains(t, field.Options, "green")
	assert.Contains(t, field.Options, "blue")
}

func TestFieldService_CreateField_WithRequired(t *testing.T) {
	svc, table, master := setupFieldTestEnv(t)

	field, err := svc.CreateField(CreateFieldRequest{
		TableID:  table.ID,
		Name:     "email",
		Type:     "string",
		Required: true,
	}, master.ID)
	require.NoError(t, err)
	assert.True(t, field.Required)
}

func TestFieldService_CreateField_UnauthorizedToken(t *testing.T) {
	db := setupTestDB(t)
	svc := NewFieldService(db)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	viewer := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer",
		Scopes: `{"databases":{"` + database.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	_, err := svc.CreateField(CreateFieldRequest{
		TableID: table.ID,
		Name:    "unauthorized_field",
		Type:    "string",
	}, viewer.ID)
	assert.Error(t, err)
}
