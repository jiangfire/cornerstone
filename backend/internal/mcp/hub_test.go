package mcp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSSEHubPublishesNotificationToRegisteredUser(t *testing.T) {
	hub := NewSSEHub()

	_, ch, _, _, cleanup := hub.Register("user-1", "")
	defer cleanup()

	delivered := hub.PublishToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"action": "created",
	})
	require.Equal(t, 1, delivered)

	select {
	case message := <-ch:
		require.Equal(t, "message", message.Event)

		notification, ok := message.Data.(Notification)
		require.True(t, ok)
		require.Equal(t, jsonRPCVersion, notification.JSONRPC)
		require.Equal(t, "notifications/databases/changed", notification.Method)
	case <-time.After(time.Second):
		t.Fatal("expected notification to be delivered")
	}
}

func TestSSEHubReturnsZeroWhenUserHasNoSubscribers(t *testing.T) {
	hub := NewSSEHub()

	delivered := hub.PublishToUser("missing-user", "notifications/databases/changed", map[string]interface{}{
		"action": "created",
	})

	require.Zero(t, delivered)
}

func TestSSEHubPublishesToMultipleSubscribers(t *testing.T) {
	hub := NewSSEHub()

	_, ch1, _, _, cleanup1 := hub.Register("user-1", "")
	defer cleanup1()
	_, ch2, _, _, cleanup2 := hub.Register("user-1", "")
	defer cleanup2()

	delivered := hub.PublishToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"action": "created",
	})
	require.Equal(t, 2, delivered)

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("expected first subscriber to receive notification")
	}

	select {
	case <-ch2:
	case <-time.After(time.Second):
		t.Fatal("expected second subscriber to receive notification")
	}
}

func TestSSEHubCleanupIsIdempotent(t *testing.T) {
	hub := NewSSEHub()

	_, _, _, _, cleanup := hub.Register("user-1", "")
	cleanup()
	cleanup()

	delivered := hub.PublishToUser("user-1", "notifications/databases/changed", nil)
	require.Zero(t, delivered)
}

func TestSSEHubDropsNotificationWhenSubscriberBufferIsFull(t *testing.T) {
	hub := NewSSEHub()

	_, ch, _, _, cleanup := hub.Register("user-1", "")
	defer cleanup()

	for i := 0; i < 8; i++ {
		delivered := hub.PublishToUser("user-1", "notifications/databases/changed", map[string]interface{}{
			"index": i,
		})
		require.Equal(t, 1, delivered)
	}

	delivered := hub.PublishToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"index": 8,
	})
	require.Zero(t, delivered)

	drained := 0
	for drained < 8 {
		select {
		case <-ch:
			drained++
		case <-time.After(time.Second):
			t.Fatalf("expected to drain buffered notifications, got %d", drained)
		}
	}
}

func TestSSEHubReplaysEventsAfterLastEventID(t *testing.T) {
	hub := NewSSEHubWithHistory(8)

	first, _ := hub.PublishNotificationToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"index": 1,
	})
	second, _ := hub.PublishNotificationToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"index": 2,
	})

	_, _, replay, status, cleanup := hub.Register("user-1", first.ID)
	defer cleanup()

	require.Equal(t, ReplayReplayed, status)
	require.Len(t, replay, 1)
	require.Equal(t, second.ID, replay[0].ID)
}

func TestSSEHubReturnsReplayUnavailableWhenLastEventIDEvicted(t *testing.T) {
	hub := NewSSEHubWithHistory(2)

	first, _ := hub.PublishNotificationToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"index": 1,
	})
	_, _ = hub.PublishNotificationToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"index": 2,
	})
	_, _ = hub.PublishNotificationToUser("user-1", "notifications/databases/changed", map[string]interface{}{
		"index": 3,
	})

	_, _, replay, status, cleanup := hub.Register("user-1", first.ID)
	defer cleanup()

	require.Equal(t, ReplayMissed, status)
	require.Empty(t, replay)
}
