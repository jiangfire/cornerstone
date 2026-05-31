package mcp

import (
	"sync"

	"github.com/google/uuid"
)

// Notification 表示服务端主动推送的 JSON-RPC/MCP notification。
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// SSEMessage 表示单条 SSE 消息。
type SSEMessage struct {
	ID    string
	Event string
	Data  interface{}
}

// ReplayStatus 描述 Last-Event-ID 的重放结果。
type ReplayStatus string

const (
	ReplayNotRequested ReplayStatus = "not_requested"
	ReplayReplayed     ReplayStatus = "replayed"
	ReplayAtHead       ReplayStatus = "at_head"
	ReplayMissed       ReplayStatus = "missed"
)

// Notifier 为需要下发 MCP 主动通知的组件提供抽象。
type Notifier interface {
	PublishToUser(userID, method string, params interface{}) int
}

type userSSEState struct {
	streams map[string]chan SSEMessage
	history []SSEMessage
}

// SSEHub 维护基于用户维度的 SSE 订阅者。
type SSEHub struct {
	mu           sync.RWMutex
	users        map[string]*userSSEState
	historyLimit int
}

// NewSSEHub 创建一个新的 SSEHub。
func NewSSEHub() *SSEHub {
	return NewSSEHubWithHistory(128)
}

// NewSSEHubWithHistory 创建带历史事件缓冲的 SSEHub。
func NewSSEHubWithHistory(historyLimit int) *SSEHub {
	if historyLimit <= 0 {
		historyLimit = 128
	}
	return &SSEHub{
		users:        make(map[string]*userSSEState),
		historyLimit: historyLimit,
	}
}

// Register 为指定用户注册一个 SSE 订阅流，并按 Last-Event-ID 返回可重放事件。
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

// PublishToUser 向指定用户的所有 SSE 订阅流下发 notification。
func (h *SSEHub) PublishToUser(userID, method string, params interface{}) int {
	_, delivered := h.PublishNotificationToUser(userID, method, params)
	return delivered
}

// PublishNotificationToUser 向指定用户发布 notification，并返回生成的 SSE 消息。
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
			// 慢消费者不阻塞主流程，直接丢弃该条通知。
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
