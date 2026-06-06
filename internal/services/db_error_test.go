package services

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func injectDBCreateError(t *testing.T, db *gorm.DB) {
	err := db.Callback().Create().Before("gorm:create").Register("test_inject_create_error", func(d *gorm.DB) {
		d.Error = fmt.Errorf("injected create error")
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Create().Remove("test_inject_create_error")
	})
}

func injectDBUpdateError(t *testing.T, db *gorm.DB) {
	err := db.Callback().Update().Before("gorm:update").Register("test_inject_update_error", func(d *gorm.DB) {
		d.Error = fmt.Errorf("injected update error")
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Update().Remove("test_inject_update_error")
	})
}

func injectDBQueryError(t *testing.T, db *gorm.DB) {
	err := db.Callback().Query().Before("gorm:query").Register("test_inject_query_error", func(d *gorm.DB) {
		d.Error = fmt.Errorf("injected query error")
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove("test_inject_query_error")
	})
}

func injectDBRowError(t *testing.T, db *gorm.DB) {
	err := db.Callback().Row().Before("gorm:row").Register("test_inject_row_error", func(d *gorm.DB) {
		d.Error = fmt.Errorf("injected row error")
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Row().Remove("test_inject_row_error")
	})
}

func injectQueryErrorOnNthCall(t *testing.T, db *gorm.DB, n int) {
	var counter int32
	err := db.Callback().Query().Before("gorm:query").Register("test_query_nth_error", func(d *gorm.DB) {
		if atomic.AddInt32(&counter, 1) == int32(n) {
			d.Error = fmt.Errorf("injected query error on call %d", n)
		}
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = db.Callback().Query().Remove("test_query_nth_error")
	})
}

func setupRecordDB(t *testing.T) (*RecordService, *gorm.DB, string) {
	t.Helper()
	db := setupTestDB(t)
	svc := NewRecordService(db)
	_, tbl, _ := createTestTableWithFields(t, db, "user1", "ErrTestDB", "items",
		struct {
			Name     string
			Type     string
			Required bool
		}{"title", "string", false},
	)
	return svc, db, tbl.ID
}

func injectListRecordsPageError(t *testing.T, db *gorm.DB) {
	if db.Name() == "mysql" {
		injectDBRowError(t, db)
		return
	}
	injectQueryErrorOnNthCall(t, db, 5)
}

func TestCreateRecord_DBCreateError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)
	injectDBCreateError(t, db)

	_, err := svc.CreateRecord(CreateRecordRequest{
		TableID: tableID,
		Data:    map[string]interface{}{"title": "test"},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected create error")
}

func TestCreateRecord_DBQueryFieldError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)

	SharedFieldCache.Clear()
	injectQueryErrorOnNthCall(t, db, 2)

	_, err := svc.CreateRecord(CreateRecordRequest{
		TableID: tableID,
		Data:    map[string]interface{}{"title": "test"},
	}, "user1")
	require.Error(t, err)
}

func TestUpdateRecord_DBUpdateError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)

	record, err := svc.CreateRecord(CreateRecordRequest{
		TableID: tableID,
		Data:    map[string]interface{}{"title": "original"},
	}, "user1")
	require.NoError(t, err)

	injectDBUpdateError(t, db)

	_, err = svc.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{"title": "updated"},
	}, "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected update error")
}

func TestUpdateRecord_DBQueryFieldError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)

	record, err := svc.CreateRecord(CreateRecordRequest{
		TableID: tableID,
		Data:    map[string]interface{}{"title": "original"},
	}, "user1")
	require.NoError(t, err)

	SharedFieldCache.Clear()
	injectDBQueryError(t, db)

	_, err = svc.UpdateRecord(record.ID, UpdateRecordRequest{
		Data: map[string]interface{}{"title": "updated"},
	}, "user1")
	require.Error(t, err)
}

func TestListRecords_DBQueryError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)

	SharedFieldCache.Clear()
	injectListRecordsPageError(t, db)

	_, err := svc.ListRecords(QueryRequest{
		TableID: tableID,
		Limit:   10,
	}, "user1")
	require.Error(t, err)
}

func TestDeleteRecord_DBDeleteError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)

	record, err := svc.CreateRecord(CreateRecordRequest{
		TableID: tableID,
		Data:    map[string]interface{}{"title": "to-delete"},
	}, "user1")
	require.NoError(t, err)

	injectDBUpdateError(t, db)

	err = svc.DeleteRecord(record.ID, "user1")
	require.Error(t, err)
}

func TestExportRecords_DBQueryError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)

	SharedFieldCache.Clear()
	injectQueryErrorOnNthCall(t, db, 5)

	_, _, _, err := svc.ExportRecords(tableID, "user1", "json", "")
	require.Error(t, err)
}

func TestGetRecord_DBQueryError(t *testing.T) {
	svc, db, _ := setupRecordDB(t)

	injectDBQueryError(t, db)

	_, err := svc.GetRecord("any_record_id", "user1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected query error")
}

func TestBatchCreateRecords_DBCreateError(t *testing.T) {
	svc, db, tableID := setupRecordDB(t)
	injectDBCreateError(t, db)

	_, err := svc.BatchCreateRecords(CreateRecordRequest{
		TableID: tableID,
		Data:    map[string]interface{}{"title": "batch"},
	}, "user1", 3)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "injected create error")
}
