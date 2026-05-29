package services

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/backend/internal/models"
)

func TestFieldService_CreateField_AcceptsExtendedTypes(t *testing.T) {
	db := setupTestDB(t)
	fieldService := NewFieldService(db)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	master := &models.Token{Name: "master", Token: "cs_master", IsMaster: true}
	require.NoError(t, db.Create(master).Error)

	types := []string{
		"select",
		"multiselect",
		"list",
		"json",
		"file",
		"link",
		"email",
		"url",
		"color",
		"rating",
	}

	for _, fieldType := range types {
		_, err := fieldService.CreateField(CreateFieldRequest{
			TableID: table.ID,
			Name:    "field_" + fieldType,
			Type:    fieldType,
		}, master.ID)
		require.NoErrorf(t, err, "type %s should be accepted", fieldType)
	}
}
