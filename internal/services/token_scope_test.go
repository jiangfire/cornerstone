package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/models"
)

func TestDatabaseService_ListDatabasesHonorsTokenScopes(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	allowedDB := &models.Database{Name: "allowed"}
	blockedDB := &models.Database{Name: "blocked"}
	require.NoError(t, db.Create(allowedDB).Error)
	require.NoError(t, db.Create(blockedDB).Error)

	viewer := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer_scope",
		Scopes: `{"databases":{"` + allowedDB.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(viewer).Error)

	databases, err := svc.ListDatabases(viewer.ID)
	require.NoError(t, err)
	require.Len(t, databases, 1)
	assert.Equal(t, allowedDB.ID, databases[0].ID)
}

