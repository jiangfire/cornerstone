package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/jiangfire/cornerstone/internal/services"
	pkgdb "github.com/jiangfire/cornerstone/pkg/db"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetMasterTokenID_MissingEnv2(t *testing.T) {
	t.Setenv("MASTER_TOKEN", "")
	_, err := getMasterTokenID()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MASTER_TOKEN")
}

func TestGetMasterTokenID_Present(t *testing.T) {
	t.Setenv("MASTER_TOKEN", "tok_abc123")
	id, err := getMasterTokenID()
	assert.NoError(t, err)
	assert.Equal(t, "tok_abc123", id)
}

func TestEnsureDB_ConfigLoadError(t *testing.T) {
	t.Setenv("DB_TYPE", "invalid_db_type")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("PORT", "8080")
	err := ensureDB()
	assert.Error(t, err)
}

func TestCommandArgs_Validation(t *testing.T) {
	tests := []struct {
		name string
		cmd  *cobra.Command
		args []string
		want string // "error" or "ok"
	}{
		// Single-arg commands (representative sample)
		{"dbCreate/empty", dbCreateCmd, []string{}, "error"},
		{"dbCreate/valid", dbCreateCmd, []string{"mydb"}, "ok"},
		{"dbCreate/tooMany", dbCreateCmd, []string{"a", "b"}, "error"},
		{"dbGet/empty", dbGetCmd, []string{}, "error"},
		{"dbUpdate/empty", dbUpdateCmd, []string{}, "error"},
		{"dbDelete/empty", dbDeleteCmd, []string{}, "error"},
		{"tableGet/empty", tableGetCmd, []string{}, "error"},
		{"tableDelete/empty", tableDeleteCmd, []string{}, "error"},
		{"fieldList/empty", fieldListCmd, []string{}, "error"},
		{"fieldGet/empty", fieldGetCmd, []string{}, "error"},
		{"recordList/empty", recordListCmd, []string{}, "error"},
		{"recordGet/empty", recordGetCmd, []string{}, "error"},
		{"recordDelete/empty", recordDeleteCmd, []string{}, "error"},
		{"tokenCreate/empty", tokenCreateCmd, []string{}, "error"},
		{"tokenUpdate/empty", tokenUpdateCmd, []string{}, "error"},
		{"tokenDelete/empty", tokenDeleteCmd, []string{}, "error"},

		// Two-arg commands
		{"tableCreate/empty", tableCreateCmd, []string{}, "error"},
		{"tableCreate/oneArg", tableCreateCmd, []string{"db1"}, "error"},
		{"tableCreate/valid", tableCreateCmd, []string{"db1", "tbl"}, "ok"},
		{"recordCreate/empty", recordCreateCmd, []string{}, "error"},
		{"recordCreate/oneArg", recordCreateCmd, []string{"tbl1"}, "error"},
		{"recordCreate/valid", recordCreateCmd, []string{"tbl1", `{"k":"v"}`}, "ok"},
		{"recordUpdate/empty", recordUpdateCmd, []string{}, "error"},
		{"recordUpdate/oneArg", recordUpdateCmd, []string{"rec1"}, "error"},

		// Three-arg commands
		{"fieldCreate/empty", fieldCreateCmd, []string{}, "error"},
		{"fieldCreate/twoArgs", fieldCreateCmd, []string{"tbl1", "fname"}, "error"},
		{"fieldCreate/valid", fieldCreateCmd, []string{"tbl1", "fname", "string"}, "ok"},
		{"recordBatch/empty", recordBatchCmd, []string{}, "error"},
		{"recordBatch/twoArgs", recordBatchCmd, []string{"tbl1", `{"k":"v"}`}, "error"},
		{"recordBatch/valid", recordBatchCmd, []string{"tbl1", `{"k":"v"}`, "5"}, "ok"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Args(tt.cmd, tt.args)
			switch tt.want {
			case "error":
				assert.Error(t, err)
			case "ok":
				assert.NoError(t, err)
			}
		})
	}
}

func TestPrintJSON_Output(t *testing.T) {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err := printJSON(map[string]string{"hello": "world"})
	_ = w.Close()
	os.Stdout = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	require.NoError(t, err)
	assert.Contains(t, buf.String(), `"hello"`)
	assert.Contains(t, buf.String(), `"world"`)
}

func setupCLIEnv(t *testing.T) {
	t.Helper()
	tmpFile, err := os.CreateTemp("", "cornerstone-cli-test-*.db")
	require.NoError(t, err)
	tmpFile.Close()
	dbPath := tmpFile.Name()

	t.Setenv("DB_TYPE", "sqlite")
	t.Setenv("DATABASE_URL", dbPath)
	t.Setenv("MASTER_TOKEN", "cs_test_master_token")
	t.Setenv("PORT", "8080")
	t.Setenv("LOG_LEVEL", "error")

	t.Cleanup(func() {
		_ = pkgdb.CloseDB()
		pkgdb.SetDB(nil)
		os.Remove(dbPath)
	})

	require.NoError(t, ensureDB())

	pkgdb.DB().Create(&models.Token{
		ID:       "cs_test_master_token",
		Token:    "cs_test_master_token",
		Name:     "master",
		IsMaster: true,
		Scopes:   "{}",
	})
}

func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	_ = w.Close()
	os.Stdout = old
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(r)
	return buf.String()
}

func TestDBListCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	out := captureOutput(t, func() {
		err := dbListCmd.RunE(dbListCmd, []string{})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "[")
}

func TestDBCreateCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	out := captureOutput(t, func() {
		dbCreateCmd.SetArgs([]string{"testdb"})
		_ = dbCreateCmd.Flags().Set("description", "a test db")
		err := dbCreateCmd.RunE(dbCreateCmd, []string{"testdb"})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "testdb")
	var m map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(extractJSON(out)), &m))
	assert.Equal(t, "testdb", m["name"])
	assert.Equal(t, "a test db", m["description"])
}

func TestDBGetCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	svc := services.NewDatabaseService(pkgdb.DB())
	created, err := svc.CreateDatabase(services.CreateDBRequest{
		Name:        "getdb",
		Description: "get test",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := dbGetCmd.RunE(dbGetCmd, []string{created.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "getdb")
}

func TestDBUpdateCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	svc := services.NewDatabaseService(pkgdb.DB())
	created, err := svc.CreateDatabase(services.CreateDBRequest{
		Name: "original",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		_ = dbUpdateCmd.Flags().Set("name", "updated")
		_ = dbUpdateCmd.Flags().Set("description", "new desc")
		err := dbUpdateCmd.RunE(dbUpdateCmd, []string{created.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "updated")
}

func TestDBDeleteCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	svc := services.NewDatabaseService(pkgdb.DB())
	created, err := svc.CreateDatabase(services.CreateDBRequest{
		Name: "delme",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := dbDeleteCmd.RunE(dbDeleteCmd, []string{created.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "deleted")
}

func TestDBListCmd_NoMasterToken(t *testing.T) {
	setupCLIEnv(t)
	t.Setenv("MASTER_TOKEN", "")
	err := dbListCmd.RunE(dbListCmd, []string{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "MASTER_TOKEN")
}

func TestTableListCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{
		Name: "tbltest",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := tableListCmd.RunE(tableListCmd, []string{createdDB.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "[")
}

func TestTableCreateCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{
		Name: "tblcreatedb",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		_ = tableCreateCmd.Flags().Set("description", "a table")
		err := tableCreateCmd.RunE(tableCreateCmd, []string{createdDB.ID, "mytable"})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "mytable")
}

func TestTableGetCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "tblgetdb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID:  createdDB.ID,
		Name:        "gettbl",
		Description: "get table",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := tableGetCmd.RunE(tableGetCmd, []string{createdTbl.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "gettbl")
}

func TestTableDeleteCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "tblDdelDB"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "deltbl",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := tableDeleteCmd.RunE(tableDeleteCmd, []string{createdTbl.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "deleted")
}

func TestFieldListCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "fldlistdb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "fldtbl",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := fieldListCmd.RunE(fieldListCmd, []string{createdTbl.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "[")
}

func TestFieldCreateCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "fldcreatedb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "fldtbl2",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		_ = fieldCreateCmd.Flags().Set("description", "a field")
		_ = fieldCreateCmd.Flags().Set("required", "true")
		err := fieldCreateCmd.RunE(fieldCreateCmd, []string{createdTbl.ID, "title", "string"})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "title")
}

func TestFieldDeleteCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "flddelDB"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "fldtbl3",
	}, "cs_test_master_token")
	require.NoError(t, err)
	fldSvc := services.NewFieldService(pkgdb.DB())
	createdFld, err := fldSvc.CreateField(services.CreateFieldRequest{
		TableID: createdTbl.ID,
		Name:    "delfld",
		Type:    "string",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := fieldDeleteCmd.RunE(fieldDeleteCmd, []string{createdFld.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "deleted")
}

func TestTokenListCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	out := captureOutput(t, func() {
		err := tokenListCmd.RunE(tokenListCmd, []string{})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "[")
}

func TestTokenCreateCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	out := captureOutput(t, func() {
		err := tokenCreateCmd.RunE(tokenCreateCmd, []string{"mytoken"})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "token created successfully!")
	assert.Contains(t, out, "mytoken")
}

func TestTokenDeleteCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	tokSvc := services.NewTokenService(pkgdb.DB())
	created, err := tokSvc.CreateToken(services.CreateTokenRequest{
		Name:   "deltok",
		Scopes: "{}",
	})
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := tokenDeleteCmd.RunE(tokenDeleteCmd, []string{created.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "deleted")
}

func TestRecordListCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "reclistdb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "rectbl",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := recordListCmd.RunE(recordListCmd, []string{createdTbl.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "records")
}

func TestRecordCreateCmd_InvalidJSON(t *testing.T) {
	setupCLIEnv(t)
	err := recordCreateCmd.RunE(recordCreateCmd, []string{"tbl_x", "not-json"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "JSON")
}

func TestRecordBatchCmd_InvalidCount(t *testing.T) {
	setupCLIEnv(t)
	err := recordBatchCmd.RunE(recordBatchCmd, []string{"tbl_x", `{"k":"v"}`, "0"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count must be between")
}

func TestRecordBatchCmd_NonNumericCount(t *testing.T) {
	setupCLIEnv(t)
	err := recordBatchCmd.RunE(recordBatchCmd, []string{"tbl_x", `{"k":"v"}`, "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count must be between")
}

func TestRecordBatchCmd_CountTooLarge(t *testing.T) {
	setupCLIEnv(t)
	err := recordBatchCmd.RunE(recordBatchCmd, []string{"tbl_x", `{"k":"v"}`, "101"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "count must be between")
}

func TestTokenCreateCmd_InvalidExpires(t *testing.T) {
	setupCLIEnv(t)
	_ = tokenCreateCmd.Flags().Set("expires", "not-a-date")
	err := tokenCreateCmd.RunE(tokenCreateCmd, []string{"badexp"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RFC3339")
}

func TestTokenUpdateCmd_InvalidExpires(t *testing.T) {
	setupCLIEnv(t)
	_ = tokenUpdateCmd.Flags().Set("expires", "bad")
	err := tokenUpdateCmd.RunE(tokenUpdateCmd, []string{"tok_x"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "RFC3339")
}

func TestCacheClearCmd_Success(t *testing.T) {
	services.SharedFieldCache.Set("test_key", nil)
	out := captureOutput(t, func() {
		err := cacheClearCmd.RunE(cacheClearCmd, []string{})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "all caches cleared")
	_, ok := services.SharedFieldCache.Get("test_key")
	assert.False(t, ok)
}

func TestRecordCreateCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "reccreatedb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "rectbl2",
	}, "cs_test_master_token")
	require.NoError(t, err)
	fldSvc := services.NewFieldService(pkgdb.DB())
	_, err = fldSvc.CreateField(services.CreateFieldRequest{
		TableID: createdTbl.ID,
		Name:    "title",
		Type:    "string",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := recordCreateCmd.RunE(recordCreateCmd, []string{createdTbl.ID, `{"title":"hello"}`})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "hello")
}

func TestRecordBatchCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "recbatchdb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "rectbl3",
	}, "cs_test_master_token")
	require.NoError(t, err)
	fldSvc := services.NewFieldService(pkgdb.DB())
	_, err = fldSvc.CreateField(services.CreateFieldRequest{
		TableID: createdTbl.ID,
		Name:    "name",
		Type:    "string",
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := recordBatchCmd.RunE(recordBatchCmd, []string{createdTbl.ID, `{"name":"batch"}`, "3"})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "created 3 records")
}

func TestRecordDeleteCmd_Success(t *testing.T) {
	setupCLIEnv(t)
	dbSvc := services.NewDatabaseService(pkgdb.DB())
	createdDB, err := dbSvc.CreateDatabase(services.CreateDBRequest{Name: "recdeldb"}, "cs_test_master_token")
	require.NoError(t, err)
	tblSvc := services.NewTableService(pkgdb.DB())
	createdTbl, err := tblSvc.CreateTable(services.CreateTableRequest{
		DatabaseID: createdDB.ID,
		Name:       "rectbl4",
	}, "cs_test_master_token")
	require.NoError(t, err)
	fldSvc := services.NewFieldService(pkgdb.DB())
	_, err = fldSvc.CreateField(services.CreateFieldRequest{
		TableID: createdTbl.ID,
		Name:    "val",
		Type:    "string",
	}, "cs_test_master_token")
	require.NoError(t, err)
	recSvc := services.NewRecordService(pkgdb.DB())
	createdRec, err := recSvc.CreateRecord(services.CreateRecordRequest{
		TableID: createdTbl.ID,
		Data:    map[string]interface{}{"val": "x"},
	}, "cs_test_master_token")
	require.NoError(t, err)

	out := captureOutput(t, func() {
		err := recordDeleteCmd.RunE(recordDeleteCmd, []string{createdRec.ID})
		require.NoError(t, err)
	})
	assert.Contains(t, out, "deleted")
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
