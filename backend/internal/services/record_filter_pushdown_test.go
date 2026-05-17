package services

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
)

// TestRecordService_StructuredFilterPushdownByFieldID 校验:
// (a) 通过字段 ID 提交的结构化过滤会解析回字段名后再下推为 SQL WHERE,
// (b) SQL 端直接 LIMIT/OFFSET, in-memory 不再装载未匹配行
// 这一行为对 SQLite (JSON_EXTRACT) 和 PG (data @> jsonb) 都成立, 测试运行 SQLite 链路.
func TestRecordService_StructuredFilterPushdownByFieldID(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)

	owner := createResourceUser(t, db, "record_owner_pushdown")
	database := createResourceDatabase(t, db, owner.ID, "PushdownDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	titleField := createResourceField(t, db, table.ID, "title", "string", true, "")

	for _, title := range []string{"A", "B", "B", "C"} {
		_, err := recordService.CreateRecord(CreateRecordRequest{
			TableID: table.ID,
			Data:    map[string]interface{}{titleField.Name: title},
		}, owner.ID)
		require.NoError(t, err)
	}

	// 通过字段 ID 而非字段名提交过滤; resolveReadableFilterField 应将其映射到字段名再下推
	result, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   10,
		Filter:  fmt.Sprintf(`{"%s":"B"}`, titleField.ID),
	}, owner.ID)
	require.NoError(t, err)
	require.Len(t, result.Records, 2)
	require.Equal(t, int64(2), result.Total)
	for _, r := range result.Records {
		payload, ok := r.Data.(map[string]interface{})
		require.True(t, ok)
		require.Equal(t, "B", payload["title"])
	}
}

// TestRecordService_StructuredFilterPushdownMultipleKeysAnd 校验:
// 多个键的结构化过滤被 AND 串联(两个 WHERE 子句都会落到 SQL).
func TestRecordService_StructuredFilterPushdownMultipleKeysAnd(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)

	owner := createResourceUser(t, db, "record_owner_pushdown_multi")
	database := createResourceDatabase(t, db, owner.ID, "PushdownMultiDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	createResourceField(t, db, table.ID, "status", "string", true, "")
	createResourceField(t, db, table.ID, "priority", "string", true, "")

	rows := []map[string]interface{}{
		{"status": "open", "priority": "high"},
		{"status": "open", "priority": "low"},
		{"status": "closed", "priority": "high"},
	}
	for _, data := range rows {
		_, err := recordService.CreateRecord(CreateRecordRequest{TableID: table.ID, Data: data}, owner.ID)
		require.NoError(t, err)
	}

	result, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   10,
		Filter:  `{"status":"open","priority":"high"}`,
	}, owner.ID)
	require.NoError(t, err)
	require.Len(t, result.Records, 1)
	require.Equal(t, int64(1), result.Total)

	payload, ok := result.Records[0].Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "open", payload["status"])
	require.Equal(t, "high", payload["priority"])
}

// TestRecordService_KeywordFilterRejectsLargeScan 校验:
// 关键字过滤路径在 SQL LIKE 预筛后行数超出 maxKeywordScanRecords 时直接拒绝,
// 避免一次性把整张表加载到内存做 in-memory 过滤.
func TestRecordService_KeywordFilterRejectsLargeScan(t *testing.T) {
	originalCap := maxKeywordScanRecords
	maxKeywordScanRecords = 3
	t.Cleanup(func() { maxKeywordScanRecords = originalCap })

	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)

	owner := createResourceUser(t, db, "record_owner_keyword_cap")
	database := createResourceDatabase(t, db, owner.ID, "KeywordCapDB")
	table := createResourceTable(t, db, database.ID, "Orders")
	createResourceField(t, db, table.ID, "title", "string", true, "")

	for i := 0; i < 5; i++ {
		data, _ := json.Marshal(map[string]interface{}{"title": fmt.Sprintf("订单-%d", i)})
		require.NoError(t, db.Create(&models.Record{
			TableID:   table.ID,
			Data:      string(data),
			CreatedBy: owner.ID,
			UpdatedBy: owner.ID,
			Version:   1,
		}).Error)
	}

	_, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   10,
		Filter:  "订单",
	}, owner.ID)
	require.Error(t, err)
	require.Contains(t, err.Error(), "关键字过滤匹配过多记录")
}

// TestRecordService_KeywordFilterWithinCapUsesPermissionAwareMatch 校验:
// 关键字回退在数据量未触发上限时仍走 in-memory 权限感知过滤,
// 即便 SQL LIKE 命中了隐藏字段值,过滤后也只返回可见字段命中的记录.
func TestRecordService_KeywordFilterWithinCapUsesPermissionAwareMatch(t *testing.T) {
	db := setupResourceTestDB(t)
	recordService := NewRecordService(db)
	fieldService := NewFieldService(db)

	owner := createResourceUser(t, db, "record_owner_keyword_match")
	viewer := createResourceUser(t, db, "record_viewer_keyword_match")
	database := createResourceDatabase(t, db, owner.ID, "KeywordMatchDB")
	grantResourceDatabaseAccess(t, db, database.ID, viewer.ID, "viewer")

	table := createResourceTable(t, db, database.ID, "Orders")
	publicField := createResourceField(t, db, table.ID, "title", "string", true, "")
	secretField := createResourceField(t, db, table.ID, "secret", "string", false, "")
	require.NoError(t, fieldService.SetFieldPermission(table.ID, FieldPermissionRequest{
		FieldID:   secretField.ID,
		Role:      "viewer",
		CanRead:   false,
		CanWrite:  false,
		CanDelete: false,
	}, owner.ID))

	// record1: 关键字命中可见字段
	_, err := recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开-keyword",
			secretField.Name: "无关",
		},
	}, owner.ID)
	require.NoError(t, err)

	// record2: 关键字仅在隐藏字段中出现
	_, err = recordService.CreateRecord(CreateRecordRequest{
		TableID: table.ID,
		Data: map[string]interface{}{
			publicField.Name: "公开-无关",
			secretField.Name: "secret-keyword",
		},
	}, owner.ID)
	require.NoError(t, err)

	result, err := recordService.ListRecords(QueryRequest{
		TableID: table.ID,
		Limit:   10,
		Filter:  "keyword",
	}, viewer.ID)
	require.NoError(t, err)
	require.Len(t, result.Records, 1)
	require.Equal(t, int64(1), result.Total)
	payload, ok := result.Records[0].Data.(map[string]interface{})
	require.True(t, ok)
	require.Equal(t, "公开-keyword", payload[publicField.Name])
}
