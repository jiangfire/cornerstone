package mcp

import "encoding/json"

const (
	jsonRPCVersion        = "2.0"
	defaultProtocolVersion = "2025-03-26"
)

// Request 表示 MCP 的 JSON-RPC 请求
type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

// Response 表示 JSON-RPC 响应
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   *ResponseError  `json:"error,omitempty"`
}

// ResponseError 表示 JSON-RPC 错误
type ResponseError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// TextContent 是 MCP 文本内容块
type TextContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolDefinition 描述 MCP tool
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolCallResult 是 tools/call 的返回值
type ToolCallResult struct {
	Content           []TextContent `json:"content"`
	StructuredContent interface{}   `json:"structuredContent,omitempty"`
	IsError           bool          `json:"isError,omitempty"`
}
