package query

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/testutil"
)

func setupScopedExecutorTestDB(t *testing.T) *gorm.DB {
	return testutil.SetupTestDB(t)
}

func TestExecutor_ExecuteHonorsTokenDatabaseScopes(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	allowedDB := &models.Database{Name: "allowed"}
	blockedDB := &models.Database{Name: "blocked"}
	require.NoError(t, db.Create(allowedDB).Error)
	require.NoError(t, db.Create(blockedDB).Error)

	allowedTable := &models.Table{DatabaseID: allowedDB.ID, Name: "allowed_records"}
	blockedTable := &models.Table{DatabaseID: blockedDB.ID, Name: "blocked_records"}
	require.NoError(t, db.Create(allowedTable).Error)
	require.NoError(t, db.Create(blockedTable).Error)

	require.NoError(t, db.Create(&models.Record{TableID: allowedTable.ID, Data: `{"name":"allowed"}`}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: blockedTable.ID, Data: `{"name":"blocked"}`}).Error)

	token := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer_scope",
		Scopes: `{"databases":{"` + allowedDB.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(token).Error)

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "records",
		Select: []string{"id", "table_id"},
		Page:   1,
		Size:   20,
	}, token.ID)
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, allowedTable.ID, result.Data[0]["table_id"])
}

func TestExecutor_EmptyScopeReturnsError(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	database := &models.Database{Name: "testdb"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	require.NoError(t, db.Create(&models.Record{TableID: table.ID, Data: `{"name":"alice"}`}).Error)

	token := &models.Token{
		Name:   "empty",
		Token:  "cs_empty_scope",
		Scopes: `{}`,
	}
	require.NoError(t, db.Create(token).Error)

	executor := NewExecutor(db)
	_, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "records",
		Select: []string{"id"},
		Page:   1,
		Size:   20,
	}, token.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no databases available")
}

func TestExecutor_TableLevelScope(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	database := &models.Database{Name: "testdb"}
	require.NoError(t, db.Create(database).Error)

	usersTable := &models.Table{DatabaseID: database.ID, Name: "users"}
	ordersTable := &models.Table{DatabaseID: database.ID, Name: "orders"}
	require.NoError(t, db.Create(usersTable).Error)
	require.NoError(t, db.Create(ordersTable).Error)

	require.NoError(t, db.Create(&models.Record{TableID: usersTable.ID, Data: `{"name":"alice"}`}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: ordersTable.ID, Data: `{"order":"123"}`}).Error)

	token := &models.Token{
		Name:   "tbl_scope",
		Token:  "cs_tbl_scope",
		Scopes: `{"databases":{"` + database.ID + `":"viewer"},"tables":{"` + usersTable.ID + `":{"role":"viewer"}}}`,
	}
	require.NoError(t, db.Create(token).Error)

	executor := NewExecutor(db)

	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "records",
		Select: []string{"id", "table_id"},
		Page:   1,
		Size:   20,
	}, token.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Data), 2)
}

func TestExecutor_MasterTokenSeesAll(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	database := &models.Database{Name: "testdb"}
	require.NoError(t, db.Create(database).Error)

	table := &models.Table{DatabaseID: database.ID, Name: "users"}
	require.NoError(t, db.Create(table).Error)

	require.NoError(t, db.Create(&models.Record{TableID: table.ID, Data: `{"name":"alice"}`}).Error)
	require.NoError(t, db.Create(&models.Record{TableID: table.ID, Data: `{"name":"bob"}`}).Error)

	master := &models.Token{
		Name:     "master",
		Token:    "cs_master",
		IsMaster: true,
	}
	require.NoError(t, db.Create(master).Error)

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "records",
		Select: []string{"id", "table_id"},
		Page:   1,
		Size:   20,
	}, master.ID)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(result.Data), 2)
}

func TestExecutor_QueryDatabasesWithScope(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	db1 := &models.Database{Name: "db1"}
	db2 := &models.Database{Name: "db2"}
	require.NoError(t, db.Create(db1).Error)
	require.NoError(t, db.Create(db2).Error)

	token := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer",
		Scopes: `{"databases":{"` + db1.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(token).Error)

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "databases",
		Select: []string{"id", "name"},
		Page:   1,
		Size:   20,
	}, token.ID)
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, db1.ID, result.Data[0]["id"])
}

func TestExecutor_QueryTablesWithScope(t *testing.T) {
	db := setupScopedExecutorTestDB(t)

	allowedDB := &models.Database{Name: "allowed"}
	blockedDB := &models.Database{Name: "blocked"}
	require.NoError(t, db.Create(allowedDB).Error)
	require.NoError(t, db.Create(blockedDB).Error)

	allowedTable := &models.Table{DatabaseID: allowedDB.ID, Name: "users"}
	blockedTable := &models.Table{DatabaseID: blockedDB.ID, Name: "orders"}
	require.NoError(t, db.Create(allowedTable).Error)
	require.NoError(t, db.Create(blockedTable).Error)

	token := &models.Token{
		Name:   "viewer",
		Token:  "cs_viewer",
		Scopes: `{"databases":{"` + allowedDB.ID + `":"viewer"}}`,
	}
	require.NoError(t, db.Create(token).Error)

	executor := NewExecutor(db)
	result, err := executor.Execute(context.Background(), &QueryRequest{
		From:   "tables",
		Select: []string{"id", "name"},
		Page:   1,
		Size:   20,
	}, token.ID)
	require.NoError(t, err)
	require.Len(t, result.Data, 1)
	assert.Equal(t, allowedTable.ID, result.Data[0]["id"])
}
