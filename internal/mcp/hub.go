package mcp

import (
	"sync"

	"github.com/google/uuid"
)

// Notification represents a server-initiated JSON-RPC/MCP notification.
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// SSEMessage represents a single SSE message.
type SSEMessage struct {
	ID    string
	Event string
	Data  interface{}
}

// ReplayStatus describes the replay result for Last-Event-ID.
type ReplayStatus string

const (
	ReplayNotRequested ReplayStatus = "not_requested"
	ReplayReplayed     ReplayStatus = "replayed"
	ReplayAtHead       ReplayStatus = "at_head"
	ReplayMissed       ReplayStatus = "missed"
)

// Notifier provides an abstraction for components that need to send MCP proactive notifications.
type Notifier interface {
	PublishToUser(userID, method string, params interface{}) int
}

type userSSEState struct {
	streams map[string]chan SSEMessage
	history []SSEMessage
}

// SSEHub maintains SSE subscribers on a per-user basis.
type SSEHub struct {
	mu           sync.RWMutex
	users        map[string]*userSSEState
	historyLimit int
}

// NewSSEHub creates a new SSEHub.
func NewSSEHub() *SSEHub {
	return NewSSEHubWithHistory(128)
}

// NewSSEHubWithHistory creates an SSEHub with a history event buffer.
func NewSSEHubWithHistory(historyLimit int) *SSEHub {
	if historyLimit <= 0 {
		historyLimit = 128
	}
	return &SSEHub{
		users:        make(map[string]*userSSEState),
		historyLimit: historyLimit,
	}
}

// Register registers an SSE subscription stream for the specified user and returns replayable events based on Last-Event-ID.
func (h *SSEHub) Register(userID, lastEventID string) (string, <-chan SSEMessage, []SSEMessage, ReplayStatus, func()) {
	streamID := uuid.NewString()
	ch := make(chan SSEMessage, 8)
	replayStatus := ReplayNotRequested
	replay := make([]SSEMessage, 0)

	h.mu.Lock()
	state := h.ensureUserStateLocked(userID)
	if lastEventID != "" {
		replay, replayStatus = h.replayFromLocked(state, lastEventID)
	}
	state.streams[streamID] = ch
	h.mu.Unlock()

	cleanup := func() {
		h.mu.Lock()
		defer h.mu.Unlock()

		state, ok := h.users[userID]
		if !ok {
			return
		}

		if stream, ok := state.streams[streamID]; ok {
			delete(state.streams, streamID)
			close(stream)
		}

		h.pruneUserLocked(userID)
	}

	return streamID, ch, replay, replayStatus, cleanup
}

// PublishToUser dispatches a notification to all SSE subscription streams of the specified user.
func (h *SSEHub) PublishToUser(userID, method string, params interface{}) int {
	_, delivered := h.PublishNotificationToUser(userID, method, params)
	return delivered
}

// PublishNotificationToUser publishes a notification to the specified user and returns the generated SSE message.
func (h *SSEHub) PublishNotificationToUser(userID, method string, params interface{}) (SSEMessage, int) {
	message := SSEMessage{
		ID:    uuid.NewString(),
		Event: "message",
		Data: Notification{
			JSONRPC: jsonRPCVersion,
			Method:  method,
			Params:  params,
		},
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	state := h.ensureUserStateLocked(userID)
	state.history = append(state.history, message)
	if len(state.history) > h.historyLimit {
		state.history = append([]SSEMessage(nil), state.history[len(state.history)-h.historyLimit:]...)
	}

	delivered := 0
	for _, ch := range state.streams {
		select {
		case ch <- message:
			delivered++
		default:
			// Slow consumers must not block the main flow; drop the notification.
		}
	}

	return message, delivered
}

func (h *SSEHub) ensureUserStateLocked(userID string) *userSSEState {
	state, ok := h.users[userID]
	if ok {
		return state
	}

	state = &userSSEState{
		streams: make(map[string]chan SSEMessage),
		history: make([]SSEMessage, 0),
	}
	h.users[userID] = state
	return state
}

func (h *SSEHub) replayFromLocked(state *userSSEState, lastEventID string) ([]SSEMessage, ReplayStatus) {
	if lastEventID == "" {
		return nil, ReplayNotRequested
	}

	for index, item := range state.history {
		if item.ID != lastEventID {
			continue
		}

		if index == len(state.history)-1 {
			return nil, ReplayAtHead
		}

		replay := append([]SSEMessage(nil), state.history[index+1:]...)
		return replay, ReplayReplayed
	}

	return nil, ReplayMissed
}

func (h *SSEHub) pruneUserLocked(userID string) {
	state, ok := h.users[userID]
	if !ok {
		return
	}

	if len(state.streams) == 0 && len(state.history) == 0 {
		delete(h.users, userID)
	}
}
