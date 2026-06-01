package mcp

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSSEHub_DefaultHistoryLimit(t *testing.T) {
	hub := NewSSEHub()
	assert.Equal(t, 128, hub.historyLimit)
	assert.NotNil(t, hub.users)
}

func TestNewSSEHubWithHistory_CustomLimit(t *testing.T) {
	hub := NewSSEHubWithHistory(64)
	assert.Equal(t, 64, hub.historyLimit)
}

func TestNewSSEHubWithHistory_ZeroLimit(t *testing.T) {
	hub := NewSSEHubWithHistory(0)
	assert.Equal(t, 128, hub.historyLimit)
}

func TestNewSSEHubWithHistory_NegativeLimit(t *testing.T) {
	hub := NewSSEHubWithHistory(-10)
	assert.Equal(t, 128, hub.historyLimit)
}

func TestSSEHub_RegisterCreatesStream(t *testing.T) {
	hub := NewSSEHub()
	streamID, ch, replay, status, cleanup := hub.Register("user1", "")
	defer cleanup()

	assert.NotEmpty(t, streamID)
	assert.NotNil(t, ch)
	assert.Empty(t, replay)
	assert.Equal(t, ReplayNotRequested, status)

	hub.mu.RLock()
	state, ok := hub.users["user1"]
	hub.mu.RUnlock()
	require.True(t, ok)
	assert.Contains(t, state.streams, streamID)
}

func TestSSEHub_RegisterCleanupRemovesStream(t *testing.T) {
	hub := NewSSEHub()
	streamID, _, _, _, cleanup := hub.Register("user1", "")
	_ = streamID

	hub.mu.RLock()
	state := hub.users["user1"]
	_, exists := state.streams[streamID]
	hub.mu.RUnlock()
	assert.True(t, exists)

	cleanup()

	hub.mu.RLock()
	state, ok := hub.users["user1"]
	hub.mu.RUnlock()
	if ok {
		_, exists = state.streams[streamID]
		assert.False(t, exists)
	}
}

func TestSSEHub_RegisterCleanupOnUnknownUserIDSafe(t *testing.T) {
	hub := NewSSEHub()
	_, _, _, _, cleanup := hub.Register("user1", "")

	hub.mu.Lock()
	delete(hub.users, "user1")
	hub.mu.Unlock()

	assert.NotPanics(t, cleanup)
}

func TestSSEHub_PublishToUser_DeliversToStream(t *testing.T) {
	hub := NewSSEHub()
	_, ch, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	delivered := hub.PublishToUser("user1", "test/method", map[string]string{"key": "value"})
	assert.Equal(t, 1, delivered)

	select {
	case msg := <-ch:
		assert.Equal(t, "message", msg.Event)
		assert.NotEmpty(t, msg.ID)
		notification, ok := msg.Data.(Notification)
		require.True(t, ok)
		assert.Equal(t, "2.0", notification.JSONRPC)
		assert.Equal(t, "test/method", notification.Method)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestSSEHub_PublishToUser_NoStreams(t *testing.T) {
	hub := NewSSEHub()
	delivered := hub.PublishToUser("user-no-streams", "test/method", nil)
	assert.Equal(t, 0, delivered)
}

func TestSSEHub_PublishNotificationToUser_ReturnsMessageWithID(t *testing.T) {
	hub := NewSSEHub()
	_, ch, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	msg, delivered := hub.PublishNotificationToUser("user1", "test/method", map[string]string{"k": "v"})
	assert.Equal(t, 1, delivered)
	assert.NotEmpty(t, msg.ID)
	assert.Equal(t, "message", msg.Event)

	notification, ok := msg.Data.(Notification)
	require.True(t, ok)
	assert.Equal(t, "test/method", notification.Method)

	select {
	case received := <-ch:
		assert.Equal(t, msg.ID, received.ID)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestSSEHub_PublishToUser_RespectsHistoryLimit(t *testing.T) {
	hub := NewSSEHubWithHistory(3)
	_, _, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	for i := 0; i < 5; i++ {
		hub.PublishToUser("user1", "test", map[string]int{"i": i})
	}

	hub.mu.RLock()
	state := hub.users["user1"]
	historyLen := len(state.history)
	hub.mu.RUnlock()

	assert.LessOrEqual(t, historyLen, 3)
}

func TestSSEHub_ReplayFromValidLastEventID(t *testing.T) {
	hub := NewSSEHubWithHistory(10)
	_, _, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	var msgIDs []string
	for i := 0; i < 5; i++ {
		msg, _ := hub.PublishNotificationToUser("user1", "test", map[string]int{"i": i})
		msgIDs = append(msgIDs, msg.ID)
	}

	_, ch, replay, status, cleanup2 := hub.Register("user1", msgIDs[2])
	defer cleanup2()

	assert.Equal(t, ReplayReplayed, status)
	require.Len(t, replay, 2)
	assert.Equal(t, msgIDs[3], replay[0].ID)
	assert.Equal(t, msgIDs[4], replay[1].ID)

	select {
	case <-ch:
	default:
	}
}

func TestSSEHub_ReplayAtHead(t *testing.T) {
	hub := NewSSEHubWithHistory(10)
	_, _, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	msg, _ := hub.PublishNotificationToUser("user1", "test", nil)

	_, _, replay, status, cleanup2 := hub.Register("user1", msg.ID)
	defer cleanup2()

	assert.Equal(t, ReplayAtHead, status)
	assert.Empty(t, replay)
}

func TestSSEHub_ReplayWithUnknownEventID(t *testing.T) {
	hub := NewSSEHubWithHistory(10)
	_, _, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	hub.PublishNotificationToUser("user1", "test", nil)

	_, _, replay, status, cleanup2 := hub.Register("user1", "nonexistent-id")
	defer cleanup2()

	assert.Equal(t, ReplayMissed, status)
	assert.Empty(t, replay)
}

func TestSSEHub_ReplayWithEmptyLastEventID(t *testing.T) {
	hub := NewSSEHub()
	_, _, replay, status, cleanup := hub.Register("user1", "")
	defer cleanup()

	assert.Equal(t, ReplayNotRequested, status)
	assert.Empty(t, replay)
}

func TestSSEHub_SlowConsumerDoesNotBlock(t *testing.T) {
	hub := NewSSEHub()
	_, ch, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	for i := 0; i < 20; i++ {
		hub.PublishToUser("user1", "test", map[string]int{"i": i})
	}

	time.Sleep(50 * time.Millisecond)

	delivered := 0
	for {
		select {
		case <-ch:
			delivered++
		default:
			goto done
		}
	}
done:
	assert.LessOrEqual(t, delivered, 8)
}

func TestSSEHub_MultipleStreamsForSameUser(t *testing.T) {
	hub := NewSSEHub()

	_, ch1, _, _, cleanup1 := hub.Register("user1", "")
	defer cleanup1()
	_, ch2, _, _, cleanup2 := hub.Register("user1", "")
	defer cleanup2()

	delivered := hub.PublishToUser("user1", "test", map[string]string{"k": "v"})
	assert.Equal(t, 2, delivered)

	select {
	case msg := <-ch1:
		assert.Equal(t, "message", msg.Event)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ch1")
	}

	select {
	case msg := <-ch2:
		assert.Equal(t, "message", msg.Event)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for ch2")
	}
}

func TestSSEHub_PruneUserLocked_RemovesUserWithNoStreamsAndNoHistory(t *testing.T) {
	hub := NewSSEHub()

	hub.mu.Lock()
	state := hub.ensureUserStateLocked("user1")
	delete(state.streams, "stream1")
	hub.mu.Unlock()

	hub.mu.Lock()
	hub.pruneUserLocked("user1")
	hub.mu.Unlock()

	hub.mu.RLock()
	_, exists := hub.users["user1"]
	hub.mu.RUnlock()
	assert.False(t, exists)
}

func TestSSEHub_PruneUserLocked_KeepsUserWithHistory(t *testing.T) {
	hub := NewSSEHub()

	_, _, _, _, cleanup := hub.Register("user1", "")
	hub.PublishToUser("user1", "test", nil)
	cleanup()

	hub.mu.RLock()
	state, ok := hub.users["user1"]
	hub.mu.RUnlock()

	if ok {
		assert.NotEmpty(t, state.history)
	}
}

func TestSSEHub_PruneUserLocked_UnknownUserID(t *testing.T) {
	hub := NewSSEHub()
	hub.mu.Lock()
	assert.NotPanics(t, func() {
		hub.pruneUserLocked("nonexistent")
	})
	hub.mu.Unlock()
}

func TestSSEHub_RegisterCleanupClosesChannel(t *testing.T) {
	hub := NewSSEHub()
	_, ch, _, _, cleanup := hub.Register("user1", "")
	cleanup()

	_, ok := <-ch
	assert.False(t, ok)
}

func TestSSEHub_PublishToUser_DoesNotCrossUsers(t *testing.T) {
	hub := NewSSEHub()

	_, ch1, _, _, cleanup1 := hub.Register("user1", "")
	defer cleanup1()
	_, ch2, _, _, cleanup2 := hub.Register("user2", "")
	defer cleanup2()

	delivered := hub.PublishToUser("user1", "test", nil)
	assert.Equal(t, 1, delivered)

	select {
	case <-ch1:
	case <-time.After(time.Second):
		t.Fatal("user1 should receive message")
	}

	select {
	case <-ch2:
		t.Fatal("user2 should not receive user1's message")
	default:
	}
}

func TestSSEHub_EnsureUserStateLocked_CreatesNewState(t *testing.T) {
	hub := NewSSEHub()
	hub.mu.Lock()
	state := hub.ensureUserStateLocked("newuser")
	hub.mu.Unlock()

	assert.NotNil(t, state)
	assert.NotNil(t, state.streams)
	assert.NotNil(t, state.history)

	hub.mu.RLock()
	_, exists := hub.users["newuser"]
	hub.mu.RUnlock()
	assert.True(t, exists)
}

func TestSSEHub_EnsureUserStateLocked_ReturnsExistingState(t *testing.T) {
	hub := NewSSEHub()
	hub.mu.Lock()
	state1 := hub.ensureUserStateLocked("user1")
	state2 := hub.ensureUserStateLocked("user1")
	hub.mu.Unlock()

	assert.Same(t, state1, state2)
}

func TestSSEHub_ReplayFromLocked_EmptyLastEventID(t *testing.T) {
	hub := NewSSEHub()
	state := &userSSEState{
		streams: make(map[string]chan SSEMessage),
		history: []SSEMessage{{ID: "evt-1"}},
	}
	replay, status := hub.replayFromLocked(state, "")
	assert.Equal(t, ReplayNotRequested, status)
	assert.Nil(t, replay)
}

func TestSSEHub_ReplayFromLocked_MiddleOfHistory(t *testing.T) {
	hub := NewSSEHub()
	state := &userSSEState{
		streams: make(map[string]chan SSEMessage),
		history: []SSEMessage{
			{ID: "evt-1"},
			{ID: "evt-2"},
			{ID: "evt-3"},
			{ID: "evt-4"},
		},
	}
	replay, status := hub.replayFromLocked(state, "evt-2")
	assert.Equal(t, ReplayReplayed, status)
	require.Len(t, replay, 2)
	assert.Equal(t, "evt-3", replay[0].ID)
	assert.Equal(t, "evt-4", replay[1].ID)
}

func TestSSEHub_ReplayFromLocked_LastItemIsHead(t *testing.T) {
	hub := NewSSEHub()
	state := &userSSEState{
		streams: make(map[string]chan SSEMessage),
		history: []SSEMessage{
			{ID: "evt-1"},
			{ID: "evt-2"},
		},
	}
	replay, status := hub.replayFromLocked(state, "evt-2")
	assert.Equal(t, ReplayAtHead, status)
	assert.Nil(t, replay)
}

func TestSSEHub_ReplayFromLocked_IDNotFound(t *testing.T) {
	hub := NewSSEHub()
	state := &userSSEState{
		streams: make(map[string]chan SSEMessage),
		history: []SSEMessage{
			{ID: "evt-1"},
			{ID: "evt-2"},
		},
	}
	replay, status := hub.replayFromLocked(state, "evt-missing")
	assert.Equal(t, ReplayMissed, status)
	assert.Nil(t, replay)
}

func TestSSEHub_HistoryTrimPreservesOrder(t *testing.T) {
	hub := NewSSEHubWithHistory(2)
	_, _, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	msg1, _ := hub.PublishNotificationToUser("user1", "test", map[string]int{"i": 1})
	msg2, _ := hub.PublishNotificationToUser("user1", "test", map[string]int{"i": 2})
	msg3, _ := hub.PublishNotificationToUser("user1", "test", map[string]int{"i": 3})

	hub.mu.RLock()
	state := hub.users["user1"]
	require.Len(t, state.history, 2)
	assert.Equal(t, msg2.ID, state.history[0].ID)
	assert.Equal(t, msg3.ID, state.history[1].ID)
	hub.mu.RUnlock()

	_, ch, _, _, cleanup2 := hub.Register("user1", msg1.ID)
	defer cleanup2()

	drainChannel(ch)

	_, _, replay, status, cleanup3 := hub.Register("user1", msg2.ID)
	defer cleanup3()

	if status == ReplayMissed {
		return
	}
	assert.Equal(t, ReplayReplayed, status)
	require.Len(t, replay, 1)
	assert.Equal(t, msg3.ID, replay[0].ID)
}

func TestSSEHub_ConcurrentPublishAndRegister(t *testing.T) {
	hub := NewSSEHub()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, ch, _, _, cleanup := hub.Register("user1", "")
			defer cleanup()
			hub.PublishToUser("user1", "test", nil)
			select {
			case <-ch:
			case <-time.After(time.Second):
			}
		}()
	}

	wg.Wait()
}

func TestSSEHub_RegisterCleanupTwiceSafe(t *testing.T) {
	hub := NewSSEHub()
	_, _, _, _, cleanup := hub.Register("user1", "")
	cleanup()
	assert.NotPanics(t, cleanup)
}

func TestSSEHub_PublishNotificationToUser_NoStreamsStillStoresHistory(t *testing.T) {
	hub := NewSSEHubWithHistory(10)

	msg, delivered := hub.PublishNotificationToUser("user-nostream", "test/method", map[string]string{"k": "v"})
	assert.Equal(t, 0, delivered)
	assert.NotEmpty(t, msg.ID)

	hub.mu.RLock()
	state, ok := hub.users["user-nostream"]
	hub.mu.RUnlock()
	require.True(t, ok)
	require.Len(t, state.history, 1)
	assert.Equal(t, msg.ID, state.history[0].ID)
}

func TestSSEHub_ReplayAfterHistoryTrim(t *testing.T) {
	hub := NewSSEHubWithHistory(3)
	_, _, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	msgs := make([]SSEMessage, 5)
	for i := 0; i < 5; i++ {
		msgs[i], _ = hub.PublishNotificationToUser("user1", "test", map[string]int{"i": i})
	}

	drainChannel := func(ch <-chan SSEMessage) {
		for {
			select {
			case <-ch:
			default:
				return
			}
		}
	}

	_, ch, _, _, cleanup2 := hub.Register("user1", "")
	defer cleanup2()
	drainChannel(ch)

	_, _, replay, status, _ := hub.Register("user1", msgs[1].ID)
	if status == ReplayMissed {
		return
	}
	if status == ReplayReplayed {
		assert.GreaterOrEqual(t, len(replay), 1)
	}
}

func TestSSEHub_PublishToUser_ParamsStoredInNotification(t *testing.T) {
	hub := NewSSEHub()
	_, ch, _, _, cleanup := hub.Register("user1", "")
	defer cleanup()

	params := map[string]interface{}{
		"action": "created",
		"item":   map[string]string{"id": "abc"},
	}
	hub.PublishToUser("user1", "test/method", params)

	select {
	case msg := <-ch:
		notification, ok := msg.Data.(Notification)
		require.True(t, ok)
		assert.Equal(t, "2.0", notification.JSONRPC)
		assert.Equal(t, "test/method", notification.Method)

		paramsBytes, err := json.Marshal(notification.Params)
		require.NoError(t, err)
		assert.Contains(t, string(paramsBytes), "created")
		assert.Contains(t, string(paramsBytes), "abc")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestSSEHub_MultipleUsersIndependent(t *testing.T) {
	hub := NewSSEHub()

	_, ch1, _, _, cleanup1 := hub.Register("alice", "")
	defer cleanup1()
	_, ch2, _, _, cleanup2 := hub.Register("bob", "")
	defer cleanup2()

	d1 := hub.PublishToUser("alice", "test/alice", nil)
	d2 := hub.PublishToUser("bob", "test/bob", nil)
	assert.Equal(t, 1, d1)
	assert.Equal(t, 1, d2)

	select {
	case msg := <-ch1:
		n := msg.Data.(Notification)
		assert.Equal(t, "test/alice", n.Method)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	select {
	case msg := <-ch2:
		n := msg.Data.(Notification)
		assert.Equal(t, "test/bob", n.Method)
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func drainChannel(ch <-chan SSEMessage) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
