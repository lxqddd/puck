package ai

import (
	"context"
	"encoding/json"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type ToolDefinition struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ToolResult struct {
	CallID  string
	Name    string
	Content string
	IsError bool
}

type ToolExecutor func(ctx context.Context, call ToolCall) (ToolResult, error)

type ChatRequest struct {
	Model        string
	System       string
	Messages     []Message
	Tools        []ToolDefinition
	ToolExecutor ToolExecutor
}

type EventType string

const (
	EventDelta     EventType = "delta"
	EventReasoning EventType = "reasoning"
)

type ChatEvent struct {
	Type EventType
	Text string
}

// Provider 是各家模型的最小统一接口：只有流式对话。
type Provider interface {
	Chat(ctx context.Context, req ChatRequest, onEvent func(ChatEvent) error) error
}
