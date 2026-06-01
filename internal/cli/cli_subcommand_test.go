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

func findSub(t *testing.T, parentPath ...string) {
	t.Helper()
	cmd, _, err := rootCmd.Find(parentPath)
	require.NoError(t, err, "Find(%v) failed", parentPath)
	require.NotNil(t, cmd, "Find(%v) returned nil command", parentPath)
}

func TestCommandStructure(t *testing.T) {
	findSub(t, "db")
	findSub(t, "db", "list")
	findSub(t, "db", "create")
	findSub(t, "db", "get")
	findSub(t, "db", "update")
	findSub(t, "db", "delete")
	findSub(t, "table")
	findSub(t, "table", "list")
	findSub(t, "table", "create")
	findSub(t, "table", "get")
	findSub(t, "table", "update")
	findSub(t, "table", "delete")
	findSub(t, "field")
	findSub(t, "field", "list")
	findSub(t, "field", "create")
	findSub(t, "field", "get")
	findSub(t, "field", "update")
	findSub(t, "field", "delete")
	findSub(t, "record")
	findSub(t, "record", "list")
	findSub(t, "record", "create")
	findSub(t, "record", "get")
	findSub(t, "record", "update")
	findSub(t, "record", "delete")
	findSub(t, "record", "batch")
	findSub(t, "token")
	findSub(t, "token", "list")
	findSub(t, "token", "create")
	findSub(t, "token", "update")
	findSub(t, "token", "delete")
	findSub(t, "cache")
	findSub(t, "cache", "clear")
}

func TestRootCmd_HasVersion(t *testing.T) {
	assert.Equal(t, "cornerstone", rootCmd.Use)
	assert.NotNil(t, rootCmd.Version)
}

func TestDBCmd_HasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"db"})
	require.NoError(t, err)
	names := subcommandNames(cmd)
	for _, n := range []string{"list", "create", "get", "update", "delete"} {
		assert.Contains(t, names, n, "db missing subcommand %q", n)
	}
}

func TestTableCmd_HasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"table"})
	require.NoError(t, err)
	names := subcommandNames(cmd)
	for _, n := range []string{"list", "create", "get", "update", "delete"} {
		assert.Contains(t, names, n, "table missing subcommand %q", n)
	}
}

func TestFieldCmd_HasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"field"})
	require.NoError(t, err)
	names := subcommandNames(cmd)
	for _, n := range []string{"list", "create", "get", "update", "delete"} {
		assert.Contains(t, names, n, "field missing subcommand %q", n)
	}
}

func TestRecordCmd_HasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"record"})
	require.NoError(t, err)
	names := subcommandNames(cmd)
	for _, n := range []string{"list", "create", "get", "update", "delete", "batch"} {
		assert.Contains(t, names, n, "record missing subcommand %q", n)
	}
}

func TestTokenCmd_HasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"token"})
	require.NoError(t, err)
	names := subcommandNames(cmd)
	for _, n := range []string{"list", "create", "update", "delete"} {
		assert.Contains(t, names, n, "token missing subcommand %q", n)
	}
}

func TestCacheCmd_HasSubcommands(t *testing.T) {
	cmd, _, err := rootCmd.Find([]string{"cache"})
	require.NoError(t, err)
	names := subcommandNames(cmd)
	assert.Contains(t, names, "clear", "cache missing subcommand \"clear\"")
}

func subcommandNames(cmd *cobra.Command) []string {
	var names []string
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	return names
}

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

func TestDBCreateCmd_RequiresArgs(t *testing.T) {
	err := dbCreateCmd.Args(dbCreateCmd, []string{})
	assert.Error(t, err)
}

func TestDBGetCmd_RequiresArgs(t *testing.T) {
	err := dbGetCmd.Args(dbGetCmd, []string{})
	assert.Error(t, err)
}

func TestDBUpdateCmd_RequiresArgs(t *testing.T) {
	err := dbUpdateCmd.Args(dbUpdateCmd, []string{})
	assert.Error(t, err)
}

func TestDBDeleteCmd_RequiresArgs(t *testing.T) {
	err := dbDeleteCmd.Args(dbDeleteCmd, []string{})
	assert.Error(t, err)
}

func TestDBListCmd_AcceptsZeroArgs(t *testing.T) {
	assert.Nil(t, dbListCmd.Args)
}

func TestDBCreateCmd_AcceptsOneArg(t *testing.T) {
	err := dbCreateCmd.Args(dbCreateCmd, []string{"mydb"})
	assert.NoError(t, err)
}

func TestDBCreateCmd_RejectsTwoArgs(t *testing.T) {
	err := dbCreateCmd.Args(dbCreateCmd, []string{"a", "b"})
	assert.Error(t, err)
}

func TestTableListCmd_RequiresArgs(t *testing.T) {
	err := tableListCmd.Args(tableListCmd, []string{})
	assert.Error(t, err)
}

func TestTableCreateCmd_RequiresTwoArgs(t *testing.T) {
	err := tableCreateCmd.Args(tableCreateCmd, []string{})
	assert.Error(t, err)
	err = tableCreateCmd.Args(tableCreateCmd, []string{"db1"})
	assert.Error(t, err)
	err = tableCreateCmd.Args(tableCreateCmd, []string{"db1", "tbl"})
	assert.NoError(t, err)
}

func TestTableGetCmd_RequiresArgs(t *testing.T) {
	err := tableGetCmd.Args(tableGetCmd, []string{})
	assert.Error(t, err)
}

func TestTableUpdateCmd_RequiresArgs(t *testing.T) {
	err := tableUpdateCmd.Args(tableUpdateCmd, []string{})
	assert.Error(t, err)
}

func TestTableDeleteCmd_RequiresArgs(t *testing.T) {
	err := tableDeleteCmd.Args(tableDeleteCmd, []string{})
	assert.Error(t, err)
}

func TestFieldListCmd_RequiresArgs(t *testing.T) {
	err := fieldListCmd.Args(fieldListCmd, []string{})
	assert.Error(t, err)
}

func TestFieldCreateCmd_RequiresThreeArgs(t *testing.T) {
	err := fieldCreateCmd.Args(fieldCreateCmd, []string{})
	assert.Error(t, err)
	err = fieldCreateCmd.Args(fieldCreateCmd, []string{"tbl1"})
	assert.Error(t, err)
	err = fieldCreateCmd.Args(fieldCreateCmd, []string{"tbl1", "fname"})
	assert.Error(t, err)
	err = fieldCreateCmd.Args(fieldCreateCmd, []string{"tbl1", "fname", "string"})
	assert.NoError(t, err)
}

func TestFieldGetCmd_RequiresArgs(t *testing.T) {
	err := fieldGetCmd.Args(fieldGetCmd, []string{})
	assert.Error(t, err)
}

func TestFieldUpdateCmd_RequiresArgs(t *testing.T) {
	err := fieldUpdateCmd.Args(fieldUpdateCmd, []string{})
	assert.Error(t, err)
}

func TestFieldDeleteCmd_RequiresArgs(t *testing.T) {
	err := fieldDeleteCmd.Args(fieldDeleteCmd, []string{})
	assert.Error(t, err)
}

func TestRecordListCmd_RequiresArgs(t *testing.T) {
	err := recordListCmd.Args(recordListCmd, []string{})
	assert.Error(t, err)
}

func TestRecordCreateCmd_RequiresTwoArgs(t *testing.T) {
	err := recordCreateCmd.Args(recordCreateCmd, []string{})
	assert.Error(t, err)
	err = recordCreateCmd.Args(recordCreateCmd, []string{"tbl1"})
	assert.Error(t, err)
	err = recordCreateCmd.Args(recordCreateCmd, []string{"tbl1", `{"k":"v"}`})
	assert.NoError(t, err)
}

func TestRecordGetCmd_RequiresArgs(t *testing.T) {
	err := recordGetCmd.Args(recordGetCmd, []string{})
	assert.Error(t, err)
}

func TestRecordUpdateCmd_RequiresTwoArgs(t *testing.T) {
	err := recordUpdateCmd.Args(recordUpdateCmd, []string{})
	assert.Error(t, err)
	err = recordUpdateCmd.Args(recordUpdateCmd, []string{"rec1"})
	assert.Error(t, err)
}

func TestRecordDeleteCmd_RequiresArgs(t *testing.T) {
	err := recordDeleteCmd.Args(recordDeleteCmd, []string{})
	assert.Error(t, err)
}

func TestRecordBatchCmd_RequiresThreeArgs(t *testing.T) {
	err := recordBatchCmd.Args(recordBatchCmd, []string{})
	assert.Error(t, err)
	err = recordBatchCmd.Args(recordBatchCmd, []string{"tbl1"})
	assert.Error(t, err)
	err = recordBatchCmd.Args(recordBatchCmd, []string{"tbl1", `{"k":"v"}`})
	assert.Error(t, err)
	err = recordBatchCmd.Args(recordBatchCmd, []string{"tbl1", `{"k":"v"}`, "5"})
	assert.NoError(t, err)
}

func TestTokenCreateCmd_RequiresArgs(t *testing.T) {
	err := tokenCreateCmd.Args(tokenCreateCmd, []string{})
	assert.Error(t, err)
}

func TestTokenUpdateCmd_RequiresArgs(t *testing.T) {
	err := tokenUpdateCmd.Args(tokenUpdateCmd, []string{})
	assert.Error(t, err)
}

func TestTokenDeleteCmd_RequiresArgs(t *testing.T) {
	err := tokenDeleteCmd.Args(tokenDeleteCmd, []string{})
	assert.Error(t, err)
}

func TestTokenListCmd_AcceptsZeroArgs(t *testing.T) {
	assert.Nil(t, tokenListCmd.Args)
}

func TestCacheClearCmd_AcceptsZeroArgs(t *testing.T) {
	assert.Nil(t, cacheClearCmd.Args)
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

func TestDBCreateCmd_HasDescriptionFlag(t *testing.T) {
	f := dbCreateCmd.Flags().Lookup("description")
	require.NotNil(t, f)
	assert.Equal(t, "d", f.Shorthand)
}

func TestDBUpdateCmd_HasFlags(t *testing.T) {
	require.NotNil(t, dbUpdateCmd.Flags().Lookup("name"))
	require.NotNil(t, dbUpdateCmd.Flags().Lookup("description"))
}

func TestFieldCreateCmd_HasFlags(t *testing.T) {
	require.NotNil(t, fieldCreateCmd.Flags().Lookup("description"))
	require.NotNil(t, fieldCreateCmd.Flags().Lookup("required"))
	require.NotNil(t, fieldCreateCmd.Flags().Lookup("options"))
}

func TestFieldUpdateCmd_HasFlags(t *testing.T) {
	require.NotNil(t, fieldUpdateCmd.Flags().Lookup("name"))
	require.NotNil(t, fieldUpdateCmd.Flags().Lookup("type"))
	require.NotNil(t, fieldUpdateCmd.Flags().Lookup("description"))
	require.NotNil(t, fieldUpdateCmd.Flags().Lookup("required"))
	require.NotNil(t, fieldUpdateCmd.Flags().Lookup("options"))
}

func TestRecordListCmd_HasFlags(t *testing.T) {
	require.NotNil(t, recordListCmd.Flags().Lookup("limit"))
	require.NotNil(t, recordListCmd.Flags().Lookup("offset"))
	require.NotNil(t, recordListCmd.Flags().Lookup("filter"))
}

func TestRecordUpdateCmd_HasVersionFlag(t *testing.T) {
	require.NotNil(t, recordUpdateCmd.Flags().Lookup("version"))
}

func TestTokenCreateCmd_HasFlags(t *testing.T) {
	require.NotNil(t, tokenCreateCmd.Flags().Lookup("scopes"))
	require.NotNil(t, tokenCreateCmd.Flags().Lookup("expires"))
}

func TestTokenUpdateCmd_HasFlags(t *testing.T) {
	require.NotNil(t, tokenUpdateCmd.Flags().Lookup("scopes"))
	require.NotNil(t, tokenUpdateCmd.Flags().Lookup("expires"))
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
	assert.Contains(t, out, "已删除")
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
	assert.Contains(t, out, "已删除")
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
	assert.Contains(t, out, "已删除")
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
	assert.Contains(t, out, "Token 创建成功")
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
	assert.Contains(t, out, "已删除")
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
	assert.Contains(t, err.Error(), "1-100")
}

func TestRecordBatchCmd_NonNumericCount(t *testing.T) {
	setupCLIEnv(t)
	err := recordBatchCmd.RunE(recordBatchCmd, []string{"tbl_x", `{"k":"v"}`, "abc"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "1-100")
}

func TestRecordBatchCmd_CountTooLarge(t *testing.T) {
	setupCLIEnv(t)
	err := recordBatchCmd.RunE(recordBatchCmd, []string{"tbl_x", `{"k":"v"}`, "101"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "1-100")
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
	assert.Contains(t, out, "缓存已清空")
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
	assert.Contains(t, out, "成功创建 3 条记录")
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
	assert.Contains(t, out, "已删除")
}

func extractJSON(s string) string {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start >= 0 && end > start {
		return s[start : end+1]
	}
	return s
}
