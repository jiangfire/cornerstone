package handlers

import (
	"testing"

	"github.com/jiangfire/cornerstone/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeRecordData_ValidJSON(t *testing.T) {
	record := &models.Record{
		ID:   "rec_1",
		Data: `{"name":"alice","age":30}`,
	}
	data, corrupted := decodeRecordData(record)
	assert.False(t, corrupted)

	asMap, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", asMap["name"])
	assert.Equal(t, float64(30), asMap["age"])
}

func TestDecodeRecordData_Corrupted(t *testing.T) {
	record := &models.Record{
		ID:      "rec_bad",
		TableID: "tbl_x",
		Data:    `{not json`,
	}
	data, corrupted := decodeRecordData(record)
	assert.True(t, corrupted)

	asMap, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, asMap)
}

func TestDecodeRecordData_EmptyData(t *testing.T) {
	record := &models.Record{ID: "rec_empty"}
	data, corrupted := decodeRecordData(record)
	assert.False(t, corrupted)

	asMap, ok := data.(map[string]any)
	require.True(t, ok)
	assert.Empty(t, asMap)
}

func TestDecodeRecordData_NestedJSON(t *testing.T) {
	record := &models.Record{
		ID:   "rec_nested",
		Data: `{"profile":{"city":"Beijing"}}`,
	}
	data, corrupted := decodeRecordData(record)
	assert.False(t, corrupted)

	asMap, ok := data.(map[string]any)
	require.True(t, ok)

	profile, ok := asMap["profile"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "Beijing", profile["city"])
}

func TestRecordObjectFromModel_CorruptedData(t *testing.T) {
	record := &models.Record{ID: "rec_bad", Data: `not json`}
	resp := recordObjectFromModel(record, map[string]any{"id": record.ID, "version": 1})
	assert.Equal(t, "rec_bad", resp.ID)
	assert.Equal(t, 1, resp.Version)
}

func TestRecordObjectFromModel_ValidData(t *testing.T) {
	record := &models.Record{
		ID:      "rec_1",
		TableID: "tbl_1",
		Data:    `{"name":"alice","score":95.5}`,
		Version: 3,
	}
	resp := recordObjectFromModel(record, map[string]any{
		"id":       record.ID,
		"version":  record.Version,
		"table_id": record.TableID,
	})
	assert.Equal(t, "rec_1", resp.ID)
	assert.Equal(t, 3, resp.Version)
	assert.Equal(t, "tbl_1", resp.TableID)

	data, ok := resp.Data.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", data["name"])
	assert.Equal(t, 95.5, data["score"])
}
