package handlers

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jiangfire/cornerstone/backend/internal/models"
)

func TestDecodeRecordData_ValidJSON(t *testing.T) {
	record := &models.Record{
		ID:   "rec_1",
		Data: `{"name":"alice","age":30}`,
	}
	data, corrupted := decodeRecordData(record)
	if corrupted {
		t.Fatalf("expected non-corrupted, got corrupted=true")
	}
	asMap, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", data)
	}
	if asMap["name"] != "alice" {
		t.Errorf("name=%v", asMap["name"])
	}
}

func TestDecodeRecordData_Corrupted(t *testing.T) {
	record := &models.Record{
		ID:      "rec_bad",
		TableID: "tbl_x",
		Data:    `{not json`,
	}
	data, corrupted := decodeRecordData(record)
	if !corrupted {
		t.Fatalf("expected corrupted=true for malformed JSON")
	}
	asMap, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any fallback, got %T", data)
	}
	if len(asMap) != 0 {
		t.Errorf("expected empty fallback map, got %v", asMap)
	}
}

func TestDecodeRecordData_EmptyData(t *testing.T) {
	record := &models.Record{ID: "rec_empty"}
	data, corrupted := decodeRecordData(record)
	if corrupted {
		t.Errorf("empty Data should not flag corrupted")
	}
	asMap, ok := data.(map[string]any)
	if !ok || len(asMap) != 0 {
		t.Errorf("empty Data should yield empty map, got %T %v", data, data)
	}
}

func TestRecordResponseWithData_AddsCorruptedFlag(t *testing.T) {
	record := &models.Record{ID: "rec_bad", Data: `not json`}
	resp := recordResponseWithData(record, gin.H{"id": record.ID, "version": 1})
	if resp["_corrupted"] != true {
		t.Fatalf("expected _corrupted=true on response, got %v", resp["_corrupted"])
	}
	if resp["id"] != "rec_bad" || resp["version"] != 1 {
		t.Errorf("extra fields not merged, got %v", resp)
	}
}

func TestRecordResponseWithData_NoCorruptedFlagOnSuccess(t *testing.T) {
	record := &models.Record{ID: "rec_ok", Data: `{"k":"v"}`}
	resp := recordResponseWithData(record, gin.H{"id": record.ID})
	if _, ok := resp["_corrupted"]; ok {
		t.Fatalf("_corrupted should not be set on success, resp=%v", resp)
	}
}
