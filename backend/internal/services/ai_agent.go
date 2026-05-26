package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ChatCompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream,omitempty"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Tools       []any     `json:"tools,omitempty"`
}

type ChatCompletionResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type ToolDef struct {
	Type     string `json:"type"`
	Function any    `json:"function"`
}

type ToolFnDef struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  any    `json:"parameters"`
}

func GetToolDefinitions() []ToolDef {
	return []ToolDef{
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "list_databases",
				Description: "List all databases",
				Parameters: map[string]any{
					"type":       "object",
					"properties": map[string]any{},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "list_tables",
				Description: "List tables in a database",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"database_id": map[string]any{
							"type":        "string",
							"description": "Database ID",
						},
					},
					"required": []string{"database_id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "get_schema",
				Description: "Get database or table schema including fields",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"database_id": map[string]any{
							"type":        "string",
							"description": "Database ID (optional, if table_id not provided)",
						},
						"table_id": map[string]any{
							"type":        "string",
							"description": "Table ID (optional, returns table fields)",
						},
					},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "create_database",
				Description: "Create a new database",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"name": map[string]any{
							"type":        "string",
							"description": "Database name",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "Database description",
						},
					},
					"required": []string{"name"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "create_table",
				Description: "Create a new table in a database",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"database_id": map[string]any{
							"type":        "string",
							"description": "Database ID",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Table name",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "Table description",
						},
						"fields": map[string]any{
							"type": "array",
							"description": "Field definitions",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"name": map[string]any{"type": "string"},
									"type": map[string]any{"type": "string", "enum": []string{"string", "text", "number", "boolean", "date", "datetime", "select", "list"}},
									"description": map[string]any{"type": "string"},
									"required": map[string]any{"type": "boolean"},
								},
								"required": []string{"name", "type"},
							},
						},
					},
					"required": []string{"database_id", "name"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "create_field",
				Description: "Create a new field in a table",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_id": map[string]any{
							"type":        "string",
							"description": "Table ID",
						},
						"name": map[string]any{
							"type":        "string",
							"description": "Field name",
						},
						"type": map[string]any{
							"type":        "string",
							"description": "Field type",
						},
						"description": map[string]any{
							"type":        "string",
							"description": "Field description",
						},
						"required": map[string]any{
							"type":        "boolean",
							"description": "Whether field is required",
						},
					},
					"required": []string{"table_id", "name", "type"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "execute_query",
				Description: "Execute a query using Cornerstone Query DSL",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"from": map[string]any{
							"type":        "string",
							"description": "Table name to query (e.g., records, tables, databases)",
						},
						"select": map[string]any{
							"type": "array",
							"description": "Fields to select",
							"items": map[string]any{"type": "string"},
						},
						"where": map[string]any{
							"type":        "object",
							"description": "Filter conditions",
						},
						"limit": map[string]any{
							"type":        "integer",
							"description": "Max records to return",
						},
						"offset": map[string]any{
							"type":        "integer",
							"description": "Offset for pagination",
						},
					},
					"required": []string{"from"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "insert_records",
				Description: "Insert multiple records into a table",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_id": map[string]any{
							"type":        "string",
							"description": "Table ID",
						},
						"records": map[string]any{
							"type": "array",
							"description": "Array of record data objects",
							"items": map[string]any{
								"type": "object",
								"description": "Record data as key-value pairs",
							},
						},
					},
					"required": []string{"table_id", "records"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "update_record",
				Description: "Update a single record",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"record_id": map[string]any{
							"type":        "string",
							"description": "Record ID",
						},
						"data": map[string]any{
							"type":        "object",
							"description": "Updated field values",
						},
					},
					"required": []string{"record_id", "data"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "delete_record",
				Description: "Delete a single record",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"record_id": map[string]any{
							"type":        "string",
							"description": "Record ID",
						},
					},
					"required": []string{"record_id"},
				},
			},
		},
		{
			Type: "function",
			Function: ToolFnDef{
				Name:        "generate_test_data",
				Description: "Generate test data for a table based on its schema",
				Parameters: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"table_id": map[string]any{
							"type":        "string",
							"description": "Table ID",
						},
						"count": map[string]any{
							"type":        "integer",
							"description": "Number of records to generate",
						},
					},
					"required": []string{"table_id", "count"},
				},
			},
		},
	}
}

type AIAgent struct {
	APIKey  string
	Model   string
	BaseURL string
	HTTP    *http.Client
}

func NewAIAgent(apiKey, model, baseURL string) *AIAgent {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	return &AIAgent{
		APIKey:  apiKey,
		Model:   model,
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (a *AIAgent) Chat(messages []Message, executeTool func(name string, args map[string]any) (any, error)) (string, error) {
	tools := GetToolDefinitions()

	reqBody := ChatCompletionRequest{
		Model:       a.Model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   4096,
	}

	if len(tools) > 0 {
		toolInterfaces := make([]any, len(tools))
		for i, t := range tools {
			toolInterfaces[i] = t
		}
		reqBody.Tools = toolInterfaces
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", a.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.APIKey)

	resp, err := a.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result ChatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	assistantMsg := result.Choices[0].Message

	if len(assistantMsg.ToolCalls) > 0 {
		toolMessages := append(messages, assistantMsg)
		for _, tc := range assistantMsg.ToolCalls {
			var args map[string]any
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				toolMessages = append(toolMessages, Message{
					Role:       "tool",
					Content:    fmt.Sprintf("invalid arguments: %v", err),
					ToolCallID: tc.ID,
				})
				continue
			}

			execResult, err := executeTool(tc.Function.Name, args)
			if err != nil {
				toolMessages = append(toolMessages, Message{
					Role:       "tool",
					Content:    fmt.Sprintf("error: %v", err),
					ToolCallID: tc.ID,
				})
				continue
			}

			content, _ := json.Marshal(execResult)
			toolMessages = append(toolMessages, Message{
				Role:       "tool",
				Content:    string(content),
				ToolCallID: tc.ID,
			})
		}
		return a.Chat(toolMessages, executeTool)
	}

	return assistantMsg.Content, nil
}
