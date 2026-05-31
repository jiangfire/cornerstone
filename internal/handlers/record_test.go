package handlers

import (
	"testing"

	"github.com/gin-gonic/gin"
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

func TestRecordResponseWithData_AddsCorruptedFlag(t *testing.T) {
	record := &models.Record{ID: "rec_bad", Data: `not json`}
	resp := recordResponseWithData(record, gin.H{"id": record.ID, "version": 1})
	assert.Equal(t, true, resp["_corrupted"])
	assert.Equal(t, "rec_bad", resp["id"])
	assert.Equal(t, 1, resp["version"])
}

func TestRecordResponseWithData_NoCorruptedFlagOnSuccess(t *testing.T) {
	record := &models.Record{ID: "rec_ok", Data: `{"k":"v"}`}
	resp := recordResponseWithData(record, gin.H{"id": record.ID})
	_, hasCorrupted := resp["_corrupted"]
	assert.False(t, hasCorrupted)
	assert.Equal(t, "rec_ok", resp["id"])
}

func TestRecordResponseWithData_MergesDataIntoResponse(t *testing.T) {
	record := &models.Record{
		ID:      "rec_1",
		TableID: "tbl_1",
		Data:    `{"name":"alice","score":95.5}`,
		Version: 3,
	}
	resp := recordResponseWithData(record, gin.H{
		"id":      record.ID,
		"version": record.Version,
	})
	assert.Equal(t, "rec_1", resp["id"])
	assert.Equal(t, 3, resp["version"])

	data, ok := resp["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "alice", data["name"])
	assert.Equal(t, 95.5, data["score"])
}
