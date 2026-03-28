package services

import (
	"testing"

	"github.com/jiangfire/cornerstone/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupIntegrationEventTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	err = db.AutoMigrate(
		&models.GovernanceTask{},
		&models.GovernanceExternalLink{},
		&models.IntegrationInboundEvent{},
		&models.ActivityLog{},
	)
	require.NoError(t, err)

	return db
}

func TestIntegrationEventService_ReceiveEventCreatesTask(t *testing.T) {
	db := setupIntegrationEventTestDB(t)
	service := NewIntegrationEventService(db)

	result, err := service.ReceiveEvent("fuckcmdb", ReceiveIntegrationEventRequest{
		EventID:      "evt_dq_001",
		EventType:    "dq.alert.triggered",
		ResourceType: "dq_result",
		ResourceID:   "dqr_001",
		TraceID:      "trace_001",
		Payload: map[string]interface{}{
			"title":        "Panel ID 空值率异常",
			"summary":      "panel_id 空值率超过阈值 5%",
			"severity":     "high",
			"display_name": "panel_id",
		},
	})
	require.NoError(t, err)
	require.False(t, result.Duplicate)
	require.Equal(t, "processed", result.Event.Status)
	require.NotNil(t, result.Task)
	require.Equal(t, "dq_issue", result.Task.TaskType)
	require.Equal(t, "high", result.Task.Priority)
}

func TestIntegrationEventService_ReceiveEventIsIdempotent(t *testing.T) {
	db := setupIntegrationEventTestDB(t)
	service := NewIntegrationEventService(db)

	request := ReceiveIntegrationEventRequest{
		EventID:      "evt_schema_001",
		EventType:    "metadata.schema.changed",
		ResourceType: "column",
		ResourceID:   "col_001",
		Payload: map[string]interface{}{
			"change_type": "type_changed",
		},
	}

	first, err := service.ReceiveEvent("fuckcmdb", request)
	require.NoError(t, err)
	require.NotNil(t, first.Task)

	second, err := service.ReceiveEvent("fuckcmdb", request)
	require.NoError(t, err)
	require.True(t, second.Duplicate)
	require.NotNil(t, second.Task)
	require.Equal(t, first.Task.ID, second.Task.ID)

	var count int64
	require.NoError(t, db.Model(&models.GovernanceTask{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
}

func TestIntegrationEventService_ReceiveEventAllowsSameEventIDAcrossSources(t *testing.T) {
	db := setupIntegrationEventTestDB(t)
	service := NewIntegrationEventService(db)

	request := ReceiveIntegrationEventRequest{
		EventID:      "evt_shared_001",
		EventType:    "metadata.schema.changed",
		ResourceType: "column",
		ResourceID:   "col_001",
		Payload: map[string]interface{}{
			"change_type": "type_changed",
		},
	}

	first, err := service.ReceiveEvent("fuckcmdb", request)
	require.NoError(t, err)
	require.NotNil(t, first.Task)

	second, err := service.ReceiveEvent("other-system", request)
	require.NoError(t, err)
	require.False(t, second.Duplicate)
	require.NotNil(t, second.Task)
	require.NotEqual(t, first.Task.ID, second.Task.ID)
}

func TestIntegrationEventService_ReceiveEventIgnoresUnknownType(t *testing.T) {
	db := setupIntegrationEventTestDB(t)
	service := NewIntegrationEventService(db)

	result, err := service.ReceiveEvent("fuckcmdb", ReceiveIntegrationEventRequest{
		EventID:      "evt_unknown_001",
		EventType:    "unknown.event",
		ResourceType: "column",
		ResourceID:   "col_001",
		Payload:      map[string]interface{}{},
	})
	require.NoError(t, err)
	require.Nil(t, result.Task)
	require.Equal(t, "ignored", result.Event.Status)
}
