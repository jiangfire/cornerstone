package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jiangfire/cornerstone/internal/authz"
	"github.com/jiangfire/cornerstone/internal/models"
)

func TestDatabaseService_ListDatabases_DBLevelScope(t *testing.T) {
	db := setupTestDB(t)
	svc := NewDatabaseService(db)

	allowedDB := &models.Database{Name: "allowed"}
	blockedDB := &models.Database{Name: "blocked"}
	require.NoError(t, db.Create(allowedDB).Error)
	require.NoError(t, db.Create(blockedDB).Error)

	t.Run("viewer sees only scoped database", func(t *testing.T) {
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
	})

	t.Run("empty scope sees nothing", func(t *testing.T) {
		empty := &models.Token{
			Name:   "empty",
			Token:  "cs_empty_scope",
			Scopes: `{}`,
		}
		require.NoError(t, db.Create(empty).Error)

		databases, err := svc.ListDatabases(empty.ID)
		require.NoError(t, err)
		assert.Empty(t, databases)
	})

	t.Run("master sees all databases", func(t *testing.T) {
		databases, err := svc.ListDatabases("user1")
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(databases), 2)
	})
}

func TestAuthorizer_TableLevelScope(t *testing.T) {
	db := setupTestDB(t)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	table1 := &models.Table{DatabaseID: database.ID, Name: "users"}
	table2 := &models.Table{DatabaseID: database.ID, Name: "orders"}
	require.NoError(t, db.Create(table1).Error)
	require.NoError(t, db.Create(table2).Error)

	t.Run("table scope overrides database scope for specific table", func(t *testing.T) {
		token := &models.Token{
			Name:   "tbl_viewer",
			Token:  "cs_tbl_viewer",
			Scopes: `{"databases":{"` + database.ID + `":"viewer"},"tables":{"` + table1.ID + `":{"role":"admin"}}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessTable(table1.ID, authz.ActionManage))
		assert.True(t, auth.CanAccessTable(table1.ID, authz.ActionRead))

		assert.True(t, auth.CanAccessTable(table2.ID, authz.ActionRead))
		assert.False(t, auth.CanAccessTable(table2.ID, authz.ActionWrite))
	})

	t.Run("table without database scope is only accessible via explicit table grant", func(t *testing.T) {
		token := &models.Token{
			Name:   "tbl_only",
			Token:  "cs_tbl_only",
			Scopes: `{"tables":{"` + table1.ID + `":{"role":"viewer"}}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessTable(table1.ID, authz.ActionRead))
		assert.False(t, auth.CanAccessTable(table2.ID, authz.ActionRead))
	})
}

func TestAuthorizer_FieldLevelScope(t *testing.T) {
	db := setupTestDB(t)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	fieldName := &models.Field{TableID: table.ID, Name: "name", Type: "string"}
	fieldEmail := &models.Field{TableID: table.ID, Name: "email", Type: "string"}
	fieldSalary := &models.Field{TableID: table.ID, Name: "salary", Type: "number"}
	require.NoError(t, db.Create(fieldName).Error)
	require.NoError(t, db.Create(fieldEmail).Error)
	require.NoError(t, db.Create(fieldSalary).Error)

	t.Run("field scope grants additional actions beyond table scope", func(t *testing.T) {
		token := &models.Token{
			Name:   "field_viewer",
			Token:  "cs_field_viewer",
			Scopes: `{"databases":{"` + database.ID + `":"viewer"},"tables":{"` + table.ID + `":{"role":"viewer","fields":{"` + fieldSalary.ID + `":["read","write"]}}}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessField(fieldSalary.ID, authz.ActionRead))
		assert.True(t, auth.CanAccessField(fieldSalary.ID, authz.ActionWrite))

		assert.True(t, auth.CanAccessField(fieldName.ID, authz.ActionRead))
		assert.False(t, auth.CanAccessField(fieldName.ID, authz.ActionWrite))
	})

	t.Run("field scope falls back to table scope when field not listed", func(t *testing.T) {
		token := &models.Token{
			Name:   "field_fallback",
			Token:  "cs_field_fallback",
			Scopes: `{"databases":{"` + database.ID + `":"editor"},"tables":{"` + table.ID + `":{"role":"editor","fields":{"` + fieldSalary.ID + `":["read"]}}}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessField(fieldName.ID, authz.ActionWrite))
	})

	t.Run("field scope by field name", func(t *testing.T) {
		token := &models.Token{
			Name:   "field_name_viewer",
			Token:  "cs_field_name_viewer",
			Scopes: `{"databases":{"` + database.ID + `":"viewer"},"tables":{"` + table.ID + `":{"role":"viewer","fields":{"salary":["read","write"]}}}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessField(fieldSalary.ID, authz.ActionRead))
		assert.True(t, auth.CanAccessField(fieldSalary.ID, authz.ActionWrite))
	})

	t.Run("master can access all fields", func(t *testing.T) {
		auth, err := authz.NewAuthorizer(db, "user1")
		require.NoError(t, err)

		assert.True(t, auth.CanAccessField(fieldName.ID, authz.ActionRead))
		assert.True(t, auth.CanAccessField(fieldSalary.ID, authz.ActionManage))
	})
}

func TestAuthorizer_EditorVsViewer(t *testing.T) {
	db := setupTestDB(t)

	database := &models.Database{Name: "TestDB"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	t.Run("viewer cannot write", func(t *testing.T) {
		token := &models.Token{
			Name:   "viewer",
			Token:  "cs_viewer_role",
			Scopes: `{"databases":{"` + database.ID + `":"viewer"}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionRead))
		assert.False(t, auth.CanAccessDatabase(database.ID, authz.ActionWrite))
		assert.False(t, auth.CanAccessDatabase(database.ID, authz.ActionDelete))
	})

	t.Run("editor can read and write but not delete", func(t *testing.T) {
		token := &models.Token{
			Name:   "editor",
			Token:  "cs_editor_role",
			Scopes: `{"databases":{"` + database.ID + `":"editor"}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionRead))
		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionWrite))
		assert.False(t, auth.CanAccessDatabase(database.ID, authz.ActionDelete))
	})

	t.Run("admin can do everything", func(t *testing.T) {
		token := &models.Token{
			Name:   "admin",
			Token:  "cs_admin_role",
			Scopes: `{"databases":{"` + database.ID + `":"admin"}}`,
		}
		require.NoError(t, db.Create(token).Error)

		auth, err := authz.NewAuthorizer(db, token.ID)
		require.NoError(t, err)

		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionRead))
		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionWrite))
		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionDelete))
		assert.True(t, auth.CanAccessDatabase(database.ID, authz.ActionManage))
	})
}
