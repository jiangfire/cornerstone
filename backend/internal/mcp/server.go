package mcp

import (
	"context"
	"encoding/json"
)

// Server 负责处理 MCP JSON-RPC 请求
type Server struct {
	toolService *ToolService
	serverName  string
	version     string
}

// NewServer 创建 MCP server
func NewServer(toolService *ToolService, version string) *Server {
	if version == "" {
		version = "dev"
	}

	return &Server{
		toolService: toolService,
		serverName:  "cornerstone-mcp",
		version:     version,
	}
}

// HandleRequest 处理单个请求；当请求是 notification 时返回 nil
func (s *Server) HandleRequest(ctx context.Context, req Request) *Response {
	if req.JSONRPC == "" {
		req.JSONRPC = jsonRPCVersion
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "notifications/initialized":
		return nil
	case "ping":
		return s.success(req.ID, map[string]interface{}{})
	case "tools/list":
		return s.success(req.ID, map[string]interface{}{
			"tools": s.toolService.ListTools(),
		})
	case "tools/call":
		return s.handleToolsCall(ctx, req)
	default:
		if len(req.ID) == 0 {
			return nil
		}
		return s.failure(req.ID, -32601, "Method not found", map[string]interface{}{"method": req.Method})
	}
}

func (s *Server) handleInitialize(req Request) *Response {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(req.Params, &params)

	protocolVersion := params.ProtocolVersion
	if protocolVersion == "" {
		protocolVersion = defaultProtocolVersion
	}

	return s.success(req.ID, map[string]interface{}{
		"protocolVersion": protocolVersion,
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    s.serverName,
			"version": s.version,
		},
	})
}

func (s *Server) handleToolsCall(ctx context.Context, req Request) *Response {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return s.failure(req.ID, -32602, "Invalid tools/call params", err.Error())
	}

	result, err := s.toolService.Call(ctx, params.Name, params.Arguments)
	if err != nil {
		return s.failure(req.ID, -32602, "Tool execution failed", err.Error())
	}

	return s.success(req.ID, result)
}

func (s *Server) success(id json.RawMessage, result interface{}) *Response {
	if len(id) == 0 {
		return nil
	}
	return &Response{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Result:  result,
	}
}

func (s *Server) failure(id json.RawMessage, code int, message string, data interface{}) *Response {
	if len(id) == 0 {
		return nil
	}
	return &Response{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &ResponseError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
}
